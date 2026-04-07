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
var vxlantemplatelog = logf.Log.WithName("vxlantemplate-resource")

// SetupVXLANTemplateWebhookWithManager registers the webhook for VXLANTemplate in the manager.
func SetupVXLANTemplateWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&kvnetv1alpha1.VXLANTemplate{}).
		WithValidator(&VXLANTemplateCustomValidator{Client: mgr.GetClient()}).
		WithDefaulter(&VXLANTemplateCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-kvnet-kojuro-date-v1alpha1-vxlantemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=vxlantemplates,verbs=create;update,versions=v1alpha1,name=mvxlantemplate-v1alpha1.kb.io,admissionReviewVersions=v1

type VXLANTemplateCustomDefaulter struct{}

var _ webhook.CustomDefaulter = &VXLANTemplateCustomDefaulter{}

func (d *VXLANTemplateCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	vxlantemplate, ok := obj.(*kvnetv1alpha1.VXLANTemplate)
	if !ok {
		return fmt.Errorf("expected a VXLANTemplate object but got %T", obj)
	}
	vxlantemplatelog.Info("Defaulting for VXLANTemplate", "name", vxlantemplate.GetName())
	return nil
}

// +kubebuilder:webhook:path=/validate-kvnet-kojuro-date-v1alpha1-vxlantemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=vxlantemplates,verbs=create;update,versions=v1alpha1,name=vvxlantemplate-v1alpha1.kb.io,admissionReviewVersions=v1

type VXLANTemplateCustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &VXLANTemplateCustomValidator{}

func (v *VXLANTemplateCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	vxlantemplate, ok := obj.(*kvnetv1alpha1.VXLANTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a VXLANTemplate object but got %T", obj)
	}
	return nil, v.validate(ctx, vxlantemplate)
}

func (v *VXLANTemplateCustomValidator) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	vxlantemplate, ok := newObj.(*kvnetv1alpha1.VXLANTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a VXLANTemplate object but got %T", newObj)
	}
	return nil, v.validate(ctx, vxlantemplate)
}

func (v *VXLANTemplateCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *VXLANTemplateCustomValidator) validate(ctx context.Context, tmpl *kvnetv1alpha1.VXLANTemplate) error {
	if tmpl.Spec.VXLANName == "" {
		return fmt.Errorf("spec.vxlanName is required")
	}
	if !tmpl.Spec.Template.Spec.External && tmpl.Spec.VXLANID <= 0 {
		return fmt.Errorf("spec.vxlanID must be positive when external mode is not enabled")
	}
	// Validate master references a bridge
	if err := validateMasterIsBridge(ctx, v.Client, tmpl.Spec.Template.Spec.Master); err != nil {
		return err
	}
	return nil
}
