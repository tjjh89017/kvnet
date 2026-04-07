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

// PortVLANConfig defines bridge port VLAN settings.
type PortVLANConfig struct {
	// Pvid is the port VLAN ID (native/untagged VLAN).
	// +optional
	Pvid *int `json:"pvid,omitempty"`

	// Vids is the list of VLAN IDs allowed on this port (trunk VLANs).
	// +optional
	Vids []int `json:"vids,omitempty"`

	// TunnelInfo maps VIDs to VNIs for VXLAN tunnel (single VXLAN device mode).
	// +optional
	TunnelInfo []VLANTunnelMapping `json:"tunnelInfo,omitempty"`
}

// VLANTunnelMapping maps a VLAN ID (or range) to a VXLAN Network Identifier.
// When VidEnd is set, maps vid..vidEnd to vni..vni+(vidEnd-vid) consecutively.
type VLANTunnelMapping struct {
	// Vid is the VLAN ID (or start of range when VidEnd is set).
	Vid int `json:"vid"`

	// VidEnd is the inclusive end of the VLAN range.
	// When set, maps vid..vidEnd to vni..vni+(vidEnd-vid).
	// +optional
	VidEnd *int `json:"vidEnd,omitempty"`

	// Vni is the VXLAN Network Identifier (or start of VNI range).
	Vni int `json:"vni"`
}
