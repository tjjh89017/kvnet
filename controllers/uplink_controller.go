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
	"os"
	"os/exec"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

// UplinkReconciler reconciles a Uplink object
type UplinkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinks/finalizers,verbs=update
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=nodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=nodes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Uplink object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *UplinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling Uplink")

	uplink := &kvnetv1alpha1.Uplink{}
	if err := r.Get(ctx, req.NamespacedName, uplink); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Uplink resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get Uplink")
		return ctrl.Result{}, err
	}

	// check this file is this node
	nodeName := os.Getenv("NODENAME")
	tmpName := strings.Split(uplink.Name, ".")
	if len(tmpName) < 2 {
		return ctrl.Result{}, fmt.Errorf("error to parse uplink name: %v", uplink.Name)
	}
	uplinkNodeName := tmpName[0]
	uplinkName := tmpName[1]
	if uplinkNodeName != nodeName {
		// not ours, skip it
		logrus.Infof("skip uplink %s", uplink.Name)
		return ctrl.Result{}, nil
	}

	if uplink.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(uplink, kvnetv1alpha1.UplinkFinalizer) {
			logrus.Info("already onRemove")
			return ctrl.Result{}, nil
		}

		if result, err := r.OnRemove(ctx, uplink, uplinkName); err != nil {
			return result, err
		}

		logrus.Info("remove finalizer")
		controllerutil.RemoveFinalizer(uplink, kvnetv1alpha1.UplinkFinalizer)
		if err := r.Update(ctx, uplink); err != nil {
			logrus.Errorf("remove finalizer fail %v", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return r.OnChange(ctx, uplink, uplinkName)
}

func (r *UplinkReconciler) OnChange(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) (ctrl.Result, error) {
	logrus.Info("OnChange")

	if err := r.findUplinkNetDev(ctx, uplink, uplinkName); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			logrus.Errorf("fail to find uplink net dev %v", err)
			return ctrl.Result{}, err
		}
		// not found, create uplink
		if err := r.addUplinkNetDev(ctx, uplink, uplinkName); err != nil {
			logrus.Errorf("fail to add uplink %s: %v", uplinkName, err)
			return ctrl.Result{}, err
		}
	}

	currentMode, err := r.getUplinkNetDevMode(ctx, uplink, uplinkName)
	if err != nil {
		logrus.Errorf("get uplink mode fail %v", err)
		return ctrl.Result{}, err
	}
	if currentMode != uplink.Spec.BondMode {
		if err := r.setUplinkNetDevDown(ctx, uplink, uplinkName); err != nil {
			logrus.Errorf("set uplink down fail %v", err)
			return ctrl.Result{}, err
		}

		if err := r.setUplinkNetDevMode(ctx, uplink, uplinkName); err != nil {
			logrus.Errorf("set uplink mode fail %v", err)
			return ctrl.Result{}, err
		}
	}

	if err := r.setUplinkNetDevSlaves(ctx, uplink, uplinkName); err != nil {
		logrus.Errorf("set slave to bond fail %v", err)
		return ctrl.Result{}, err
	}

	// set bond master to bridge
	if err := r.setUplinkMaster(uplinkName, uplink.Spec.Master); err != nil {
		logrus.Errorf("set uplink to master fail %v", err)
		return ctrl.Result{}, err
	}

	if err := r.setUplinkNetDevUp(ctx, uplink, uplinkName); err != nil {
		logrus.Errorf("fail to set uplink %s up: %v", uplinkName, err)
		return ctrl.Result{}, err
	}

	if err := r.setUplinkNodeLabel(ctx, uplink, uplinkName); err != nil {
		logrus.Errorf("fail to set node label %v", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *UplinkReconciler) OnRemove(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) (ctrl.Result, error) {
	logrus.Info("OnRemove")

	if err := r.delUplinkNetDev(ctx, uplink, uplinkName); err != nil {
		logrus.Errorf("del uplink net dev error %v", err)
		return ctrl.Result{}, err
	}

	if err := r.delUplinkNodeLabel(ctx, uplink, uplinkName); err != nil {
		logrus.Errorf("del node label fail %v", err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *UplinkReconciler) findUplinkNetDev(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) error {
	cmd := exec.Command("ip", "-j", "link", "show", "dev", uplinkName)
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *UplinkReconciler) addUplinkNetDev(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) error {
	cmd := exec.Command("ip", "link", "add", uplinkName, "type", "bond")
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *UplinkReconciler) delUplinkNetDev(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) error {
	cmd := exec.Command("ip", "link", "del", uplinkName)
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *UplinkReconciler) setUplinkNetDevUp(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) error {
	cmd := exec.Command("ip", "link", "set", uplinkName, "up")
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *UplinkReconciler) setUplinkNetDevDown(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) error {
	cmd := exec.Command("ip", "link", "set", uplinkName, "down")
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *UplinkReconciler) getUplinkNetDevMode(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) (string, error) {
	cmd := exec.Command("ip", "-j", "-d", "link", "show", uplinkName)
	cmd.Env = os.Environ()
	output, err := cmd.Output()

	if err != nil {
		return "", err
	}

	var intf []map[string]interface{}
	json.Unmarshal([]byte(output), &intf)
	linkinfo := intf[0]["linkinfo"].(map[string]interface{})
	infodata := linkinfo["info_data"].(map[string]interface{})
	mode := infodata["mode"].(string)
	return mode, nil
}

func (r *UplinkReconciler) setUplinkNetDevMode(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) error {
	cmd := exec.Command("ip", "link", "set", uplinkName, "type", "bond", "mode", uplink.Spec.BondMode)
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *UplinkReconciler) getUplinkNetDevSlaves(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) ([]string, error) {
	cmd := exec.Command("ip", "-j", "link", "show", "master", uplinkName)
	cmd.Env = os.Environ()
	output, err := cmd.Output()

	if err != nil {
		logrus.Errorf("get uplink current slave failed %v", err)
		return nil, err
	}

	var intf []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &intf); err != nil {
		logrus.Errorf("json Unmarshal failed %v", err)
		return nil, err
	}
	var slaves []string

	for _, i := range intf {
		slaves = append(slaves, i["ifname"].(string))
	}
	return slaves, nil
}

func (r *UplinkReconciler) setUplinkNetDevSlaves(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) error {
	currentSlaves, err := r.getUplinkNetDevSlaves(ctx, uplink, uplinkName)
	if err != nil {
		return err
	}

	slavesMap := make(map[string]int)
	for _, slave := range currentSlaves {
		slavesMap[slave] = 1
	}

	for _, slave := range uplink.Spec.BondSlaves {
		delete(slavesMap, slave)

		// check bondslaves has master
		master := r.getSlavesMaster(slave)
		switch master {
		case "":
			// set bondslave to down
			if err := r.setSalvesDown(slave); err != nil {
				return err
			}
			// set bondslave to bond
			if err := r.setSlavesMaster(slave, uplinkName); err != nil {
				return err
			}
			// set bondslave to up
			if err := r.setSalvesUp(slave); err != nil {
				return err
			}
		case uplinkName:
			// skip
		default:
			return fmt.Errorf("slave %s has other master", slave)
		}
	}

	// delete remain slave
	for slave, _ := range slavesMap {
		if err := r.setSlavesMaster(slave, ""); err != nil {
			logrus.Errorf("faile to set slave to nomaster %v", err)
			return err
		}
	}

	return nil
}

func (r *UplinkReconciler) getSlavesMaster(slave string) string {
	cmd := exec.Command("ip", "-j", "link", "show", slave)
	cmd.Env = os.Environ()
	output, err := cmd.Output()

	if err != nil {
		return ""
	}

	var intf []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &intf); err != nil {
		logrus.Errorf("json Unmarshal failed %v", err)
		return ""
	}
	if m, ok := intf[0]["master"]; ok {
		return m.(string)
	}
	return ""
}

func (r *UplinkReconciler) setSlavesMaster(slave string, uplink string) error {
	cmd := exec.Command("ip", "link", "set", slave, "nomaster")
	if uplink != "" {
		cmd = exec.Command("ip", "link", "set", slave, "master", uplink)
	}
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *UplinkReconciler) setSalvesDown(slave string) error {
	cmd := exec.Command("ip", "link", "set", slave, "down")
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *UplinkReconciler) setSalvesUp(slave string) error {
	cmd := exec.Command("ip", "link", "set", slave, "up")
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *UplinkReconciler) setUplinkMaster(uplink string, master string) error {
	cmd := exec.Command("ip", "link", "set", uplink, "nomaster")
	if master != "" {
		cmd = exec.Command("ip", "link", "set", uplink, "master", master)
	}
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *UplinkReconciler) setUplinkNodeLabel(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) error {
	nodeName := os.Getenv("NODENAME")
	node := &corev1.Node{}
	if err := r.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		logrus.Errorf("get node %s failed %v", nodeName, err)
		return err
	}

	nodeCopy := node.DeepCopy()
	if nodeCopy.Labels == nil {
		nodeCopy.Labels = make(map[string]string)
	}

	nodeCopy.Labels[kvnetv1alpha1.UplinkNodeLabel+uplinkName] = r.checkIfMasterIsBridge(uplink.Spec.Master)
	if !reflect.DeepEqual(node, nodeCopy) {
		if err := r.Update(ctx, nodeCopy); err != nil {
			logrus.Errorf("update node label failed %v", err)
			return err
		}
	}
	return nil
}

