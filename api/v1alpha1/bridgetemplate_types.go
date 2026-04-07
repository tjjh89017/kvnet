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

// BridgeTemplateInner is the template body that embeds the full BridgeSpec.
type BridgeTemplateInner struct {
	// Spec is the full BridgeSpec to apply to each matching node.
	Spec BridgeSpec `json:"spec"`
}

// BridgeTemplateSpec defines the desired state of BridgeTemplate.
type BridgeTemplateSpec struct {
	// BridgeName is the name of the bridge device to create on each node.
	BridgeName string `json:"bridgeName"`

	// NodeSelector selects which nodes this bridge should be created on.
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`

	// Template contains the BridgeSpec to apply.
	Template BridgeTemplateInner `json:"template"`
}

// BridgeTemplateStatus defines the observed state of BridgeTemplate.
type BridgeTemplateStatus struct {
	// Conditions represent the latest available observations of the BridgeTemplate's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// BridgeTemplate is the Schema for the bridgetemplates API.
type BridgeTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BridgeTemplateSpec   `json:"spec,omitempty"`
	Status BridgeTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BridgeTemplateList contains a list of BridgeTemplate.
type BridgeTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BridgeTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BridgeTemplate{}, &BridgeTemplateList{})
}
