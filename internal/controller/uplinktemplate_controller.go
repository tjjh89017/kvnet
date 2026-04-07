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

// UplinkTemplateReconciler reconciles a UplinkTemplate object
type UplinkTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinktemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinktemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinktemplates/finalizers,verbs=update
// +kubebuilder:rbac:groups=kvnet.kojuro.date,resources=uplinks,verbs=get;list;watch;create;update;patch;delete

func (r *UplinkTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var tmpl kvnetv1alpha1.UplinkTemplate
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

func (r *UplinkTemplateReconciler) onDelete(ctx context.Context, _ ctrl.Request, tmpl *kvnetv1alpha1.UplinkTemplate) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var uplinks kvnetv1alpha1.UplinkList
	if err := r.List(ctx, &uplinks, client.MatchingLabels{
		kvnetv1alpha1.LabelTemplateName: tmpl.Name,
		kvnetv1alpha1.LabelTemplateNS:   tmpl.Namespace,
	}); err != nil {
		return ctrl.Result{}, err
	}
	for i := range uplinks.Items {
		if err := r.Delete(ctx, &uplinks.Items[i]); err != nil {
			log.Error(err, "failed to delete Uplink", "name", uplinks.Items[i].Name)
		}
	}

	controllerutil.RemoveFinalizer(tmpl, kvnetv1alpha1.FinalizerName)
	return ctrl.Result{}, r.Update(ctx, tmpl)
}

func (r *UplinkTemplateReconciler) onChange(ctx context.Context, _ ctrl.Request, tmpl *kvnetv1alpha1.UplinkTemplate) (ctrl.Result, error) {
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
		uplinkCRName := fmt.Sprintf("%s.%s", node.Name, tmpl.Spec.BondName)
		desired[uplinkCRName] = true

		uplink := &kvnetv1alpha1.Uplink{
			ObjectMeta: metav1.ObjectMeta{
				Name: uplinkCRName,
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, uplink, func() error {
			if uplink.Labels == nil {
				uplink.Labels = map[string]string{}
			}
			uplink.Labels[kvnetv1alpha1.LabelTemplateName] = tmpl.Name
			uplink.Labels[kvnetv1alpha1.LabelTemplateNS] = tmpl.Namespace
			uplink.Labels[kvnetv1alpha1.LabelNode] = node.Name
			uplink.Spec = tmpl.Spec.Template.Spec
			return nil
		})
		if err != nil {
			r.setReadyCondition(ctx, tmpl, metav1.ConditionFalse, "ReconcileError", err.Error())
			return ctrl.Result{}, err
		}
		log.Info("synced Uplink", "name", uplinkCRName, "node", node.Name)
	}

	// Orphan cleanup
	var existing kvnetv1alpha1.UplinkList
	if err := r.List(ctx, &existing, client.MatchingLabels{
		kvnetv1alpha1.LabelTemplateName: tmpl.Name,
		kvnetv1alpha1.LabelTemplateNS:   tmpl.Namespace,
	}); err != nil {
		return ctrl.Result{}, err
	}
	for i := range existing.Items {
		if !desired[existing.Items[i].Name] {
			if err := r.Delete(ctx, &existing.Items[i]); err != nil {
				log.Error(err, "failed to delete orphan Uplink", "name", existing.Items[i].Name)
			} else {
				log.Info("deleted orphan Uplink", "name", existing.Items[i].Name)
			}
		}
	}

	r.setReadyCondition(ctx, tmpl, metav1.ConditionTrue, "ReconcileSucceeded", "All Uplinks reconciled successfully.")
	return ctrl.Result{}, nil
}

func (r *UplinkTemplateReconciler) setReadyCondition(ctx context.Context, tmpl *kvnetv1alpha1.UplinkTemplate, status metav1.ConditionStatus, reason, message string) {
	patch := client.MergeFrom(tmpl.DeepCopy())
	meta.SetStatusCondition(&tmpl.Status.Conditions, metav1.Condition{
		Type:               kvnetv1alpha1.ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: tmpl.Generation,
	})
	if err := r.Status().Patch(ctx, tmpl, patch); err != nil {
		logf.FromContext(ctx).Error(err, "failed to patch UplinkTemplate status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *UplinkTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.UplinkTemplate{}).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.templatesForNode)).
		Named("uplinktemplate").
		Complete(r)
}

func (r *UplinkTemplateReconciler) templatesForNode(ctx context.Context, _ client.Object) []reconcile.Request {
	var list kvnetv1alpha1.UplinkTemplateList
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
