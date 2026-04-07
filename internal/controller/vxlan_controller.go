/*
Copyright 2026.

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

package controller

import (
	"context"
	"fmt"
	"net"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// VXLANReconciler reconciles a VXLAN object (agent mode)
type VXLANReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	NodeName string
}

// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlans/finalizers,verbs=update
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlantemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

func (r *VXLANReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var vxlan kvnetv1alpha1.VXLAN
	if err := r.Get(ctx, req.NamespacedName, &vxlan); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	nodeLabel := vxlan.Labels[kvnetv1alpha1.LabelNode]

	// This node owns this VXLAN
	if nodeLabel == r.NodeName {
		if !vxlan.DeletionTimestamp.IsZero() {
			return r.onDelete(ctx, req, &vxlan)
		}

		if !controllerutil.ContainsFinalizer(&vxlan, kvnetv1alpha1.FinalizerName) {
			controllerutil.AddFinalizer(&vxlan, kvnetv1alpha1.FinalizerName)
			if err := r.Update(ctx, &vxlan); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

		return r.onChange(ctx, req, &vxlan)
	}

	// Remote VXLAN: update FDB entries for this peer
	return r.onRemoteChange(ctx, req, &vxlan)
}

func (r *VXLANReconciler) onChange(ctx context.Context, _ ctrl.Request, vxlan *kvnetv1alpha1.VXLAN) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	vxlanDevName, err := parseDeviceName(vxlan.Name, r.NodeName)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Check if VXLAN exists, create if not
	if err := execCmd("ip", "link", "show", "dev", vxlanDevName); err != nil {
		log.Info("creating VXLAN", "name", vxlanDevName, "external", vxlan.Spec.External)

		if vxlan.Spec.External {
			// Single VXLAN device mode
			args := []string{"link", "add", vxlanDevName, "type", "vxlan", "external", "dstport", "4789"}
			if vxlan.Spec.VNIFilter {
				args = append(args, "vnifilter")
			}
			if err := execCmd("ip", args...); err != nil {
				r.setReadyCondition(ctx, vxlan, metav1.ConditionFalse, "CreateFailed", err.Error())
				return ctrl.Result{}, err
			}
		} else {
			// Classic per-VNI mode: use Spec.VNI if set, otherwise fall back to template VXLANID
			vxlanID := vxlan.Spec.VNI
			if vxlanID == 0 {
				vxlanID, err = r.resolveVXLANID(ctx, vxlan)
				if err != nil {
					r.setReadyCondition(ctx, vxlan, metav1.ConditionFalse, "ResolveIDFailed", err.Error())
					return ctrl.Result{}, err
				}
			}
			if err := execCmd("ip", "link", "add", vxlanDevName, "type", "vxlan", "id", strconv.Itoa(vxlanID), "dstport", "4789"); err != nil {
				r.setReadyCondition(ctx, vxlan, metav1.ConditionFalse, "CreateFailed", err.Error())
				return ctrl.Result{}, err
			}
		}
	}

	// Resolve local IP
	localIP := vxlan.Spec.LocalIP
	if localIP == "" && vxlan.Spec.Dev != "" {
		localIP, err = getInterfaceIP(vxlan.Spec.Dev)
		if err != nil {
			r.setReadyCondition(ctx, vxlan, metav1.ConditionFalse, "LocalIPFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	// Set local IP
	if localIP != "" {
		if err := execCmd("ip", "link", "set", vxlanDevName, "type", "vxlan", "local", localIP); err != nil {
			r.setReadyCondition(ctx, vxlan, metav1.ConditionFalse, "ConfigFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	// Set MTU only when non-zero
	if vxlan.Spec.MTU > 0 {
		if err := execCmd("ip", "link", "set", vxlanDevName, "mtu", strconv.Itoa(vxlan.Spec.MTU)); err != nil {
			r.setReadyCondition(ctx, vxlan, metav1.ConditionFalse, "MTUFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	// Set master bridge if specified
	if vxlan.Spec.Master != "" {
		if err := execCmd("ip", "link", "set", vxlanDevName, "master", vxlan.Spec.Master); err != nil {
			r.setReadyCondition(ctx, vxlan, metav1.ConditionFalse, "MasterFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	// Set VXLAN VTEP learning (kernel vxlan driver)
	if vxlan.Spec.Learning {
		_ = execCmd("ip", "link", "set", vxlanDevName, "type", "vxlan", "learning")
	} else {
		_ = execCmd("ip", "link", "set", vxlanDevName, "type", "vxlan", "nolearning")
	}

	// Bring up
	if err := execCmd("ip", "link", "set", vxlanDevName, "up"); err != nil {
		r.setReadyCondition(ctx, vxlan, metav1.ConditionFalse, "UpFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Add VNIs for vnifilter mode (applied on the VXLAN device, not bridge-slave-gated)
	if vxlan.Spec.VNIFilter && vxlan.Spec.PortVLANConfig != nil {
		for _, mapping := range vxlan.Spec.PortVLANConfig.TunnelInfo {
			vidEnd := mapping.Vid
			if mapping.VidEnd != nil {
				vidEnd = *mapping.VidEnd
			}
			for i := 0; i <= vidEnd-mapping.Vid; i++ {
				vni := mapping.Vni + i
				if err := execCmd("bridge", "vni", "add", "dev", vxlanDevName, "vni", strconv.Itoa(vni)); err != nil {
					log.Error(err, "failed to add vni", "dev", vxlanDevName, "vni", vni)
				}
			}
		}
	}

	// Bridge slave settings: only apply when device is actually a bridge slave
	if isBridgeSlave(vxlanDevName) {
		// Set bridge slave MAC learning
		bridgeLearning := "off"
		if vxlan.Spec.BridgeLearning {
			bridgeLearning = "on"
		}
		if err := execCmd("bridge", "link", "set", "dev", vxlanDevName, "learning", bridgeLearning); err != nil {
			log.Error(err, "failed to set bridge learning", "dev", vxlanDevName)
		}

		// Configure bridge port VLAN settings
		if vxlan.Spec.PortVLANConfig != nil {
			if err := applyPortVLANConfig(vxlanDevName, vxlan.Spec.PortVLANConfig); err != nil {
				r.setReadyCondition(ctx, vxlan, metav1.ConditionFalse, "VLANConfigFailed", err.Error())
				return ctrl.Result{}, err
			}
		}
	}

	// Label the node
	if err := r.labelNode(ctx, vxlanDevName, vxlan.Spec.Master); err != nil {
		log.Error(err, "failed to label node")
	}

	// Update status with local IP
	if localIP != "" && vxlan.Status.LocalIP != localIP {
		patch := client.MergeFrom(vxlan.DeepCopy())
		vxlan.Status.LocalIP = localIP
		if err := r.Status().Patch(ctx, vxlan, patch); err != nil {
			log.Error(err, "failed to update VXLAN status localIP")
		}
	}

	r.setReadyCondition(ctx, vxlan, metav1.ConditionTrue, "Configured", "VXLAN configured successfully.")
	log.Info("VXLAN configured", "name", vxlanDevName, "localIP", localIP)
	return ctrl.Result{}, nil
}

func (r *VXLANReconciler) onRemoteChange(ctx context.Context, _ ctrl.Request, vxlan *kvnetv1alpha1.VXLAN) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if vxlan.Status.LocalIP == "" {
		return ctrl.Result{}, nil
	}

	// Find all local VXLANs with the same template
	tmplName := vxlan.Labels[kvnetv1alpha1.LabelTemplateName]
	tmplNS := vxlan.Labels[kvnetv1alpha1.LabelTemplateNS]
	if tmplName == "" {
		return ctrl.Result{}, nil
	}

	var localVXLANs kvnetv1alpha1.VXLANList
	if err := r.List(ctx, &localVXLANs, client.MatchingLabels{
		kvnetv1alpha1.LabelTemplateName: tmplName,
		kvnetv1alpha1.LabelTemplateNS:   tmplNS,
		kvnetv1alpha1.LabelNode:         r.NodeName,
	}); err != nil {
		return ctrl.Result{}, err
	}

	for i := range localVXLANs.Items {
		localDev, err := parseDeviceName(localVXLANs.Items[i].Name, r.NodeName)
		if err != nil {
			continue
		}

		// Check if local device exists
		if err := execCmd("ip", "link", "show", "dev", localDev); err != nil {
			continue
		}

		// Update FDB: remove old entry, then append remote IP
		_ = execCmd("bridge", "fdb", "del", "00:00:00:00:00:00", "dev", localDev, "dst", vxlan.Status.LocalIP)
		if err := execCmd("bridge", "fdb", "append", "00:00:00:00:00:00", "dev", localDev, "dst", vxlan.Status.LocalIP); err != nil {
			log.Error(err, "failed to add FDB entry", "dev", localDev, "dst", vxlan.Status.LocalIP)
		} else {
			log.Info("updated FDB", "dev", localDev, "dst", vxlan.Status.LocalIP)
		}
	}

	return ctrl.Result{}, nil
}

func (r *VXLANReconciler) onDelete(ctx context.Context, _ ctrl.Request, vxlan *kvnetv1alpha1.VXLAN) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	vxlanDevName, err := parseDeviceName(vxlan.Name, r.NodeName)
	if err == nil {
		if delErr := execCmd("ip", "link", "del", vxlanDevName); delErr != nil {
			log.Info("VXLAN already removed or failed to delete", "name", vxlanDevName, "error", delErr)
		}
		if err := r.labelNode(ctx, vxlanDevName, ""); err != nil {
			log.Error(err, "failed to remove node label")
		}
	}

	controllerutil.RemoveFinalizer(vxlan, kvnetv1alpha1.FinalizerName)
	return ctrl.Result{}, r.Update(ctx, vxlan)
}

func (r *VXLANReconciler) resolveVXLANID(ctx context.Context, vxlan *kvnetv1alpha1.VXLAN) (int, error) {
	tmplName := vxlan.Labels[kvnetv1alpha1.LabelTemplateName]
	tmplNS := vxlan.Labels[kvnetv1alpha1.LabelTemplateNS]
	if tmplName == "" || tmplNS == "" {
		return 0, fmt.Errorf("VXLAN %q missing template labels", vxlan.Name)
	}

	var tmpl kvnetv1alpha1.VXLANTemplate
	if err := r.Get(ctx, types.NamespacedName{Name: tmplName, Namespace: tmplNS}, &tmpl); err != nil {
		return 0, fmt.Errorf("get VXLANTemplate %s/%s: %w", tmplNS, tmplName, err)
	}
	return tmpl.Spec.VXLANID, nil
}

func (r *VXLANReconciler) labelNode(ctx context.Context, vxlanDevName, value string) error {
	var node corev1.Node
	if err := r.Get(ctx, types.NamespacedName{Name: r.NodeName}, &node); err != nil {
		return err
	}

	patch := client.MergeFrom(node.DeepCopy())
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}
	labelKey := fmt.Sprintf("vxlan.kvnet.kojuro.date/%s", vxlanDevName)
	if value == "" {
		delete(node.Labels, labelKey)
	} else {
		node.Labels[labelKey] = value
	}
	return r.Patch(ctx, &node, patch)
}

func (r *VXLANReconciler) setReadyCondition(ctx context.Context, vxlan *kvnetv1alpha1.VXLAN, status metav1.ConditionStatus, reason, message string) {
	patch := client.MergeFrom(vxlan.DeepCopy())
	meta.SetStatusCondition(&vxlan.Status.Conditions, metav1.Condition{
		Type:               kvnetv1alpha1.ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: vxlan.Generation,
	})
	if err := r.Status().Patch(ctx, vxlan, patch); err != nil {
		logf.FromContext(ctx).Error(err, "failed to patch VXLAN status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *VXLANReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.VXLAN{}).
		Named("vxlan").
		Complete(r)
}

// applyPortVLANConfig configures bridge port VLAN settings for a device.
func applyPortVLANConfig(devName string, cfg *kvnetv1alpha1.PortVLANConfig) error {
	// Set pvid (native VLAN)
	if cfg.Pvid != nil {
		if err := execCmd("bridge", "vlan", "add", "dev", devName, "vid", strconv.Itoa(*cfg.Pvid), "pvid", "untagged"); err != nil {
			return fmt.Errorf("set pvid %d on %s: %w", *cfg.Pvid, devName, err)
		}
	}

	// Set trunk VLANs
	for _, vid := range cfg.Vids {
		if err := execCmd("bridge", "vlan", "add", "dev", devName, "vid", strconv.Itoa(vid)); err != nil {
			return fmt.Errorf("add vid %d on %s: %w", vid, devName, err)
		}
	}

	// Set VLAN-to-VNI tunnel mappings (for single VXLAN device mode)
	for _, mapping := range cfg.TunnelInfo {
		vidEnd := mapping.Vid
		if mapping.VidEnd != nil {
			vidEnd = *mapping.VidEnd
		}
		for i := 0; i <= vidEnd-mapping.Vid; i++ {
			vid := mapping.Vid + i
			vni := mapping.Vni + i
			if err := execCmd("bridge", "vlan", "add", "dev", devName, "vid", strconv.Itoa(vid), "tunnel_info", "id", strconv.Itoa(vni)); err != nil {
				return fmt.Errorf("add tunnel_info vid=%d vni=%d on %s: %w", vid, vni, devName, err)
			}
		}
	}

	return nil
}

func getInterfaceIP(devName string) (string, error) {
	iface, err := net.InterfaceByName(devName)
	if err != nil {
		return "", fmt.Errorf("interface %q not found: %w", devName, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("get addrs for %q: %w", devName, err)
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			return ipnet.IP.String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address found on %q", devName)
}
