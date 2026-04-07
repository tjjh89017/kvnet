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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UplinkTemplateInner is the template body that embeds the full UplinkSpec.
type UplinkTemplateInner struct {
	// Spec is the full UplinkSpec to apply to each matching node.
	Spec UplinkSpec `json:"spec"`
}

// UplinkTemplateSpec defines the desired state of UplinkTemplate.
type UplinkTemplateSpec struct {
	// BondName is the name of the bond interface to create on each node.
	BondName string `json:"bondName"`

	// NodeSelector selects which nodes this uplink should be created on.
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`

	// Template contains the UplinkSpec to apply.
	Template UplinkTemplateInner `json:"template"`
}

// UplinkTemplateStatus defines the observed state of UplinkTemplate.
type UplinkTemplateStatus struct {
	// Conditions represent the latest available observations of the UplinkTemplate's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// UplinkTemplate is the Schema for the uplinktemplates API.
type UplinkTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UplinkTemplateSpec   `json:"spec,omitempty"`
	Status UplinkTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UplinkTemplateList contains a list of UplinkTemplate.
type UplinkTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UplinkTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UplinkTemplate{}, &UplinkTemplateList{})
}
