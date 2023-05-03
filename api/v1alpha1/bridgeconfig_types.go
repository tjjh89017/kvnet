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
	BridgeConfigFinalizer      = "kvnet.kojuro.date/finalizer"
	BridgeConfigNamespaceLabel = "bridgeconfig.kvnet.kojuro.date/namespace"
	BridgeConfigNameLabel      = "bridgeconfig.kvnet.kojuro.date/name"
)

// BridgeConfigSpec defines the desired state of BridgeConfig
type BridgeConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	NodeSelector  *metav1.LabelSelector `json:"selector,omitempty"`
	BridgeName    string                `json:"bridge,omitempty"`
	VlanFiltering bool                  `json:"vlanFiltering,omitempty"`
}

// BridgeConfigStatus defines the observed state of BridgeConfig
type BridgeConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BridgeConfig is the Schema for the bridgeconfigs API
type BridgeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BridgeConfigSpec   `json:"spec,omitempty"`
	Status BridgeConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BridgeConfigList contains a list of BridgeConfig
type BridgeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BridgeConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BridgeConfig{}, &BridgeConfigList{})
}
