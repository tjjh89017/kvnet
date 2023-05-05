/*
Copyright 2023.

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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"reflect"

	nadv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// NetworkAttachmentDefinitionReconciler reconciles a NetworkAttachmentDefinition object
type NetworkAttachmentDefinitionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type BridgeNetConf struct {
	Type       string `json:"type,omitempty"`
	BridgeName string `json:"bridge,omitempty"`
}

//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NetworkAttachmentDefinition object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *NetworkAttachmentDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling net-attach-def")

	nad := &nadv1.NetworkAttachmentDefinition{}
	if err := r.Get(ctx, req.NamespacedName, nad); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("net-attach-def resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get net-attach-def")
		return ctrl.Result{}, err
	}

	if nad.GetDeletionTimestamp() != nil {
		// skip
		return ctrl.Result{}, nil
	}

	return r.OnChange(ctx, nad)
}

func (r *NetworkAttachmentDefinitionReconciler) OnChange(ctx context.Context, nad *nadv1.NetworkAttachmentDefinition) (ctrl.Result, error) {
	nadCopy := nad.DeepCopy()
	if nadCopy.Labels == nil {
		nadCopy.Labels = make(map[string]string)
	}

	// parse bridge config
	var bridgeNetConf BridgeNetConf
	if err := json.Unmarshal([]byte(nadCopy.Spec.Config), &bridgeNetConf); err != nil {
		// ignore this error
		logrus.Warnf("failed to parse net-attach-def spec")
		return ctrl.Result{}, nil
	}

	if bridgeNetConf.Type != "bridge" {
		// ignore
		logrus.Infof("not bridge cni")
		return ctrl.Result{}, nil
	}

	if bridgeNetConf.BridgeName == "" {
		logrus.Errorf("bridge type netConfg without bridge name")
		return ctrl.Result{}, fmt.Errorf("bridge type netConfg without bridge name")
	}

	nadCopy.Labels[kvnetv1alpha1.BridgeNodeLabel+bridgeNetConf.BridgeName] = ""
	if !reflect.DeepEqual(nad, nadCopy) {
		if err := r.Update(ctx, nadCopy); err != nil {
			logrus.Errorf("update net-attach-def with labels")
			return ctrl.Result{}, fmt.Errorf("update net-attach-def with labels")
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkAttachmentDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nadv1.NetworkAttachmentDefinition{}).
		Complete(r)
}
