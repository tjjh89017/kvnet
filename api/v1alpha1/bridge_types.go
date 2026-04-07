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

// BridgeSpec defines the desired state of Bridge.
type BridgeSpec struct {
	// VlanFiltering enables or disables VLAN filtering on the bridge.
	// +optional
	VlanFiltering bool `json:"vlanFiltering,omitempty"`

	// VniFilter enables per-VLAN VNI filtering on the bridge (for single VXLAN device mode).
	// +optional
	VniFilter bool `json:"vniFilter,omitempty"`

	// NfCallIptables enables or disables nf_call_iptables on the bridge.
	// +kubebuilder:default=false
	// +optional
	NfCallIptables bool `json:"nfCallIptables,omitempty"`
}

// BridgeStatus defines the observed state of Bridge.
type BridgeStatus struct {
	// Conditions represent the latest available observations of the Bridge's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// Bridge is the Schema for the bridges API.
type Bridge struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BridgeSpec   `json:"spec,omitempty"`
	Status BridgeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BridgeList contains a list of Bridge.
type BridgeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bridge `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Bridge{}, &BridgeList{})
}
