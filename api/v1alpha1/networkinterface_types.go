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
	"k8s.io/apimachinery/pkg/types"
)

// NetworkInterfaceSpec defines the desired state of NetworkInterface
type NetworkInterfaceSpec struct {
	// NetworkRef is the Network this NetworkInterface is connected to
	NetworkRef corev1.LocalObjectReference `json:"networkRef"`
	// IPFamilies defines which IPFamilies this NetworkInterface is supporting
	IPFamilies []corev1.IPFamily `json:"ipFamilies"`
	// IPs are the provided IPs or EphemeralIPs which should be assigned to this NetworkInterface
	// Only one IP supported at the moment.
	// +kubebuilder:validation:Required
	IPs []IP `json:"ips"`
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
	// FirewallRules are the firewall rules to be applied to this interface.
	FirewallRules []FirewallRuleSpec `json:"firewallRules,omitempty"`
}

// NetworkInterfaceStatus defines the observed state of NetworkInterface
type NetworkInterfaceStatus struct {
	PCIAddress *PCIAddress `json:"pciAddress,omitempty"`

	// VirtualIP is any virtual ip assigned to the NetworkInterface.
	VirtualIP *IP `json:"virtualIP,omitempty"`

	// NatIP is detailed information about the NAT on this interface
	NatIP *NATDetails `json:"natIP,omitempty"`

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

// FirewallRuleSpec defines the desired state of FirewallRule
type FirewallRuleSpec struct {
	FirewallRuleID    types.UID       `json:"firewallRuleID"`
	Direction         string          `json:"direction"`
	Action            string          `json:"action"`
	Priority          *int32          `json:"priority,omitempty"`
	IpFamily          corev1.IPFamily `json:"ipFamily"`
	SourcePrefix      *IPPrefix       `json:"sourcePrefix,omitempty"`
	DestinationPrefix *IPPrefix       `json:"destinationPrefix,omitempty"`
	ProtocolMatch     *ProtocolMatch  `json:"protocolMatch,omitempty"`
}

type ProtocolMatch struct {
	ProtocolType *ProtocolType `json:"protocolType,omitempty"`
	ICMP         *ICMPMatch    `json:"icmp,omitempty"`
	PortRange    *PortMatch    `json:"portRange,omitempty"`
}

type ICMPMatch struct {
	IcmpType *int32 `json:"icmpType,omitempty"`
	IcmpCode *int32 `json:"icmpCode,omitempty"`
}

type PortMatch struct {
	SrcPort    *int32 `json:"srcPort,omitempty"`
	EndSrcPort int32  `json:"endSrcPort,omitempty"`
	DstPort    *int32 `json:"dstPort,omitempty"`
	EndDstPort int32  `json:"endDstPort,omitempty"`
}

// ProtocolType is the type for the network protocol
type ProtocolType string

const (
	// FirewallRuleProtocolTypeTCP is used for TCP traffic.
	FirewallRuleProtocolTypeTCP ProtocolType = "TCP"
	// FirewallRuleProtocolTypeUDP is used for UDP traffic.
	FirewallRuleProtocolTypeUDP ProtocolType = "UDP"
	// FirewallRuleProtocolTypeICMP is used for ICMP traffic.
	FirewallRuleProtocolTypeICMP ProtocolType = "ICMP"
)

// FirewallRuleAction is the action of the rule.
type FirewallRuleAction string

// Currently only Accept rules can be used.
const (
	// FirewallRuleAccept is used to accept traffic.
	FirewallRuleAccept FirewallRuleAction = "ACCEPT"
	// FirewallRuleDeny is used to deny traffic.
	FirewallRuleDeny FirewallRuleAction = "DENY"
)

// FirewallRuleDirection is the direction of the rule.
type FirewallRuleDirection string

const (
	// FirewallRuleIngress is used to define rules for incoming traffic.
	FirewallRuleIngress FirewallRuleDirection = "INGRESS"
	// FirewallRuleEgress is used to define rules for outgoing traffic.
	FirewallRuleEgress FirewallRuleDirection = "EGRESS"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ni
// +kubebuilder:printcolumn:name="Status",type=string,description="Status of the network interface.",JSONPath=`.status.state`,priority=0
// +kubebuilder:printcolumn:name="NodeName",type=string,description="Node the network interface is running on.",JSONPath=`.spec.nodeName`,priority=0
// +kubebuilder:printcolumn:name="Network",type=string,description="Network Reference of the network interface.",JSONPath=`.spec.networkRef.name`,priority=10
// +kubebuilder:printcolumn:name="Targets",type=string,description="loadBalancerTargets for the network interface.",JSONPath=`.spec.loadBalancerTargets`,priority=10
// +kubebuilder:printcolumn:name="IPS",type=string,description="IP Addresses of the network interface.",JSONPath=`.spec.ips`,priority=10
// +kubebuilder:printcolumn:name="VirtualIP",type=string,description="Virtual IP Address of the network interface.",JSONPath=`.spec.virtualIP`,priority=10
// +kubebuilder:printcolumn:name="Age",type=date,description="Age of the network interface.",JSONPath=`.metadata.creationTimestamp`,priority=0

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
