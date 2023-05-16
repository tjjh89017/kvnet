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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
const (
	VxlanConfigFinalizer      = "vxlanconfig.kvnet.kojuro.date/finalizer"
	VxlanConfigNamespaceLabel = "vxlanconfig.kvnet.kojuro.date/namespace"
	VxlanConfigNameLabel      = "vxlanconfig.kvnet.kojuro.date/name"
	VxlanConfigNodeLabel      = "vxlanconfig.kvnet.kojuro.date/node"
)

// VxlanConfigSpec defines the desired state of VxlanConfig
type VxlanConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	NodeSelector *metav1.LabelSelector `json:"selector,omitempty"`
	VxlanID      int                   `json:"vxlanID,omitempty"`
	Template     VxlanTemplateSpec     `json:"template,omitempty"`
}

// VxlanTemplateSpec
type VxlanTemplateSpec struct {
	metav1.ObjectMeta `json:"metadata.omitempty"`
	Spec              VxlanSpec `json:"spec,omitempty"`
}

// VxlanConfigStatus defines the observed state of VxlanConfig
type VxlanConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VxlanConfig is the Schema for the vxlanconfigs API
type VxlanConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VxlanConfigSpec   `json:"spec,omitempty"`
	Status VxlanConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VxlanConfigList contains a list of VxlanConfig
type VxlanConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VxlanConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VxlanConfig{}, &VxlanConfigList{})
}
