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

// FirewallRuleSpec defines the desired state of FirewallRule
type FirewallRuleSpec struct {
	Direction         string          `json:"direction"`
	Action            string          `json:"action"`
	Priority          uint32          `json:"priority"`
	IpFamily          corev1.IPFamily `json:"ipFamily"`
	SourcePrefix      IPPrefix        `json:"sourcePrefix,omitempty"`
	DestinationPrefix IPPrefix        `json:"destinationPrefix,omitempty"`
	ProtocolMatch     ProtocolMatch   `json:"protocolMatch,omitempty"`
}

type ProtocolMatch struct {
	ProtocolType ProtocolType `json:"protocolType,omitempty"`
	ICMP         ICMPMatch    `json:"icmp,omitempty"`
	PortRange    PortMatch    `json:"portRange,omitempty"`
}

type ICMPMatch struct {
	IcmpType int32 `json:"icmpType,omitempty"`
	IcmpCode int32 `json:"icmpCode,omitempty"`
}

type PortMatch struct {
	SrcPortLower int32 `json:"srcPortLower,omitempty"`
	SrcPortUpper int32 `json:"srcPortUpper,omitempty"`
	DstPortLower int32 `json:"dstPortLower,omitempty"`
	DstPortUpper int32 `json:"dstPortUpper,omitempty"`
}

// ProtocolType is the type for the network protocol
type ProtocolType string

const (
	// FirewallRuleProtocolTypeTCP is used for TCP traffic.
	FirewallRuleProtocolTypeTCP ProtocolType = "tcp"
	// FirewallRuleProtocolTypeUDP is used for UDP traffic.
	FirewallRuleProtocolTypeUDP ProtocolType = "udp"
	// FirewallRuleProtocolTypeICMP is used for ICMP traffic.
	FirewallRuleProtocolTypeICMP ProtocolType = "icmp"
)

// FirewallRuleAction is the action of the rule.
type FirewallRuleAction string

// Currently only Accept rules can be used.
const (
	// FirewallRuleAccept is used to accept traffic.
	FirewallRuleAccept FirewallRuleAction = "accept"
	// FirewallRuleDeny is used to deny traffic.
	FirewallRuleDeny FirewallRuleAction = "deny"
)

// FirewallRuleDirection is the direction of the rule.
type FirewallRuleDirection string

const (
	// FirewallRuleIngress is used to define rules for incoming traffic.
	FirewallRuleIngress FirewallRuleDirection = "ingress"
	// FirewallRuleEgress is used to define rules for outgoing traffic.
	FirewallRuleEgress FirewallRuleDirection = "egress"
)

const (
	// Can be applied to PortLower and ProtocolType fields to match all
	FirewallRuleMatchAll int32 = -1
)

// FirewallRuleStatus defines the observed state of FirewallRule
type FirewallRuleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FirewallRule is the Schema for the firewallrules API
type FirewallRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FirewallRuleSpec   `json:"spec,omitempty"`
	Status FirewallRuleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FirewallRuleList contains a list of FirewallRule
type FirewallRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FirewallRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FirewallRule{}, &FirewallRuleList{})
}