func (r *UplinkReconciler) delUplinkNodeLabel(ctx context.Context, uplink *kvnetv1alpha1.Uplink, uplinkName string) error {
	nodeName := os.Getenv("NODENAME")
	node := &corev1.Node{}
	if err := r.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		logrus.Errorf("get node %s failed %v", nodeName, err)
		return err
	}

	nodeCopy := node.DeepCopy()
	if nodeCopy.Labels == nil {
		// no label
		return nil
	}

	delete(nodeCopy.Labels, kvnetv1alpha1.UplinkNodeLabel+uplinkName)
	if !reflect.DeepEqual(node, nodeCopy) {
		if err := r.Update(ctx, nodeCopy); err != nil {
			logrus.Errorf("update node label failed %v", err)
			return err
		}
	}
	return nil
}

func (r *UplinkReconciler) checkIfMasterIsBridge(master string) string {
	// if master is bridge, return bridge name
	// otherwise empty string
	cmd := exec.Command("ip", "-j", "-d", "link", "show", master)
	cmd.Env = os.Environ()
	output, err := cmd.Output()

	if err != nil {
		return ""
	}

	var intf []map[string]interface{}
	json.Unmarshal([]byte(output), &intf)
	linkinfo := intf[0]["linkinfo"].(map[string]interface{})
	infokind := linkinfo["info_kind"].(string)

	if infokind == "bridge" {
		return master
	}
	return ""
}

// SetupWithManager sets up the controller with the Manager.
func (r *UplinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.Uplink{}).
		Watches(
			&source.Kind{Type: &kvnetv1alpha1.Bridge{}},
			handler.EnqueueRequestsFromMapFunc(r.UplinkBridgeWatchMap),
		).
		Complete(r)
}

func (r *UplinkReconciler) UplinkBridgeWatchMap(obj client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	uplinks := &kvnetv1alpha1.UplinkList{}
	if err := r.List(context.Background(), uplinks); err != nil {
		logrus.Errorf("cannot reconcile uplink for bridge change")
		return requests
	}

	for _, uplink := range uplinks.Items {
		requests = append(requests,
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      uplink.Name,
					Namespace: uplink.Namespace,
				},
			},
		)
	}
	return requests
}
