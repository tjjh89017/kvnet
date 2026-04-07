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

// UplinkSpec defines the desired state of Uplink.
type UplinkSpec struct {
	// Master is the bridge device this uplink is attached to.
	// +optional
	Master string `json:"master,omitempty"`

	// BondMode is the bonding mode (e.g., balance-rr, active-backup, 802.3ad).
	// +optional
	BondMode string `json:"bondMode,omitempty"`

	// BondSlaves is the list of slave interfaces for bonding.
	// +optional
	BondSlaves []string `json:"bondSlaves,omitempty"`

	// PortVLANConfig defines bridge port VLAN settings for this uplink.
	// +optional
	PortVLANConfig *PortVLANConfig `json:"portVLANConfig,omitempty"`
}

// UplinkStatus defines the observed state of Uplink.
type UplinkStatus struct {
	// Conditions represent the latest available observations of the Uplink's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// Uplink is the Schema for the uplinks API.
type Uplink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UplinkSpec   `json:"spec,omitempty"`
	Status UplinkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UplinkList contains a list of Uplink.
type UplinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Uplink `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Uplink{}, &UplinkList{})
}
