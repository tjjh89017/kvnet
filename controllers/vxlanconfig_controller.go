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

// VxlanConfigReconciler reconciles a VxlanConfig object
type VxlanConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlanconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlanconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlanconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlans/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=nodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=nodes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VxlanConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *VxlanConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling VxlanConfig")

	vxlanConfig := &kvnetv1alpha1.VxlanConfig{}
	if err := r.Get(ctx, req.NamespacedName, vxlanConfig); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("VxlanConfig resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get VxlanConfig")
		return ctrl.Result{}, err
	}

	if vxlanConfig.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(vxlanConfig, kvnetv1alpha1.VxlanConfigFinalizer) {
			logrus.Info("already onRemove")
			return ctrl.Result{}, nil
		}

		if result, err := r.OnRemove(ctx, vxlanConfig); err != nil {
			return result, err
		}

		controllerutil.RemoveFinalizer(vxlanConfig, kvnetv1alpha1.VxlanConfigFinalizer)
		if err := r.Update(ctx, vxlanConfig); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return r.OnChange(ctx, vxlanConfig)
}

// SetupWithManager sets up the controller with the Manager.
func (r *VxlanConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.VxlanConfig{}).
		Watches(
			&source.Kind{Type: &corev1.Node{}},
			handler.EnqueueRequestsFromMapFunc(r.VxlanConfigNodeWatchMap),
			builder.WithPredicates(r.VxlanConfigNodePredicates()),
		).
		Complete(r)
}

