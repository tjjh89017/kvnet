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
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// nolint:unused
var uplinktemplatelog = logf.Log.WithName("uplinktemplate-resource")

// SetupUplinkTemplateWebhookWithManager registers the webhook for UplinkTemplate in the manager.
func SetupUplinkTemplateWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&kvnetv1alpha1.UplinkTemplate{}).
		WithValidator(&UplinkTemplateCustomValidator{Client: mgr.GetClient()}).
		WithDefaulter(&UplinkTemplateCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-kvnet-kojuro-date-v1alpha1-uplinktemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=uplinktemplates,verbs=create;update,versions=v1alpha1,name=muplinktemplate-v1alpha1.kb.io,admissionReviewVersions=v1

type UplinkTemplateCustomDefaulter struct{}

var _ webhook.CustomDefaulter = &UplinkTemplateCustomDefaulter{}

func (d *UplinkTemplateCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	uplinktemplate, ok := obj.(*kvnetv1alpha1.UplinkTemplate)
	if !ok {
		return fmt.Errorf("expected an UplinkTemplate object but got %T", obj)
	}
	uplinktemplatelog.Info("Defaulting for UplinkTemplate", "name", uplinktemplate.GetName())
	return nil
}

// +kubebuilder:webhook:path=/validate-kvnet-kojuro-date-v1alpha1-uplinktemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=uplinktemplates,verbs=create;update,versions=v1alpha1,name=vuplinktemplate-v1alpha1.kb.io,admissionReviewVersions=v1

type UplinkTemplateCustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &UplinkTemplateCustomValidator{}

func (v *UplinkTemplateCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	uplinktemplate, ok := obj.(*kvnetv1alpha1.UplinkTemplate)
	if !ok {
		return nil, fmt.Errorf("expected an UplinkTemplate object but got %T", obj)
	}
	return nil, v.validate(ctx, uplinktemplate)
}

func (v *UplinkTemplateCustomValidator) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	uplinktemplate, ok := newObj.(*kvnetv1alpha1.UplinkTemplate)
	if !ok {
		return nil, fmt.Errorf("expected an UplinkTemplate object but got %T", newObj)
	}
	return nil, v.validate(ctx, uplinktemplate)
}

func (v *UplinkTemplateCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *UplinkTemplateCustomValidator) validate(ctx context.Context, tmpl *kvnetv1alpha1.UplinkTemplate) error {
	if tmpl.Spec.BondName == "" {
		return fmt.Errorf("spec.bondName is required")
	}
	// Validate master references a bridge
	if err := validateMasterIsBridge(ctx, v.Client, tmpl.Spec.Template.Spec.Master); err != nil {
		return err
	}
	return nil
}
