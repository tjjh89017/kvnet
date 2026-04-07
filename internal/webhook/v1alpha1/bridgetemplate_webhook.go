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
var bridgetemplatelog = logf.Log.WithName("bridgetemplate-resource")

// SetupBridgeTemplateWebhookWithManager registers the webhook for BridgeTemplate in the manager.
func SetupBridgeTemplateWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&kvnetv1alpha1.BridgeTemplate{}).
		WithValidator(&BridgeTemplateCustomValidator{}).
		WithDefaulter(&BridgeTemplateCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-kvnet-kojuro-date-v1alpha1-bridgetemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=bridgetemplates,verbs=create;update,versions=v1alpha1,name=mbridgetemplate-v1alpha1.kb.io,admissionReviewVersions=v1

type BridgeTemplateCustomDefaulter struct{}

var _ webhook.CustomDefaulter = &BridgeTemplateCustomDefaulter{}

func (d *BridgeTemplateCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	bridgetemplate, ok := obj.(*kvnetv1alpha1.BridgeTemplate)
	if !ok {
		return fmt.Errorf("expected a BridgeTemplate object but got %T", obj)
	}
	bridgetemplatelog.Info("Defaulting for BridgeTemplate", "name", bridgetemplate.GetName())
	return nil
}

// +kubebuilder:webhook:path=/validate-kvnet-kojuro-date-v1alpha1-bridgetemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=kvnet.kojuro.date,resources=bridgetemplates,verbs=create;update,versions=v1alpha1,name=vbridgetemplate-v1alpha1.kb.io,admissionReviewVersions=v1

type BridgeTemplateCustomValidator struct{}

var _ webhook.CustomValidator = &BridgeTemplateCustomValidator{}

func (v *BridgeTemplateCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	tmpl, ok := obj.(*kvnetv1alpha1.BridgeTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a BridgeTemplate object but got %T", obj)
	}
	return nil, v.validate(tmpl)
}

func (v *BridgeTemplateCustomValidator) ValidateUpdate(_ context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	tmpl, ok := newObj.(*kvnetv1alpha1.BridgeTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a BridgeTemplate object but got %T", newObj)
	}
	return nil, v.validate(tmpl)
}

func (v *BridgeTemplateCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *BridgeTemplateCustomValidator) validate(tmpl *kvnetv1alpha1.BridgeTemplate) error {
	if tmpl.Spec.BridgeName == "" {
		return fmt.Errorf("spec.bridgeName is required")
	}
	return nil
}
