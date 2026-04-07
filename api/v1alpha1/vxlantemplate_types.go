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

// VXLANTemplateInner is the template body that embeds the full VXLANSpec.
type VXLANTemplateInner struct {
	// Spec is the full VXLANSpec to apply to each matching node.
	Spec VXLANSpec `json:"spec"`
}

// VXLANTemplateSpec defines the desired state of VXLANTemplate.
type VXLANTemplateSpec struct {
	// VXLANName is the name of the VXLAN device to create on each node.
	VXLANName string `json:"vxlanName"`

	// VXLANID is the VXLAN Network Identifier (VNI).
	VXLANID int `json:"vxlanID"`

	// NodeSelector selects which nodes this VXLAN should be created on.
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`

	// Template contains the VXLANSpec to apply.
	Template VXLANTemplateInner `json:"template"`
}

// VXLANTemplateStatus defines the observed state of VXLANTemplate.
type VXLANTemplateStatus struct {
	// Conditions represent the latest available observations of the VXLANTemplate's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// VXLANTemplate is the Schema for the vxlantemplates API.
type VXLANTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VXLANTemplateSpec   `json:"spec,omitempty"`
	Status VXLANTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VXLANTemplateList contains a list of VXLANTemplate.
type VXLANTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VXLANTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VXLANTemplate{}, &VXLANTemplateList{})
}
