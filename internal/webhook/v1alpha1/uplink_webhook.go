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
var uplinklog = logf.Log.WithName("uplink-resource")

// SetupUplinkWebhookWithManager registers the webhook for Uplink in the manager.
func SetupUplinkWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&kvnetv1alpha1.Uplink{}).
		WithValidator(&UplinkCustomValidator{}).
		WithDefaulter(&UplinkCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-kvnet-kojuro-date-v1alpha1-uplink,mutating=true,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=uplinks,verbs=create;update,versions=v1alpha1,name=muplink-v1alpha1.kb.io,admissionReviewVersions=v1

// UplinkCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Uplink when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type UplinkCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &UplinkCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Uplink.
func (d *UplinkCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	uplink, ok := obj.(*kvnetv1alpha1.Uplink)

	if !ok {
		return fmt.Errorf("expected an Uplink object but got %T", obj)
	}
	uplinklog.Info("Defaulting for Uplink", "name", uplink.GetName())

	// TODO(user): fill in your defaulting logic.

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-kvnet-kojuro-date-v1alpha1-uplink,mutating=false,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=uplinks,verbs=create;update,versions=v1alpha1,name=vuplink-v1alpha1.kb.io,admissionReviewVersions=v1

// UplinkCustomValidator struct is responsible for validating the Uplink resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type UplinkCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &UplinkCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Uplink.
func (v *UplinkCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	uplink, ok := obj.(*kvnetv1alpha1.Uplink)
	if !ok {
		return nil, fmt.Errorf("expected a Uplink object but got %T", obj)
	}
	uplinklog.Info("Validation for Uplink upon creation", "name", uplink.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Uplink.
func (v *UplinkCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	uplink, ok := newObj.(*kvnetv1alpha1.Uplink)
	if !ok {
		return nil, fmt.Errorf("expected a Uplink object for the newObj but got %T", newObj)
	}
	uplinklog.Info("Validation for Uplink upon update", "name", uplink.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Uplink.
func (v *UplinkCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	uplink, ok := obj.(*kvnetv1alpha1.Uplink)
	if !ok {
		return nil, fmt.Errorf("expected a Uplink object but got %T", obj)
	}
	uplinklog.Info("Validation for Uplink upon deletion", "name", uplink.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
