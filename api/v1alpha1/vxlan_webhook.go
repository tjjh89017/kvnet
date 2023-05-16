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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var vxlanlog = logf.Log.WithName("vxlan-resource")

func (r *Vxlan) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-kvnet-kojuro-date-v1alpha1-vxlan,mutating=true,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=vxlans,verbs=create;update,versions=v1alpha1,name=mvxlan.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Vxlan{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Vxlan) Default() {
	vxlanlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
	if r.GetDeletionTimestamp() == nil {
		controllerutil.AddFinalizer(r, VxlanFinalizer)
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-kvnet-kojuro-date-v1alpha1-vxlan,mutating=false,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=vxlans,verbs=create;update,versions=v1alpha1,name=vvxlan.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Vxlan{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Vxlan) ValidateCreate() error {
	vxlanlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Vxlan) ValidateUpdate(old runtime.Object) error {
	vxlanlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Vxlan) ValidateDelete() error {
	vxlanlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
