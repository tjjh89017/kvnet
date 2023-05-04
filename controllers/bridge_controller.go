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
	"os"
	"os/exec"
	"strings"

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

	// check this file is this node
	nodeName := os.Getenv("NODENAME")
	tmpName := strings.Split(bridge.Name, ".")
	if len(tmpName) < 2 {
		return ctrl.Result{}, fmt.Errorf("error to parse bridge name: %v", bridge.Name)
	}
	bridgeNodeName := tmpName[0]
	bridgeName := tmpName[1]
	if bridgeNodeName != nodeName {
		// not ours, skip it
		logrus.Infof("skip bridge %s", bridge.Name)
		return ctrl.Result{}, nil
	}

	if bridge.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(bridge, kvnetv1alpha1.BridgeFinalizer) {
			logrus.Info("already onRemove")
			return ctrl.Result{}, nil
		}

		if result, err := r.OnRemove(ctx, bridge, bridgeName); err != nil {
			return result, err
		}

		logrus.Info("remove finalizer")
		controllerutil.RemoveFinalizer(bridge, kvnetv1alpha1.BridgeFinalizer)
		if err := r.Update(ctx, bridge); err != nil {
			logrus.Errorf("remove finalizer fail %v", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return r.OnChange(ctx, bridge, bridgeName)
}

func (r *BridgeReconciler) OnChange(ctx context.Context, bridge *kvnetv1alpha1.Bridge, bridgeName string) (ctrl.Result, error) {
	logrus.Info("OnChange")

	if err := r.findBridgeNetDev(ctx, bridge, bridgeName); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			logrus.Errorf("fail to find bridge net dev %v", err)
			return ctrl.Result{}, err
		}
		// not found, create bridge
		if err := r.addBridgeNetDev(ctx, bridge, bridgeName); err != nil {
			logrus.Errorf("fail to add bridge %s: %v", bridgeName, err)
			return ctrl.Result{}, err
		}
	}

	// override the config always
	// vlan filtering
	if err := r.setBridgeNetDevVlanFiltering(ctx, bridge, bridgeName); err != nil {
		logrus.Errorf("fail to set vlanfiltering %s: %v", bridgeName, err)
		return ctrl.Result{}, err
	}

	if err := r.setBridgeNetDevUp(ctx, bridge, bridgeName); err != nil {
		logrus.Errorf("fail to set bridge %s up: %v", bridgeName, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *BridgeReconciler) OnRemove(ctx context.Context, bridge *kvnetv1alpha1.Bridge, bridgeName string) (ctrl.Result, error) {
	logrus.Info("OnRemove")

	err := r.delBridgeNetDev(ctx, bridge, bridgeName)
	if err != nil {
		logrus.Errorf("del bridge net dev error %v", err)
	}
	return ctrl.Result{}, err
}

func (r *BridgeReconciler) findBridgeNetDev(ctx context.Context, bridge *kvnetv1alpha1.Bridge, bridgeName string) error {
	cmd := exec.Command("ip", "-j", "link", "show", "dev", bridgeName)
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *BridgeReconciler) addBridgeNetDev(ctx context.Context, bridge *kvnetv1alpha1.Bridge, bridgeName string) error {
	cmd := exec.Command("ip", "link", "add", bridgeName, "type", "bridge")
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *BridgeReconciler) delBridgeNetDev(ctx context.Context, bridge *kvnetv1alpha1.Bridge, bridgeName string) error {
	cmd := exec.Command("ip", "link", "del", bridgeName)
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *BridgeReconciler) setBridgeNetDevUp(ctx context.Context, bridge *kvnetv1alpha1.Bridge, bridgeName string) error {
	cmd := exec.Command("ip", "link", "set", bridgeName, "up")
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *BridgeReconciler) setBridgeNetDevVlanFiltering(ctx context.Context, bridge *kvnetv1alpha1.Bridge, bridgeName string) error {
	enable := "0"
	if bridge.Spec.VlanFiltering {
		enable = "1"
	}
	cmd := exec.Command("ip", "link", "set", bridgeName, "type", "bridge", "vlan_filtering", enable)
	cmd.Env = os.Environ()
	return cmd.Run()
}

// SetupWithManager sets up the controller with the Manager.
func (r *BridgeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.Bridge{}).
		Complete(r)
}
