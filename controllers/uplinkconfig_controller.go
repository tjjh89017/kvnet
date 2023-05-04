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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// UplinkConfigReconciler reconciles a UplinkConfig object
type UplinkConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinkconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinkconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinkconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinks/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=nodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=nodes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the UplinkConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *UplinkConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling UplinkConfig")

	uplinkConfig := &kvnetv1alpha1.UplinkConfig{}
	if err := r.Get(ctx, req.NamespacedName, uplinkConfig); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("UplinkConfig resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get UplinkConfig")
		return ctrl.Result{}, err
	}

	if uplinkConfig.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(uplinkConfig, kvnetv1alpha1.UplinkConfigFinalizer) {
			logrus.Info("already onRemove")
			return ctrl.Result{}, nil
		}

		if result, err := r.OnRemove(ctx, uplinkConfig); err != nil {
			return result, err
		}

		controllerutil.RemoveFinalizer(uplinkConfig, kvnetv1alpha1.UplinkConfigFinalizer)
		if err := r.Update(ctx, uplinkConfig); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return r.OnChange(ctx, uplinkConfig)
}

func (r *UplinkConfigReconciler) OnChange(ctx context.Context, uplinkConfig *kvnetv1alpha1.UplinkConfig) (ctrl.Result, error) {
	logrus.Info("OnChange")

	selector, err := metav1.LabelSelectorAsSelector(uplinkConfig.Spec.NodeSelector)
	if err != nil {
		logrus.Errorf("NodeSelector parse error %v", err)
		return ctrl.Result{}, err
	}

	nodes := &corev1.NodeList{}
	if err := r.List(ctx, nodes, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, err
	}

	uplinks, err := r.findUplink(ctx, "", uplinkConfig)
	if err != nil {
		logrus.Errorf("uplink list fail %v", err)
		return ctrl.Result{}, err
	}

	nodeMap := make(map[string]int)
	for _, node := range nodes.Items {
		// set 1 to mark we should create but we still not
		nodeMap[node.Name] = 1
	}
	for _, uplink := range uplinks.Items {
		// get node name
		name := strings.Split(uplink.Name, ".")[0]
		// already have bridge
		nodeMap[name] |= 2
	}

	for nodeName, state := range nodeMap {
		switch state {
		case 1:
			// need create
			if err := r.addUplink(ctx, nodeName, uplinkConfig); err != nil {
				return ctrl.Result{}, err
			}
		case 2:
			// need delete
			if err := r.delUplink(ctx, nodeName, uplinkConfig); err != nil {
				return ctrl.Result{}, err
			}
		case 3:
			// check spec is the same
			// check uplink belongs to this uplinkConfig first
			if err := r.updateUplink(ctx, nodeName, uplinkConfig); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *UplinkConfigReconciler) OnRemove(ctx context.Context, uplinkConfig *kvnetv1alpha1.UplinkConfig) (ctrl.Result, error) {
	logrus.Info("OnRemove")

	uplinkList, err := r.findUplink(ctx, "", uplinkConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	logrus.Info("remove uplink")
	// TODO change to DeleteAllOf
	for _, uplink := range uplinkList.Items {
		if err := r.Delete(ctx, &uplink); err != nil {
			logrus.Errorf("delete fail %v", err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *UplinkConfigReconciler) addUplink(ctx context.Context, nodeName string, uplinkConfig *kvnetv1alpha1.UplinkConfig) error {
	uplink := &kvnetv1alpha1.Uplink{
		ObjectMeta: uplinkConfig.Spec.Template.ObjectMeta,
		Spec:       uplinkConfig.Spec.Template.Spec,
	}
	uplink.Namespace = uplinkConfig.Namespace
	uplink.Name = fmt.Sprintf("%s.%s", nodeName, uplinkConfig.Spec.BondName)

	if uplink.Labels == nil {
		uplink.Labels = make(map[string]string)
	}
	uplink.Labels[kvnetv1alpha1.UplinkConfigNamespaceLabel] = uplinkConfig.Namespace
	uplink.Labels[kvnetv1alpha1.UplinkConfigNameLabel] = uplinkConfig.Name
	uplink.Labels[kvnetv1alpha1.UplinkConfigNodeLabel] = nodeName

	return r.Create(ctx, uplink)
}

func (r *UplinkConfigReconciler) delUplink(ctx context.Context, nodeName string, uplinkConfig *kvnetv1alpha1.UplinkConfig) error {
	uplinks, err := r.findUplink(ctx, nodeName, uplinkConfig)
	if err != nil {
		return err
	}
	for _, uplink := range uplinks.Items {
		if err := r.Delete(ctx, &uplink); err != nil {
			return err
		}
	}
	return nil
}

func (r *UplinkConfigReconciler) updateUplink(ctx context.Context, nodeName string, uplinkConfig *kvnetv1alpha1.UplinkConfig) error {
	uplink := &kvnetv1alpha1.Uplink{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: uplinkConfig.Namespace,
		Name:      fmt.Sprintf("%s.%s", nodeName, uplinkConfig.Spec.BondName),
	}, uplink); err != nil {
		return err
	}

	if uplink.Labels[kvnetv1alpha1.UplinkConfigNamespaceLabel] != uplinkConfig.Namespace ||
		uplink.Labels[kvnetv1alpha1.UplinkConfigNameLabel] != uplinkConfig.Name {
		return fmt.Errorf("uplink %s is not belong to this uplinkConfig", uplink.Name)
	}
	// It should be only one
	if !reflect.DeepEqual(uplink.Spec, uplinkConfig.Spec.Template.Spec) {
		uplink.Spec = uplinkConfig.Spec.Template.Spec
		if err := r.Update(ctx, uplink); err != nil {
			return err
		}
	}
	return nil
}

func (r *UplinkConfigReconciler) findUplink(ctx context.Context, nodeName string, uplinkConfig *kvnetv1alpha1.UplinkConfig) (*kvnetv1alpha1.UplinkList, error) {
	uplinkList := &kvnetv1alpha1.UplinkList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			kvnetv1alpha1.UplinkConfigNamespaceLabel: uplinkConfig.Namespace,
			kvnetv1alpha1.UplinkConfigNameLabel:      uplinkConfig.Name,
		},
	}

	if nodeName != "" {
		opts = append(opts, client.MatchingLabels{
			kvnetv1alpha1.UplinkConfigNodeLabel: nodeName,
		})
	}

	if err := r.List(ctx, uplinkList, opts...); err != nil {
		return nil, err
	}
	return uplinkList, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UplinkConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.UplinkConfig{}).
		Watches(
			&source.Kind{Type: &corev1.Node{}},
			handler.EnqueueRequestsFromMapFunc(r.UplinkConfigNodeWatchMap),
		).
		Complete(r)
}

func (r *UplinkConfigReconciler) UplinkConfigNodeWatchMap(obj client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	uplinkConfigs := &kvnetv1alpha1.UplinkConfigList{}
	if err := r.List(context.Background(), uplinkConfigs); err != nil {
		logrus.Errorf("cannot reconcile uplink configs for node change")
		return requests
	}

	for _, uplinkConfig := range uplinkConfigs.Items {
		requests = append(requests,
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      uplinkConfig.Name,
					Namespace: uplinkConfig.Namespace,
				},
			},
		)
	}
	return requests
}
