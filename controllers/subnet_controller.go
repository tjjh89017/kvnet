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
	"reflect"
	"strings"

	nadv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// SubnetReconciler reconciles a Subnet object
type SubnetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=subnets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=subnets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=subnets/finalizers,verbs=update
//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Subnet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *SubnetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling Subnet")

	subnet := &kvnetv1alpha1.Subnet{}
	if err := r.Get(ctx, req.NamespacedName, subnet); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Subnet resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get Subnet")
		return ctrl.Result{}, err
	}

	if subnet.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(subnet, kvnetv1alpha1.SubnetFinalizer) {
			logrus.Info("already onRemove")
			return ctrl.Result{}, nil
		}

		if result, err := r.OnRemove(ctx, subnet); err != nil {
			return result, err
		}

		controllerutil.RemoveFinalizer(subnet, kvnetv1alpha1.SubnetFinalizer)
		if err := r.Update(ctx, subnet); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return r.OnChange(ctx, subnet)
}

func (r *SubnetReconciler) OnChange(ctx context.Context, subnet *kvnetv1alpha1.Subnet) (ctrl.Result, error) {
	logrus.Infof("Subnet OnChange")
	subnetCopy := subnet.DeepCopy()

	if subnetCopy.Labels == nil {
		subnetCopy.Labels = make(map[string]string)
	}

	nad := &nadv1.NetworkAttachmentDefinition{}
	network := subnetCopy.Spec.Network
	if err := r.Get(ctx, types.NamespacedName{Namespace: subnetCopy.Namespace, Name: network}, nad); err != nil {
		logrus.Errorf("get nad failed %v", err)
		return ctrl.Result{}, err
	}

	// remove all labels
	for k, _ := range subnetCopy.Labels {
		if strings.HasPrefix(k, kvnetv1alpha1.BridgeNodeLabel) {
			delete(subnetCopy.Labels, k)
		}
	}

	// add labels
	for k, _ := range nad.Labels {
		if strings.HasPrefix(k, kvnetv1alpha1.BridgeNodeLabel) {
			// setup a label
			subnetCopy.Labels[k] = ""
		}
	}

	if !reflect.DeepEqual(subnet, subnetCopy) {
		if err := r.Update(ctx, subnetCopy); err != nil {
			logrus.Errorf("update subnet labels failed %v", err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *SubnetReconciler) OnRemove(ctx context.Context, subnet *kvnetv1alpha1.Subnet) (ctrl.Result, error) {
	logrus.Infof("Subnet OnRemove")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SubnetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.Subnet{}).
		Complete(r)
}
