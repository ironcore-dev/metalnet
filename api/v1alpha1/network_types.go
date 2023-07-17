/*
Copyright 2022 The Metal Authors.

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

// NetworkSpec defines the desired state of Network
type NetworkSpec struct {
	// +kubebuilder:validation:Maximum=16777215
	// +kubebuilder:validation:Minimum=1
	// ID is the unique identifier of the Network
	ID int32 `json:"id"`

	// PeeredIDs are the IDs of networks to peer with.
	PeeredIDs []int32 `json:"peeredIDs,omitempty"`

	// PeeredPrefixes are the allowed CIDRs of the peered networks.
	// +optional
	// +patchMergeKey=id
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=id
	PeeredPrefixes []PeeredPrefix `json:"peeredPrefixes,omitempty" patchStrategy:"merge" patchMergeKey:"peeredPrefixes"`
}

// PeeredPrefix contains information of the peered networks and their allowed CIDRs.
type PeeredPrefix struct {
	ID       int32      `json:"id"`
	Prefixes []IPPrefix `json:"prefixes"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Handle",type=integer,description="ID of the network.",JSONPath=`.spec.id`,priority=10
// +kubebuilder:printcolumn:name="Age",type=date,description="Age of the network.",JSONPath=`.metadata.creationTimestamp`,priority=0

// Network is the Schema for the networks API
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NetworkSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// NetworkList contains a list of Network
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Network{}, &NetworkList{})
}
