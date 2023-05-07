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
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

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
		if !controllerutil.ContainsFinalizer(bridgeConfig, kvnetv1alpha1.BridgeConfigFinalizer) {
			logrus.Info("already onRemove")
			return ctrl.Result{}, nil
		}

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

	bridges, err := r.findBridge(ctx, "", bridgeConfig)
	if err != nil {
		logrus.Errorf("bridge list fail %v", err)
		return ctrl.Result{}, err
	}

	nodeMap := make(map[string]int)
	for _, node := range nodes.Items {
		// set 1 to mark we should create but we still not
		nodeMap[node.Name] = 1
	}
	for _, bridge := range bridges.Items {
		// get node name
		name := strings.Split(bridge.Name, ".")[0]
		// already have bridge
		nodeMap[name] |= 2
	}

	for nodeName, state := range nodeMap {
		switch state {
		case 1:
			// need create
			if err := r.addBridge(ctx, nodeName, bridgeConfig); err != nil {
				return ctrl.Result{}, err
			}
		case 2:
			// need delete
			if err := r.delBridge(ctx, nodeName, bridgeConfig); err != nil {
				return ctrl.Result{}, err
			}
		case 3:
			// check spec is the same
			// check bridge belongs to this bridgeConfig first
			if err := r.updateBridge(ctx, nodeName, bridgeConfig); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *BridgeConfigReconciler) OnRemove(ctx context.Context, bridgeConfig *kvnetv1alpha1.BridgeConfig) (ctrl.Result, error) {
	logrus.Info("OnRemove")

	bridgeList, err := r.findBridge(ctx, "", bridgeConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	logrus.Info("remove bridge")
	// TODO change to DeleteAllOf
	for _, bridge := range bridgeList.Items {
		if err := r.Delete(ctx, &bridge); err != nil {
			logrus.Errorf("delete fail %v", err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *BridgeConfigReconciler) addBridge(ctx context.Context, nodeName string, bridgeConfig *kvnetv1alpha1.BridgeConfig) error {
	bridge := &kvnetv1alpha1.Bridge{
		ObjectMeta: bridgeConfig.Spec.Template.ObjectMeta,
		Spec:       bridgeConfig.Spec.Template.Spec,
	}
	bridge.Namespace = bridgeConfig.Namespace
	bridge.Name = fmt.Sprintf("%s.%s", nodeName, bridgeConfig.Spec.BridgeName)

	if bridge.Labels == nil {
		bridge.Labels = make(map[string]string)
	}
	bridge.Labels[kvnetv1alpha1.BridgeConfigNamespaceLabel] = bridgeConfig.Namespace
	bridge.Labels[kvnetv1alpha1.BridgeConfigNameLabel] = bridgeConfig.Name
	bridge.Labels[kvnetv1alpha1.BridgeConfigNodeLabel] = nodeName

	return r.Create(ctx, bridge)
}

func (r *BridgeConfigReconciler) delBridge(ctx context.Context, nodeName string, bridgeConfig *kvnetv1alpha1.BridgeConfig) error {
	bridges, err := r.findBridge(ctx, nodeName, bridgeConfig)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}
	for _, bridge := range bridges.Items {
		if err := r.Delete(ctx, &bridge); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func (r *BridgeConfigReconciler) updateBridge(ctx context.Context, nodeName string, bridgeConfig *kvnetv1alpha1.BridgeConfig) error {
	bridge := &kvnetv1alpha1.Bridge{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: bridgeConfig.Namespace,
		Name:      fmt.Sprintf("%s.%s", nodeName, bridgeConfig.Spec.BridgeName),
	}, bridge); err != nil {
		return err
	}

	if bridge.Labels[kvnetv1alpha1.BridgeConfigNamespaceLabel] != bridgeConfig.Namespace ||
		bridge.Labels[kvnetv1alpha1.BridgeConfigNameLabel] != bridgeConfig.Name {
		return fmt.Errorf("bridge %s is not belong to this bridgeConfig", bridge.Name)
	}

	bridgeCopy := bridge.DeepCopy()
	bridgeCopy.Spec = *bridgeConfig.Spec.Template.Spec.DeepCopy()
	// It should be only one
	if !reflect.DeepEqual(bridge, bridgeCopy) {
		if err := r.Update(ctx, bridgeCopy); err != nil {
			return err
		}
	}
	return nil
}

func (r *BridgeConfigReconciler) findBridge(ctx context.Context, nodeName string, bridgeConfig *kvnetv1alpha1.BridgeConfig) (*kvnetv1alpha1.BridgeList, error) {
	bridgeList := &kvnetv1alpha1.BridgeList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			kvnetv1alpha1.BridgeConfigNamespaceLabel: bridgeConfig.Namespace,
			kvnetv1alpha1.BridgeConfigNameLabel:      bridgeConfig.Name,
		},
	}

	if nodeName != "" {
		opts = append(opts, client.MatchingLabels{
			kvnetv1alpha1.BridgeConfigNodeLabel: nodeName,
		})
	}

	if err := r.List(ctx, bridgeList, opts...); err != nil {
		return nil, err
	}
	return bridgeList, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BridgeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.BridgeConfig{}).
		Watches(
			&source.Kind{Type: &corev1.Node{}},
			handler.EnqueueRequestsFromMapFunc(r.BridgeConfigNodeWatchMap),
			builder.WithPredicates(r.BridgeConfigNodePredicates()),
		).
		Complete(r)
}

func (r *BridgeConfigReconciler) BridgeConfigNodeWatchMap(obj client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	bridgeConfigs := &kvnetv1alpha1.BridgeConfigList{}
	if err := r.List(context.Background(), bridgeConfigs); err != nil {
		logrus.Errorf("cannot reconcile bridge configs for node change")
		return requests
	}

	for _, bridgeConfig := range bridgeConfigs.Items {
		requests = append(requests,
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      bridgeConfig.Name,
					Namespace: bridgeConfig.Namespace,
				},
			},
		)
	}
	return requests
}

func (r *BridgeConfigReconciler) BridgeConfigNodePredicates() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return !reflect.DeepEqual(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels())
		},
	}
}