func (r *VxlanConfigReconciler) OnChange(ctx context.Context, vxlanConfig *kvnetv1alpha1.VxlanConfig) (ctrl.Result, error) {
	logrus.Info("OnChange")

	selector, err := metav1.LabelSelectorAsSelector(vxlanConfig.Spec.NodeSelector)
	if err != nil {
		logrus.Errorf("NodeSelector parse error %v", err)
		return ctrl.Result{}, err
	}

	nodes := &corev1.NodeList{}
	if err := r.List(ctx, nodes, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, err
	}

	vxlans, err := r.findVxlan(ctx, "", vxlanConfig)
	if err != nil {
		logrus.Errorf("vxlan list fail %v", err)
		return ctrl.Result{}, err
	}

	nodeMap := make(map[string]int)
	for _, node := range nodes.Items {
		// set 1 to mark we should create but we still not
		nodeMap[node.Name] = 1
	}
	for _, vxlan := range vxlans.Items {
		// get node name
		name := strings.Split(vxlan.Name, ".")[0]
		// already have bridge
		nodeMap[name] |= 2
	}

	for nodeName, state := range nodeMap {
		switch state {
		case 1:
			// need create
			if err := r.addVxlan(ctx, nodeName, vxlanConfig); err != nil {
				return ctrl.Result{}, err
			}
		case 2:
			// need delete
			if err := r.delVxlan(ctx, nodeName, vxlanConfig); err != nil {
				return ctrl.Result{}, err
			}
		case 3:
			// check spec is the same
			// check vxlan belongs to this vxlanConfig first
			if err := r.updateVxlan(ctx, nodeName, vxlanConfig); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *VxlanConfigReconciler) OnRemove(ctx context.Context, vxlanConfig *kvnetv1alpha1.VxlanConfig) (ctrl.Result, error) {
	logrus.Info("OnRemove")

	vxlanList, err := r.findVxlan(ctx, "", vxlanConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	logrus.Info("remove vxlan")
	// TODO change to DeleteAllOf
	for _, vxlan := range vxlanList.Items {
		if err := r.Delete(ctx, &vxlan); err != nil {
			logrus.Errorf("delete fail %v", err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *VxlanConfigReconciler) addVxlan(ctx context.Context, nodeName string, vxlanConfig *kvnetv1alpha1.VxlanConfig) error {
	vxlan := &kvnetv1alpha1.Vxlan{
		ObjectMeta: vxlanConfig.Spec.Template.ObjectMeta,
		Spec:       vxlanConfig.Spec.Template.Spec,
	}
	vxlan.Namespace = vxlanConfig.Namespace
	vxlan.Name = fmt.Sprintf("%s.vx%d", nodeName, vxlanConfig.Spec.VxlanID)

	if vxlan.Labels == nil {
		vxlan.Labels = make(map[string]string)
	}
	vxlan.Labels[kvnetv1alpha1.VxlanConfigNamespaceLabel] = vxlanConfig.Namespace
	vxlan.Labels[kvnetv1alpha1.VxlanConfigNameLabel] = vxlanConfig.Name
	vxlan.Labels[kvnetv1alpha1.VxlanConfigNodeLabel] = nodeName

	return r.Create(ctx, vxlan)
}

func (r *VxlanConfigReconciler) delVxlan(ctx context.Context, nodeName string, vxlanConfig *kvnetv1alpha1.VxlanConfig) error {
	vxlans, err := r.findVxlan(ctx, nodeName, vxlanConfig)
	if err != nil {
		return err
	}
	for _, vxlan := range vxlans.Items {
		if err := r.Delete(ctx, &vxlan); err != nil {
			return err
		}
	}
	return nil
}

func (r *VxlanConfigReconciler) updateVxlan(ctx context.Context, nodeName string, vxlanConfig *kvnetv1alpha1.VxlanConfig) error {
	vxlan := &kvnetv1alpha1.Vxlan{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: vxlanConfig.Namespace,
		Name:      fmt.Sprintf("%s.vx%d", nodeName, vxlanConfig.Spec.VxlanID),
	}, vxlan); err != nil {
		return err
	}

	if vxlan.Labels[kvnetv1alpha1.VxlanConfigNamespaceLabel] != vxlanConfig.Namespace ||
		vxlan.Labels[kvnetv1alpha1.VxlanConfigNameLabel] != vxlanConfig.Name {
		return fmt.Errorf("vxlan %s is not belong to this vxlanConfig", vxlan.Name)
	}

	vxlanCopy := vxlan.DeepCopy()
	vxlanCopy.Spec = *vxlanConfig.Spec.Template.Spec.DeepCopy()
	// It should be only one
	if !reflect.DeepEqual(vxlan, vxlanCopy) {
		if err := r.Update(ctx, vxlanCopy); err != nil {
			return err
		}
	}
	return nil
}

func (r *VxlanConfigReconciler) findVxlan(ctx context.Context, nodeName string, vxlanConfig *kvnetv1alpha1.VxlanConfig) (*kvnetv1alpha1.VxlanList, error) {
	vxlanList := &kvnetv1alpha1.VxlanList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			kvnetv1alpha1.VxlanConfigNamespaceLabel: vxlanConfig.Namespace,
			kvnetv1alpha1.VxlanConfigNameLabel:      vxlanConfig.Name,
		},
	}

	if nodeName != "" {
		opts = append(opts, client.MatchingLabels{
			kvnetv1alpha1.VxlanConfigNodeLabel: nodeName,
		})
	}

	if err := r.List(ctx, vxlanList, opts...); err != nil {
		return nil, err
	}
	return vxlanList, nil
}

func (r *VxlanConfigReconciler) VxlanConfigNodeWatchMap(obj client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	vxlanConfigs := &kvnetv1alpha1.VxlanConfigList{}
	if err := r.List(context.Background(), vxlanConfigs); err != nil {
		logrus.Errorf("cannot reconcile vxlan configs for node change")
		return requests
	}

	for _, vxlanConfig := range vxlanConfigs.Items {
		requests = append(requests,
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      vxlanConfig.Name,
					Namespace: vxlanConfig.Namespace,
				},
			},
		)
	}
	return requests
}

func (r *VxlanConfigReconciler) VxlanConfigNodePredicates() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return !reflect.DeepEqual(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels())
		},
	}
}
