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
	"fmt"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// BridgeConfigReconciler reconciles a BridgeConfig object
type BridgeConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridgeconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridgeconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridgeconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=nodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=nodes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BridgeConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *BridgeConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling BridgeConfig")

	bridgeConfig := &kvnetv1alpha1.BridgeConfig{}
	if err := r.Get(ctx, req.NamespacedName, bridgeConfig); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("BridgeConfig resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get BridgeConfig")
		return ctrl.Result{}, err
	}

	if bridgeConfig.GetDeletionTimestamp() != nil {
		if result, err := r.OnRemove(ctx, bridgeConfig); err != nil {
			return result, err
		}

		controllerutil.RemoveFinalizer(bridgeConfig, kvnetv1alpha1.BridgeConfigFinalizer)
		if err := r.Update(ctx, bridgeConfig); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return r.OnChange(ctx, bridgeConfig)
}

func (r *BridgeConfigReconciler) OnChange(ctx context.Context, bridgeConfig *kvnetv1alpha1.BridgeConfig) (ctrl.Result, error) {
	logrus.Info("OnChange")

	selector, err := metav1.LabelSelectorAsSelector(bridgeConfig.Spec.NodeSelector)
	if err != nil {
		logrus.Errorf("NodeSelector parse error %v", err)
		return ctrl.Result{}, err
	}

	nodes := &corev1.NodeList{}
	if err := r.List(ctx, nodes, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, err
	}

	logrus.Infof("nodes: %v", nodes)

	// TODO need to check if bridge is existed

	for _, node := range nodes.Items {
		if err := r.Create(ctx, &kvnetv1alpha1.Bridge{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: bridgeConfig.Namespace,
				Name:      fmt.Sprintf("%s.%s", node.Name, bridgeConfig.Name),
				Labels: map[string]string{
					kvnetv1alpha1.BridgeConfigNamespaceLabel: bridgeConfig.Namespace,
					kvnetv1alpha1.BridgeConfigNameLabel:      bridgeConfig.Name,
				},
			},
		}); err != nil {
			logrus.Errorf("create error %v", err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *BridgeConfigReconciler) OnRemove(ctx context.Context, bridgeConfig *kvnetv1alpha1.BridgeConfig) (ctrl.Result, error) {
	logrus.Info("OnRemove")

	bridgeList := &kvnetv1alpha1.BridgeList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			kvnetv1alpha1.BridgeConfigNamespaceLabel: bridgeConfig.Namespace,
			kvnetv1alpha1.BridgeConfigNameLabel:      bridgeConfig.Name,
		},
	}

	if err := r.List(ctx, bridgeList, opts...); err != nil {
		return ctrl.Result{}, err
	}

	logrus.Info("remove bridge")
	for _, bridge := range bridgeList.Items {
		if err := r.Delete(ctx, &bridge); err != nil {
			logrus.Errorf("delete fail %v", err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BridgeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.BridgeConfig{}).
		Complete(r)
}
