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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkInterfaceSpec defines the desired state of NetworkInterface
type NetworkInterfaceSpec struct {
	// NetworkRef is the Network this NetworkInterface is connected to
	NetworkRef corev1.LocalObjectReference `json:"networkRef"`
	// IPFamilies defines which IPFamilies this NetworkInterface is supporting
	IPFamilies []corev1.IPFamily `json:"ipFamilies"`
	// IPs are the provided IPs or EphemeralIPs which should be assigned to this NetworkInterface
	// Only one IP supported at the moment.
	// +optional
	IPs []IP `json:"ips,omitempty"`
	// Virtual IP
	VirtualIP *IP `json:"virtualIP,omitempty"`
	// Prefixes are the provided Prefix
	Prefixes []IPPrefix `json:"prefixes,omitempty"`
	// Loadbalancer Targets are the provided Prefix
	LoadBalancerTargets []IPPrefix `json:"loadBalancerTargets,omitempty"`
	// NATInfo is detailed information about the NAT on this interface
	NAT *NATDetails `json:"nat,omitempty"`
	// NodeName is the name of the node on which the interface should be created.
	NodeName *string `json:"nodeName,omitempty"`
}

// NetworkInterfaceStatus defines the observed state of NetworkInterface
type NetworkInterfaceStatus struct {
	PCIAddress *PCIAddress `json:"pciAddress,omitempty"`

	// VirtualIP is any virtual ip assigned to the NetworkInterface.
	VirtualIP *IP `json:"virtualIP,omitempty"`

	// NatIP is any nat ip assigned to the NetworkInterface.
	NatIP *IP `json:"natIP,omitempty"`

	// Prefixes are the Prefixes reserved for this NetworkInterface
	Prefixes []IPPrefix `json:"prefixes,omitempty"`

	// LoadBalancerTargets are the Targets reserved for this NetworkInterface
	LoadBalancerTargets []IPPrefix `json:"loadBalancerTargets,omitempty"`

	// State is the NetworkInterfaceState of the NetworkInterface.
	State NetworkInterfaceState `json:"state,omitempty"`
}

// PCIAddress is a PCI address.
type PCIAddress struct {
	Domain   string `json:"domain,omitempty"`
	Bus      string `json:"bus,omitempty"`
	Slot     string `json:"slot,omitempty"`
	Function string `json:"function,omitempty"`
}

// NetworkInterfaceState is the binding state of a NetworkInterface.
type NetworkInterfaceState string

const (
	// NetworkInterfaceStateReady is used for any NetworkInterface that is ready.
	NetworkInterfaceStateReady NetworkInterfaceState = "Ready"
	// NetworkInterfaceStatePending is used for any NetworkInterface that is in an intermediate state.
	NetworkInterfaceStatePending NetworkInterfaceState = "Pending"
	// NetworkInterfaceStateError is used for any NetworkInterface that is some error occurred.
	NetworkInterfaceStateError NetworkInterfaceState = "Error"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ni
// +kubebuilder:printcolumn:name="Status",type=string,description="Status of the network interface.",JSONPath=`.status.state`,priority=0
// +kubebuilder:printcolumn:name="NodeName",type=string,description="Node the network interface is running on.",JSONPath=`.spec.nodeName`,priority=0

// NetworkInterface is the Schema for the networkinterfaces API
type NetworkInterface struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of NetworkInterface.
	Spec NetworkInterfaceSpec `json:"spec,omitempty"`
	// Status defines the observed state of NetworkInterface.
	Status NetworkInterfaceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NetworkInterfaceList contains a list of NetworkInterface
type NetworkInterfaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is a list of NetworkInterface.
	Items []NetworkInterface `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkInterface{}, &NetworkInterfaceList{})
}
