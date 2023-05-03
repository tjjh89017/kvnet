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
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// BridgeReconciler reconciles a Bridge object
type BridgeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Bridge object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *BridgeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling Bridge")

	bridge := &kvnetv1alpha1.Bridge{}
	if err := r.Get(ctx, req.NamespacedName, bridge); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Bridge resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get Bridge")
		return ctrl.Result{}, err
	}

	if bridge.GetDeletionTimestamp() != nil {
		if result, err := r.OnRemove(ctx, bridge); err != nil {
			return result, err
		}

		controllerutil.RemoveFinalizer(bridge, kvnetv1alpha1.BridgeFinalizer)
		if err := r.Update(ctx, bridge); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return r.OnChange(ctx, bridge)
}

func (r *BridgeReconciler) OnChange(ctx context.Context, bridge *kvnetv1alpha1.Bridge) (ctrl.Result, error) {
	logrus.Info("OnChange")

	return ctrl.Result{}, nil
}

func (r *BridgeReconciler) OnRemove(ctx context.Context, bridge *kvnetv1alpha1.Bridge) (ctrl.Result, error) {
	logrus.Info("OnRemove")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BridgeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.Bridge{}).
		Complete(r)
}
