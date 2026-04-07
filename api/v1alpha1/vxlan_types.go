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

// VXLANSpec defines the desired state of VXLAN.
type VXLANSpec struct {
	// VNI is the VXLAN Network Identifier used in traditional (non-external) mode.
	// Ignored when External is true.
	// +optional
	VNI int `json:"vni,omitempty"`

	// Master is the bridge device this VXLAN is attached to.
	// +optional
	Master string `json:"master,omitempty"`

	// LocalIP is the source IP for VXLAN encapsulation.
	// If empty, the agent will auto-detect from the Dev interface.
	// +optional
	LocalIP string `json:"localIP,omitempty"`

	// Dev is the device used to determine the local IP for VXLAN.
	// +optional
	Dev string `json:"dev,omitempty"`

	// MTU for the VXLAN interface. Default 0 means do not set (use system default).
	// Only applied when non-zero.
	// +optional
	MTU int `json:"mtu,omitempty"`

	// Learning enables or disables dynamic VTEP learning on the VXLAN interface.
	// This is the kernel vxlan driver's [no]learning setting.
	// Set via: ip link set {dev} type vxlan [no]learning
	// Should be disabled for EVPN setups.
	// +optional
	Learning bool `json:"learning,omitempty"`

	// BridgeLearning enables or disables dynamic MAC learning on the bridge port.
	// This is the kernel bridge driver's per-port learning setting.
	// Set via: bridge link set dev {dev} learning {on|off}
	// Should be disabled for EVPN setups.
	// +optional
	BridgeLearning bool `json:"bridgeLearning,omitempty"`

	// External enables single VXLAN device mode (collect_metadata).
	// When true, VXLANID from template is ignored; VNI mapping is done via
	// bridge VLAN tunnel_info in PortVLANConfig.
	// +optional
	External bool `json:"external,omitempty"`

	// VNIFilter enables per-VNI filtering on the VXLAN device.
	// Only VNIs explicitly added via bridge vni add are accepted.
	// Requires External: true.
	// +optional
	VNIFilter bool `json:"vniFilter,omitempty"`

	// PortVLANConfig defines bridge port VLAN settings for this VXLAN device.
	// +optional
	PortVLANConfig *PortVLANConfig `json:"portVLANConfig,omitempty"`
}

// VXLANStatus defines the observed state of VXLAN.
type VXLANStatus struct {
	// LocalIP is the observed local IP address used for this VXLAN.
	// +optional
	LocalIP string `json:"localIP,omitempty"`

	// Conditions represent the latest available observations of the VXLAN's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// VXLAN is the Schema for the vxlans API.
type VXLAN struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VXLANSpec   `json:"spec,omitempty"`
	Status VXLANStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VXLANList contains a list of VXLAN.
type VXLANList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VXLAN `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VXLAN{}, &VXLANList{})
}
