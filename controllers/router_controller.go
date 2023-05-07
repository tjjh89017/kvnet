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

	_ "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// RouterReconciler reconciles a Router object
type RouterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=routers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=routers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=routers/finalizers,verbs=update
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=subnets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=subnets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=subnets/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Router object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *RouterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling Router")

	router := &kvnetv1alpha1.Router{}
	if err := r.Get(ctx, req.NamespacedName, router); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Router resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get Router")
		return ctrl.Result{}, err
	}

	if router.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(router, kvnetv1alpha1.RouterFinalizer) {
			logrus.Info("already onRemove")
			return ctrl.Result{}, nil
		}

		if result, err := r.OnRemove(ctx, router); err != nil {
			return result, err
		}

		controllerutil.RemoveFinalizer(router, kvnetv1alpha1.RouterFinalizer)
		if err := r.Update(ctx, router); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return r.OnChange(ctx, router)
}

func (r *RouterReconciler) OnChange(ctx context.Context, router *kvnetv1alpha1.Router) (ctrl.Result, error) {
	logrus.Infof("Router OnChange")

	// check subnets to update labels
	if err := r.updateSubnetLabel(ctx, router); err != nil {
		return ctrl.Result{}, err
	}

	// override all subnets
	subnets, err := r.getSubnetListWithRouter(ctx, router)
	if err != nil {
		logrus.Errorf("get updated subnet failed %v", err)
		return ctrl.Result{}, err
	}

	// cretae or update configMap
	// TODO custom route for FRR
	_ = subnets

	// create or update deployment

	return ctrl.Result{}, nil
}

func (r *RouterReconciler) OnRemove(ctx context.Context, router *kvnetv1alpha1.Router) (ctrl.Result, error) {
	logrus.Infof("Router OnRemove")

	// remove all deployment

	// remove all configMap

	// remove subnets labels

	return ctrl.Result{}, nil
}

func (r *RouterReconciler) updateConfigMap(ctx context.Context, router *kvnetv1alpha1.Router, subnets *kvnetv1alpha1.SubnetList) error {
	return nil
}

func (r *RouterReconciler) getSubnetListWithRouter(ctx context.Context, router *kvnetv1alpha1.Router) (*kvnetv1alpha1.SubnetList, error) {
	// get all labels with current router name
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			kvnetv1alpha1.RouterSubnetOwnerLabel: router.Name,
		},
	})
	if err != nil {
		logrus.Errorf("error to create router selector, %v", err)
		return nil, err
	}

	subnets := &kvnetv1alpha1.SubnetList{}
	if err := r.List(ctx, subnets, client.InNamespace(router.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		logrus.Errorf("get subnets failed %v", err)
		return nil, err
	}

	return subnets, nil
}

func (r *RouterReconciler) updateSubnetLabel(ctx context.Context, router *kvnetv1alpha1.Router) error {
	subnets, err := r.getSubnetListWithRouter(ctx, router)
	if err != nil {
		return err
	}

	subnetMap := make(map[string]int)
	for _, subnet := range router.Spec.Subnets {
		// set 1 to mark we should create but we still not
		subnetMap[subnet.Name] = 1
	}
	for _, subnet := range subnets.Items {
		// already have this subnet
		subnetMap[subnet.Name] |= 2
	}

	for subnetName, state := range subnetMap {
		switch state {
		case 1:
			// need create
			subnet := &kvnetv1alpha1.Subnet{}
			if err := r.Get(ctx, types.NamespacedName{Namespace: router.Namespace, Name: subnetName}, subnet); err != nil {
				logrus.Errorf("get subnet to add router label failed %v", err)
				return err
			}

			if subnet.Labels == nil {
				subnet.Labels = make(map[string]string)
			}
			subnet.Labels[kvnetv1alpha1.RouterSubnetOwnerLabel] = router.Name
			if err := r.Update(ctx, subnet); err != nil {
				logrus.Errorf("add router label to subnet fail %v", err)
				return err
			}
		case 2:
			// need delete
			subnet := &kvnetv1alpha1.Subnet{}
			if err := r.Get(ctx, types.NamespacedName{Namespace: router.Namespace, Name: subnetName}, subnet); err != nil {
				logrus.Errorf("get subnet to delete router label failed %v", err)
				return err
			}

			delete(subnet.Labels, kvnetv1alpha1.RouterSubnetOwnerLabel)
			if err := r.Update(ctx, subnet); err != nil {
				logrus.Errorf("del router label to subnet fail %v", err)
				return err
			}
		case 3:
			// override config
			// do nothing
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RouterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.Router{}).
		Complete(r)
}
