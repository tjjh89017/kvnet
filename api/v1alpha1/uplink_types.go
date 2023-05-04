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
	UplinkFinalizer = "uplink.kvnet.kojuro.date/finalizer"
)

// UplinkSpec defines the desired state of Uplink
type UplinkSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Master     string   `json:"master,omitempty"`
	BondMode   string   `json:"mode,omitempty"`
	BondSlaves []string `json:"slaves,omitempty"`
}

// UplinkStatus defines the observed state of Uplink
type UplinkStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Uplink is the Schema for the uplinks API
type Uplink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UplinkSpec   `json:"spec,omitempty"`
	Status UplinkStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UplinkList contains a list of Uplink
type UplinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Uplink `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Uplink{}, &UplinkList{})
}
