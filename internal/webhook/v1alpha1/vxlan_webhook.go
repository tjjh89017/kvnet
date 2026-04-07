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

package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var vxlanlog = logf.Log.WithName("vxlan-resource")

// SetupVXLANWebhookWithManager registers the webhook for VXLAN in the manager.
func SetupVXLANWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&kvnetv1alpha1.VXLAN{}).
		WithValidator(&VXLANCustomValidator{}).
		WithDefaulter(&VXLANCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-kvnet-kojuro-date-v1alpha1-vxlan,mutating=true,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=vxlans,verbs=create;update,versions=v1alpha1,name=mvxlan-v1alpha1.kb.io,admissionReviewVersions=v1

// VXLANCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind VXLAN when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type VXLANCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &VXLANCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind VXLAN.
func (d *VXLANCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	vxlan, ok := obj.(*kvnetv1alpha1.VXLAN)

	if !ok {
		return fmt.Errorf("expected an VXLAN object but got %T", obj)
	}
	vxlanlog.Info("Defaulting for VXLAN", "name", vxlan.GetName())

	// TODO(user): fill in your defaulting logic.

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-kvnet-kojuro-date-v1alpha1-vxlan,mutating=false,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=vxlans,verbs=create;update,versions=v1alpha1,name=vvxlan-v1alpha1.kb.io,admissionReviewVersions=v1

// VXLANCustomValidator struct is responsible for validating the VXLAN resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type VXLANCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &VXLANCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type VXLAN.
func (v *VXLANCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	vxlan, ok := obj.(*kvnetv1alpha1.VXLAN)
	if !ok {
		return nil, fmt.Errorf("expected a VXLAN object but got %T", obj)
	}
	vxlanlog.Info("Validation for VXLAN upon creation", "name", vxlan.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type VXLAN.
func (v *VXLANCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	vxlan, ok := newObj.(*kvnetv1alpha1.VXLAN)
	if !ok {
		return nil, fmt.Errorf("expected a VXLAN object for the newObj but got %T", newObj)
	}
	vxlanlog.Info("Validation for VXLAN upon update", "name", vxlan.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type VXLAN.
func (v *VXLANCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	vxlan, ok := obj.(*kvnetv1alpha1.VXLAN)
	if !ok {
		return nil, fmt.Errorf("expected a VXLAN object but got %T", obj)
	}
	vxlanlog.Info("Validation for VXLAN upon deletion", "name", vxlan.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
