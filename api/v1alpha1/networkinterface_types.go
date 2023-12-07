// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NetworkInterfaceSpec defines the desired state of NetworkInterface
type NetworkInterfaceSpec struct {
	// NetworkRef is the Network this NetworkInterface is connected to
	// +kubebuilder:validation:Required
	NetworkRef corev1.LocalObjectReference `json:"networkRef"`
	// IPFamilies defines which IPFamilies this NetworkInterface is supporting
	// Only one IP supported at the moment.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=1
	IPFamilies []corev1.IPFamily `json:"ipFamilies"`
	// IPs are the provided IPs or EphemeralIPs which should be assigned to this NetworkInterface
	// Only one IP supported at the moment.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=1
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
	FirewallRules []FirewallRule `json:"firewallRules,omitempty"`
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

// FirewallRule defines the desired state of FirewallRule
type FirewallRule struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	FirewallRuleID types.UID `json:"firewallRuleID"`
	// +kubebuilder:validation:Required
	Direction FirewallRuleDirection `json:"direction"`
	// +kubebuilder:validation:Required
	Action FirewallRuleAction `json:"action"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=1000
	Priority          *int32          `json:"priority,omitempty"`
	IpFamily          corev1.IPFamily `json:"ipFamily"`
	SourcePrefix      *IPPrefix       `json:"sourcePrefix,omitempty"`
	DestinationPrefix *IPPrefix       `json:"destinationPrefix,omitempty"`
	ProtocolMatch     *ProtocolMatch  `json:"protocolMatch,omitempty"`
}

type ProtocolMatch struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=TCP;tcp;UDP;udp;ICMP;icmp
	ProtocolType *ProtocolType `json:"protocolType"`
	ICMP         *ICMPMatch    `json:"icmp,omitempty"`
	PortRange    *PortMatch    `json:"portRange,omitempty"`
}

type ICMPMatch struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=-1
	// +kubebuilder:validation:Maximum=255
	IcmpType *int32 `json:"icmpType"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=-1
	// +kubebuilder:validation:Maximum=255
	IcmpCode *int32 `json:"icmpCode"`
}

type PortMatch struct {
	// +kubebuilder:validation:Minimum=-1
	// +kubebuilder:validation:Maximum=65535
	SrcPort *int32 `json:"srcPort,omitempty"`
	// +kubebuilder:validation:Minimum=-1
	// +kubebuilder:validation:Maximum=65535
	EndSrcPort int32 `json:"endSrcPort,omitempty"`
	// +kubebuilder:validation:Minimum=-1
	// +kubebuilder:validation:Maximum=65535
	DstPort *int32 `json:"dstPort,omitempty"`
	// +kubebuilder:validation:Minimum=-1
	// +kubebuilder:validation:Maximum=65535
	EndDstPort int32 `json:"endDstPort,omitempty"`
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
	// FirewallRuleActionAccept is used to accept traffic.
	FirewallRuleActionAccept FirewallRuleAction = "Accept"
	// FirewallRuleActionDeny is used to deny traffic.
	FirewallRuleActionDeny FirewallRuleAction = "Deny"
)

// FirewallRuleDirection is the direction of the rule.
type FirewallRuleDirection string

const (
	// FirewallRuleDirectionIngress is used to define rules for incoming traffic.
	FirewallRuleDirectionIngress FirewallRuleDirection = "Ingress"
	// FirewallRuleDirectionEgress is used to define rules for outgoing traffic.
	FirewallRuleDirectionEgress FirewallRuleDirection = "Egress"
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
	// +kubebuilder:validation:Required
	Spec NetworkInterfaceSpec `json:"spec"`
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
