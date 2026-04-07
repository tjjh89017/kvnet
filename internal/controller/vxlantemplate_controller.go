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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// VXLANTemplateReconciler reconciles a VXLANTemplate object
type VXLANTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlantemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlantemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlantemplates/finalizers,verbs=update
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=vxlans,verbs=get;list;watch;create;update;patch;delete

func (r *VXLANTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var tmpl kvnetv1alpha1.VXLANTemplate
	if err := r.Get(ctx, req.NamespacedName, &tmpl); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !tmpl.DeletionTimestamp.IsZero() {
		return r.onDelete(ctx, req, &tmpl)
	}

	if !controllerutil.ContainsFinalizer(&tmpl, kvnetv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(&tmpl, kvnetv1alpha1.FinalizerName)
		if err := r.Update(ctx, &tmpl); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return r.onChange(ctx, req, &tmpl)
}

func (r *VXLANTemplateReconciler) onDelete(ctx context.Context, _ ctrl.Request, tmpl *kvnetv1alpha1.VXLANTemplate) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var vxlans kvnetv1alpha1.VXLANList
	if err := r.List(ctx, &vxlans, client.MatchingLabels{
		kvnetv1alpha1.LabelTemplateName: tmpl.Name,
		kvnetv1alpha1.LabelTemplateNS:   tmpl.Namespace,
	}); err != nil {
		return ctrl.Result{}, err
	}
	for i := range vxlans.Items {
		if err := r.Delete(ctx, &vxlans.Items[i]); err != nil {
			log.Error(err, "failed to delete VXLAN", "name", vxlans.Items[i].Name)
		}
	}

	controllerutil.RemoveFinalizer(tmpl, kvnetv1alpha1.FinalizerName)
	return ctrl.Result{}, r.Update(ctx, tmpl)
}

func (r *VXLANTemplateReconciler) onChange(ctx context.Context, _ ctrl.Request, tmpl *kvnetv1alpha1.VXLANTemplate) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	selector, err := metav1.LabelSelectorAsSelector(tmpl.Spec.NodeSelector)
	if err != nil {
		r.setReadyCondition(ctx, tmpl, metav1.ConditionFalse, "InvalidSelector", err.Error())
		return ctrl.Result{}, err
	}

	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes, &client.ListOptions{
		LabelSelector: selector,
	}); err != nil {
		return ctrl.Result{}, err
	}

	desired := make(map[string]bool, len(nodes.Items))
	for _, node := range nodes.Items {
		vxlanCRName := fmt.Sprintf("%s.%s", node.Name, tmpl.Spec.VXLANName)
		desired[vxlanCRName] = true

		vxlan := &kvnetv1alpha1.VXLAN{
			ObjectMeta: metav1.ObjectMeta{
				Name: vxlanCRName,
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, vxlan, func() error {
			if vxlan.Labels == nil {
				vxlan.Labels = map[string]string{}
			}
			vxlan.Labels[kvnetv1alpha1.LabelTemplateName] = tmpl.Name
			vxlan.Labels[kvnetv1alpha1.LabelTemplateNS] = tmpl.Namespace
			vxlan.Labels[kvnetv1alpha1.LabelNode] = node.Name
			vxlan.Spec = tmpl.Spec.Template.Spec
			return nil
		})
		if err != nil {
			r.setReadyCondition(ctx, tmpl, metav1.ConditionFalse, "ReconcileError", err.Error())
			return ctrl.Result{}, err
		}
		log.Info("synced VXLAN", "name", vxlanCRName, "node", node.Name)
	}

	// Orphan cleanup
	var existing kvnetv1alpha1.VXLANList
	if err := r.List(ctx, &existing, client.MatchingLabels{
		kvnetv1alpha1.LabelTemplateName: tmpl.Name,
		kvnetv1alpha1.LabelTemplateNS:   tmpl.Namespace,
	}); err != nil {
		return ctrl.Result{}, err
	}
	for i := range existing.Items {
		if !desired[existing.Items[i].Name] {
			if err := r.Delete(ctx, &existing.Items[i]); err != nil {
				log.Error(err, "failed to delete orphan VXLAN", "name", existing.Items[i].Name)
			} else {
				log.Info("deleted orphan VXLAN", "name", existing.Items[i].Name)
			}
		}
	}

	r.setReadyCondition(ctx, tmpl, metav1.ConditionTrue, "ReconcileSucceeded", "All VXLANs reconciled successfully.")
	return ctrl.Result{}, nil
}

func (r *VXLANTemplateReconciler) setReadyCondition(ctx context.Context, tmpl *kvnetv1alpha1.VXLANTemplate, status metav1.ConditionStatus, reason, message string) {
	patch := client.MergeFrom(tmpl.DeepCopy())
	meta.SetStatusCondition(&tmpl.Status.Conditions, metav1.Condition{
		Type:               kvnetv1alpha1.ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: tmpl.Generation,
	})
	if err := r.Status().Patch(ctx, tmpl, patch); err != nil {
		logf.FromContext(ctx).Error(err, "failed to patch VXLANTemplate status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *VXLANTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.VXLANTemplate{}).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.templatesForNode)).
		Named("vxlantemplate").
		Complete(r)
}

func (r *VXLANTemplateReconciler) templatesForNode(ctx context.Context, _ client.Object) []reconcile.Request {
	var list kvnetv1alpha1.VXLANTemplateList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&list.Items[i]),
		})
	}
	return reqs
}
