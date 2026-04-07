/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
	"github.com/tjjh89017/kvnet/internal/helper"
)

// BridgeReconciler reconciles a Bridge object (agent mode)
type BridgeReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	NodeName string
}

// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

func (r *BridgeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var bridge kvnetv1alpha1.Bridge
	if err := r.Get(ctx, req.NamespacedName, &bridge); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Only handle bridges for this node
	nodeLabel := bridge.Labels[kvnetv1alpha1.LabelNode]
	if nodeLabel != r.NodeName {
		return ctrl.Result{}, nil
	}

	if !bridge.DeletionTimestamp.IsZero() {
		return r.onDelete(ctx, req, &bridge)
	}

	if !controllerutil.ContainsFinalizer(&bridge, kvnetv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(&bridge, kvnetv1alpha1.FinalizerName)
		if err := r.Update(ctx, &bridge); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return r.onChange(ctx, req, &bridge)
}

func (r *BridgeReconciler) onChange(ctx context.Context, _ ctrl.Request, bridge *kvnetv1alpha1.Bridge) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	bridgeName, err := helper.ParseDeviceName(bridge.Name, r.NodeName)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Check if bridge exists, create if not
	if err := helper.ExecCmd("ip", "link", "show", "dev", bridgeName); err != nil {
		log.Info("creating bridge", "name", bridgeName)
		if err := helper.ExecCmd("ip", "link", "add", bridgeName, "type", "bridge"); err != nil {
			r.setReadyCondition(ctx, bridge, metav1.ConditionFalse, "CreateFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	// Set VLAN filtering
	vlanFiltering := "0"
	if bridge.Spec.VlanFiltering {
		vlanFiltering = "1"
	}
	if err := helper.ExecCmd("ip", "link", "set", bridgeName, "type", "bridge", "vlan_filtering", vlanFiltering); err != nil {
		r.setReadyCondition(ctx, bridge, metav1.ConditionFalse, "ConfigFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Set nf_call_iptables
	nfCallIptables := "0"
	if bridge.Spec.NfCallIptables {
		nfCallIptables = "1"
	}
	if err := helper.ExecCmd("ip", "link", "set", bridgeName, "type", "bridge", "nf_call_iptables", nfCallIptables); err != nil {
		r.setReadyCondition(ctx, bridge, metav1.ConditionFalse, "ConfigFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Bring bridge up
	if err := helper.ExecCmd("ip", "link", "set", bridgeName, "up"); err != nil {
		r.setReadyCondition(ctx, bridge, metav1.ConditionFalse, "UpFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Label the node
	if err := r.labelNode(ctx, bridgeName, "true"); err != nil {
		log.Error(err, "failed to label node")
	}

	r.setReadyCondition(ctx, bridge, metav1.ConditionTrue, "Configured", "Bridge configured successfully.")
	log.Info("bridge configured", "name", bridgeName)
	return ctrl.Result{}, nil
}

func (r *BridgeReconciler) onDelete(ctx context.Context, _ ctrl.Request, bridge *kvnetv1alpha1.Bridge) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	bridgeName, err := helper.ParseDeviceName(bridge.Name, r.NodeName)
	if err == nil {
		if delErr := helper.ExecCmd("ip", "link", "del", bridgeName); delErr != nil {
			log.Info("bridge already removed or failed to delete", "name", bridgeName, "error", delErr)
		}
		if err := r.labelNode(ctx, bridgeName, ""); err != nil {
			log.Error(err, "failed to remove node label")
		}
	}

	controllerutil.RemoveFinalizer(bridge, kvnetv1alpha1.FinalizerName)
	return ctrl.Result{}, r.Update(ctx, bridge)
}

func (r *BridgeReconciler) labelNode(ctx context.Context, bridgeName, value string) error {
	var node corev1.Node
	if err := r.Get(ctx, types.NamespacedName{Name: r.NodeName}, &node); err != nil {
		return err
	}

	patch := client.MergeFrom(node.DeepCopy())
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}
	labelKey := fmt.Sprintf("bridge.kvnet.kojuro.date/%s", bridgeName)
	if value == "" {
		delete(node.Labels, labelKey)
	} else {
		node.Labels[labelKey] = value
	}
	return r.Patch(ctx, &node, patch)
}

func (r *BridgeReconciler) setReadyCondition(ctx context.Context, bridge *kvnetv1alpha1.Bridge, status metav1.ConditionStatus, reason, message string) {
	patch := client.MergeFrom(bridge.DeepCopy())
	meta.SetStatusCondition(&bridge.Status.Conditions, metav1.Condition{
		Type:               kvnetv1alpha1.ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: bridge.Generation,
	})
	if err := r.Status().Patch(ctx, bridge, patch); err != nil {
		logf.FromContext(ctx).Error(err, "failed to patch Bridge status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BridgeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.Bridge{}).
		Named("bridge").
		Complete(r)
}
