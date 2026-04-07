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

// UplinkReconciler reconciles a Uplink object (agent mode)
type UplinkReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	NodeName string
}

// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinks/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

func (r *UplinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var uplink kvnetv1alpha1.Uplink
	if err := r.Get(ctx, req.NamespacedName, &uplink); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	nodeLabel := uplink.Labels[kvnetv1alpha1.LabelNode]
	if nodeLabel != r.NodeName {
		return ctrl.Result{}, nil
	}

	if !uplink.DeletionTimestamp.IsZero() {
		return r.onDelete(ctx, req, &uplink)
	}

	if !controllerutil.ContainsFinalizer(&uplink, kvnetv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(&uplink, kvnetv1alpha1.FinalizerName)
		if err := r.Update(ctx, &uplink); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return r.onChange(ctx, req, &uplink)
}

func (r *UplinkReconciler) onChange(ctx context.Context, _ ctrl.Request, uplink *kvnetv1alpha1.Uplink) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	bondName, err := helper.ParseDeviceName(uplink.Name, r.NodeName)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Check if bond exists, create if not
	if err := helper.ExecCmd("ip", "link", "show", "dev", bondName); err != nil {
		log.Info("creating bond", "name", bondName)
		if err := helper.ExecCmd("ip", "link", "add", bondName, "type", "bond"); err != nil {
			r.setReadyCondition(ctx, uplink, metav1.ConditionFalse, "CreateFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	// Set bond mode
	if uplink.Spec.BondMode != "" {
		if err := helper.ExecCmd("ip", "link", "set", bondName, "type", "bond", "miimon", "100", "mode", uplink.Spec.BondMode); err != nil {
			r.setReadyCondition(ctx, uplink, metav1.ConditionFalse, "ConfigFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	// Attach slaves
	for _, slave := range uplink.Spec.BondSlaves {
		_ = helper.ExecCmd("ip", "link", "set", slave, "down")
		if err := helper.ExecCmd("ip", "link", "set", slave, "master", bondName); err != nil {
			log.Error(err, "failed to attach slave", "slave", slave, "bond", bondName)
		}
	}

	// Set master bridge if specified
	if uplink.Spec.Master != "" {
		if err := helper.ExecCmd("ip", "link", "set", bondName, "master", uplink.Spec.Master); err != nil {
			r.setReadyCondition(ctx, uplink, metav1.ConditionFalse, "MasterFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	// Bring bond up
	if err := helper.ExecCmd("ip", "link", "set", bondName, "up"); err != nil {
		r.setReadyCondition(ctx, uplink, metav1.ConditionFalse, "UpFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Configure bridge port VLAN settings (only when device is a bridge slave)
	if helper.IsBridgeSlave(bondName) && uplink.Spec.PortVLANConfig != nil {
		if err := helper.ApplyPortVLANConfig(bondName, uplink.Spec.PortVLANConfig); err != nil {
			r.setReadyCondition(ctx, uplink, metav1.ConditionFalse, "VLANConfigFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	// Label the node
	if err := r.labelNode(ctx, bondName, "true"); err != nil {
		log.Error(err, "failed to label node")
	}

	r.setReadyCondition(ctx, uplink, metav1.ConditionTrue, "Configured", "Uplink configured successfully.")
	log.Info("uplink configured", "name", bondName)
	return ctrl.Result{}, nil
}

func (r *UplinkReconciler) onDelete(ctx context.Context, _ ctrl.Request, uplink *kvnetv1alpha1.Uplink) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	bondName, err := helper.ParseDeviceName(uplink.Name, r.NodeName)
	if err == nil {
		if delErr := helper.ExecCmd("ip", "link", "del", bondName); delErr != nil {
			log.Info("bond already removed or failed to delete", "name", bondName, "error", delErr)
		}
		if err := r.labelNode(ctx, bondName, ""); err != nil {
			log.Error(err, "failed to remove node label")
		}
	}

	controllerutil.RemoveFinalizer(uplink, kvnetv1alpha1.FinalizerName)
	return ctrl.Result{}, r.Update(ctx, uplink)
}

func (r *UplinkReconciler) labelNode(ctx context.Context, bondName, value string) error {
	var node corev1.Node
	if err := r.Get(ctx, types.NamespacedName{Name: r.NodeName}, &node); err != nil {
		return err
	}

	patch := client.MergeFrom(node.DeepCopy())
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}
	labelKey := fmt.Sprintf("uplink.kvnet.kojuro.date/%s", bondName)
	if value == "" {
		delete(node.Labels, labelKey)
	} else {
		node.Labels[labelKey] = value
	}
	return r.Patch(ctx, &node, patch)
}

func (r *UplinkReconciler) setReadyCondition(ctx context.Context, uplink *kvnetv1alpha1.Uplink, status metav1.ConditionStatus, reason, message string) {
	patch := client.MergeFrom(uplink.DeepCopy())
	meta.SetStatusCondition(&uplink.Status.Conditions, metav1.Condition{
		Type:               kvnetv1alpha1.ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: uplink.Generation,
	})
	if err := r.Status().Patch(ctx, uplink, patch); err != nil {
		logf.FromContext(ctx).Error(err, "failed to patch Uplink status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *UplinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.Uplink{}).
		Named("uplink").
		Complete(r)
}
