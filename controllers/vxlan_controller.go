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
	"strconv"
	"strings"
	"time"

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

// VxlanReconciler reconciles a Vxlan object
type VxlanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlans/finalizers,verbs=update
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=bridges/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=nodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=nodes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Vxlan object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *VxlanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling Vxlan")

	vxlan := &kvnetv1alpha1.Vxlan{}
	if err := r.Get(ctx, req.NamespacedName, vxlan); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Vxlan resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get Vxlan")
		return ctrl.Result{}, err
	}

	// check this file is this node
	nodeName := os.Getenv("NODENAME")
	tmpName := strings.Split(vxlan.Name, ".")
	if len(tmpName) < 2 {
		return ctrl.Result{}, fmt.Errorf("error to parse vxlan name: %v", vxlan.Name)
	}
	vxlanNodeName := tmpName[0]
	vxlanName := tmpName[1]
	if vxlanNodeName != nodeName {
		logrus.Infof("update vxlan remote ip")
		if err := r.setVxlanNetDevRemote(ctx, vxlan, vxlanName); err != nil {
			logrus.Errorf("update vxlan remote ip fail %v", err)
			return ctrl.Result{}, err
		}
		// not ours, skip it
		logrus.Infof("skip vxlan %s", vxlan.Name)
		return ctrl.Result{}, nil
	}

	if vxlan.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(vxlan, kvnetv1alpha1.VxlanFinalizer) {
			logrus.Info("already onRemove")
			return ctrl.Result{}, nil
		}

		if result, err := r.OnRemove(ctx, vxlan, vxlanName); err != nil {
			return result, err
		}

		logrus.Info("remove finalizer")
		controllerutil.RemoveFinalizer(vxlan, kvnetv1alpha1.VxlanFinalizer)
		if err := r.Update(ctx, vxlan); err != nil {
			logrus.Errorf("remove finalizer fail %v", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return r.OnChange(ctx, vxlan, vxlanName)
}

func (r *VxlanReconciler) OnChange(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) (ctrl.Result, error) {
	logrus.Info("vxlan OnChange")
	vxlanCopy := vxlan.DeepCopy()

	// find vxlan netdev
	// not found add netdev
	if err := r.findVxlanNetDev(ctx, vxlanCopy, vxlanName); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			logrus.Errorf("fail to find vxlan net dev %v", err)
			return ctrl.Result{}, err
		}

		// not found, create vxlan
		if err := r.addVxlanNetDev(ctx, vxlanCopy, vxlanName); err != nil {
			logrus.Errorf("fail to add vxlan %s: %v", vxlanName, err)
			return ctrl.Result{}, err
		}
	}

	// set vxlan attr
	if err := r.setVxlanNetDevLocalDev(ctx, vxlanCopy, vxlanName); err != nil {
		logrus.Errorf("fail to set vxlan local ip %v", err)
		return ctrl.Result{}, err
	}

	if err := r.setVxlanNetDevMTU(ctx, vxlanCopy, vxlanName); err != nil {
		logrus.Errorf("fail to set vxlan mtu %v", err)
		return ctrl.Result{}, err
	}

	// set vxlan master
	if err := r.setVxlanNetDevMaster(ctx, vxlanCopy, vxlanName); err != nil {
		logrus.Errorf("fail to set vxlan master %v", err)
		return ctrl.Result{}, err
	}

	// set vxlan up
	if err := r.setVxlanNetDevUp(ctx, vxlanCopy, vxlanName); err != nil {
		logrus.Errorf("fail to set vxlan up %v", err)
		return ctrl.Result{}, err
	}

	// set vxlan node label
	if err := r.setVxlanNodeLabel(ctx, vxlanCopy, vxlanName); err != nil {
		logrus.Errorf("fail to set vxlan node label %v", err)
		return ctrl.Result{}, err
	}

	// update local ip to status
	if !reflect.DeepEqual(vxlan.Status, vxlanCopy.Status) {
		if err := r.updateStatus(ctx, vxlan, vxlanCopy); err != nil {
			logrus.Errorf("update vxlan status fail %v", err)
			return ctrl.Result{}, err
		}
		// local IP is updated, need to notify all daemonset again to set up
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *VxlanReconciler) OnRemove(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) (ctrl.Result, error) {
	logrus.Infof("vxlan OnRemove")

	if err := r.delVxlanNetDev(ctx, vxlan, vxlanName); err != nil {
		logrus.Errorf("del vxlan net dev error %v", err)
		return ctrl.Result{}, err
	}

	// delete node label
	if err := r.delVxlanNodeLabel(ctx, vxlan, vxlanName); err != nil {
		logrus.Errorf("del vxlan node label fail %v", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *VxlanReconciler) updateStatus(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanCopy *kvnetv1alpha1.Vxlan) error {
	if err := r.Status().Update(ctx, vxlanCopy); err != nil {
		logrus.Errorf("update vxlan status fail %v", err)
		return err
	}
	return nil
}

func (r *VxlanReconciler) findVxlanNetDev(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) error {
	cmd := exec.Command("ip", "link", "show", "dev", vxlanName)
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *VxlanReconciler) addVxlanNetDev(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) error {
	if err := r.findVxlanNetDev(ctx, vxlan, vxlanName); err == nil {
		logrus.Infof("vxlan %s is existing. skip", vxlanName)
		return nil
	}
	// vx<ID in dec>
	vxlanIDstr := vxlanName[2:]
	cmd := exec.Command("ip", "link", "add", vxlanName, "type", "vxlan", "id", vxlanIDstr, "dstport", "4789")
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *VxlanReconciler) setVxlanNetDevLocalDev(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) error {
	localIP := vxlan.Spec.LocalIP
	if localIP == "" {
		l, err := r.getDefaultRouteIP(ctx, vxlan, vxlanName)
		if err != nil {
			logrus.Errorf("get default route ip failed %v", err)
			return err
		}
		localIP = l
	}

	cmd := exec.Command("ip", "link", "set", vxlanName, "type", "vxlan", "local", localIP)
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		logrus.Errorf("vxlan set local fail %v", err)
		return err
	}

	// update status local IP
	vxlan.Status.LocalIP = localIP

	dev := vxlan.Spec.Dev
	if dev == "" {
		// TODO get default route interface as dev
	} else {
		cmd := exec.Command("ip", "link", "set", vxlanName, "type", "vxlan", "dev", dev)
		cmd.Env = os.Environ()
		if err := cmd.Run(); err != nil {
			logrus.Errorf("vxlan set dev fail %v", err)
			return err
		}
	}

	return nil
}

func (r *VxlanReconciler) getDefaultRouteIP(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) (string, error) {
	// TODO fix this
	cmd := exec.Command("ip", "-j", "route", "get", "8.8.8.8")
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var tmpInfo []map[string]interface{}
	if err = json.Unmarshal([]byte(output), &tmpInfo); err != nil {
		logrus.Errorf("json Unmarshal fail %v", err)
		return "", err
	}
	return tmpInfo[0]["prefsrc"].(string), nil
}

func (r *VxlanReconciler) setVxlanNetDevMTU(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) error {
	mtu := "1450"
	if vxlan.Spec.MTU != 0 {
		mtu = strconv.Itoa(vxlan.Spec.MTU)
	}

	cmd := exec.Command("ip", "link", "set", vxlanName, "mtu", mtu)
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *VxlanReconciler) setVxlanNetDevMaster(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) error {
	cmd := exec.Command("ip", "link", "set", vxlanName, "nomaster")
	master := vxlan.Spec.Master
	if master != "" {
		cmd = exec.Command("ip", "link", "set", vxlanName, "master", master)
	}
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *VxlanReconciler) setVxlanNetDevUp(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) error {
	cmd := exec.Command("ip", "link", "set", vxlanName, "up")
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *VxlanReconciler) setVxlanNodeLabel(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) error {
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

	nodeCopy.Labels[kvnetv1alpha1.VxlanNodeLabel+vxlanName] = r.checkIfMasterIsBridge(vxlan.Spec.Master)
	if !reflect.DeepEqual(node, nodeCopy) {
		if err := r.Update(ctx, nodeCopy); err != nil {
			logrus.Errorf("update node label failed %v", err)
			return err
		}
	}
	return nil
}

func (r *VxlanReconciler) checkIfMasterIsBridge(master string) string {
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

func (r *VxlanReconciler) delVxlanNetDev(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) error {
	if err := r.findVxlanNetDev(ctx, vxlan, vxlanName); err != nil {
		logrus.Infof("vxlan %s is not found. skip", vxlanName)
		return nil
	}

	cmd := exec.Command("ip", "link", "del", vxlanName)
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (r *VxlanReconciler) delVxlanNodeLabel(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) error {
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

	delete(nodeCopy.Labels, kvnetv1alpha1.VxlanNodeLabel+vxlanName)
	if !reflect.DeepEqual(node, nodeCopy) {
		if err := r.Update(ctx, nodeCopy); err != nil {
			logrus.Errorf("update node label failed %v", err)
			return err
		}
	}
	return nil
}

func (r *VxlanReconciler) setVxlanNetDevRemote(ctx context.Context, vxlan *kvnetv1alpha1.Vxlan, vxlanName string) error {
	nodeName := os.Getenv("NODENAME")
	// remove all fdb first
	// ignore error
	// TODO find a way to add/update/delete fdb
	cmd := exec.Command("bridge", "fdb", "del", "00:00:00:00:00:00", "dev", vxlanName)
	cmd.Env = os.Environ()
	_ = cmd.Run()

	vxlans := &kvnetv1alpha1.VxlanList{}
	if err := r.List(ctx, vxlans); err != nil {
		logrus.Errorf("list all vxlans fail %v", err)
		return err
	}

	for _, vxlan := range vxlans.Items {
		tmpName := strings.Split(vxlan.Name, ".")
		if len(tmpName) < 2 {
			return fmt.Errorf("remote: error to parse vxlan name: %v", vxlan.Name)
		}
		node := tmpName[0]
		name := tmpName[1]
		remoteIP := vxlan.Status.LocalIP

		if node == nodeName || name != vxlanName || remoteIP == "" {
			// skip
			continue
		}

		logrus.Infof("vxlan %s add remote IP %s", vxlanName, remoteIP)
		cmd = exec.Command("bridge", "fdb", "append", "00:00:00:00:00:00", "dev", vxlanName, "dst", remoteIP)
		cmd.Env = os.Environ()
		// TODO
		_ = cmd.Run()
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VxlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.Vxlan{}).
		Watches(
			&source.Kind{Type: &kvnetv1alpha1.Bridge{}},
			handler.EnqueueRequestsFromMapFunc(r.VxlanBridgeWatchMap),
		).
		Complete(r)
}

func (r *VxlanReconciler) VxlanBridgeWatchMap(obj client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	vxlans := &kvnetv1alpha1.VxlanList{}
	if err := r.List(context.Background(), vxlans); err != nil {
		logrus.Errorf("cannot reconcile vxlan for bridge change")
		return requests
	}

	for _, vxlan := range vxlans.Items {
		requests = append(requests,
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: vxlan.Name,
				},
			},
		)
	}
	return requests
}
