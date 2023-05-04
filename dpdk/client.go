// Copyright 2022 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dpdk

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	dpdkproto "github.com/onmetal/net-dpservice-go/proto"
	"k8s.io/apimachinery/pkg/types"
)

type Client interface {
	GetInterface(ctx context.Context, uid types.UID) (*Interface, error)
	CreateInterface(ctx context.Context, iface *Interface) (*Interface, error)
	DeleteInterface(ctx context.Context, uid types.UID) error

	GetVirtualIP(ctx context.Context, interfaceUID types.UID) (*VirtualIP, error)
	CreateVirtualIP(ctx context.Context, virtualIP *VirtualIP) (*VirtualIP, error)
	DeleteVirtualIP(ctx context.Context, interfaceUID types.UID) error

	ListLBTargetIPs(ctx context.Context, uid types.UID) (*LBTargetIPList, error)
	CreateLBTargetIP(ctx context.Context, lbtargetIP *LBTargetIP) (*LBTargetIP, error)
	DeleteLBTargetIP(ctx context.Context, lbtargetIP *LBTargetIP) error

	ListPrefixes(ctx context.Context, interfaceUID types.UID) (*PrefixList, error)
	CreatePrefix(ctx context.Context, prefix *Prefix) (*Prefix, error)
	DeletePrefix(ctx context.Context, interfaceUID types.UID, prefix netip.Prefix) error

	ListLBPrefixes(ctx context.Context, interfaceUID types.UID) (*PrefixList, error)
	CreateLBPrefix(ctx context.Context, prefix *Prefix) (*Prefix, error)
	DeleteLBPrefix(ctx context.Context, interfaceUID types.UID, prefix netip.Prefix) error

	CreateRoute(ctx context.Context, route *Route) (*Route, error)
	DeleteRoute(ctx context.Context, route *Route) error

	GetLoadBalancer(ctx context.Context, uid types.UID) (*DpLoadBalancer, error)
	CreateLoadBalancer(ctx context.Context, lb *DpLoadBalancer) (*DpLoadBalancer, error)
	DeleteLoadBalancer(ctx context.Context, uid types.UID) error

	GetNATLocal(ctx context.Context, uid types.UID) (*NATLocal, error)
	CreateNATLocal(ctx context.Context, nl *NATLocal) (*NATLocal, error)
	DeleteNATLocal(ctx context.Context, uid types.UID) error

	CreateNATRoute(ctx context.Context, route *NATRoute) (*NATRoute, error)
	DeleteNATRoute(ctx context.Context, route *NATRoute) error

	GetNATInfoEntries(ctx context.Context, ip netip.Addr, natInfoType dpdkproto.NATInfoType) ([]NATInfo, error)
}

type NATRoute struct {
	NATRouteMetadata
	Spec NATRouteSpec
}

type NATRouteMetadata struct {
	VNI uint32
}

type NATRouteSpec struct {
	Prefix  netip.Prefix
	NextHop NATRouteNextHop
}

type NATRouteNextHop struct {
	VNI     uint32
	Address netip.Addr
	MinPort uint16
	MaxPort uint16
}

type Route struct {
	RouteMetadata
	Spec RouteSpec
}

type RouteMetadata struct {
	VNI uint32
}

type RouteSpec struct {
	Prefix  netip.Prefix
	NextHop RouteNextHop
}

type RouteNextHop struct {
	VNI     uint32
	Address netip.Addr
}

type PrefixList struct {
	Items []Prefix
}

type Prefix struct {
	PrefixMetadata
	Spec PrefixSpec
}

type PrefixMetadata struct {
	InterfaceUID types.UID
}

type PrefixSpec struct {
	Prefix        netip.Prefix
	UnderlayRoute netip.Addr
}

type VirtualIP struct {
	VirtualIPMetadata
	Spec   VirtualIPSpec
	Status VirtualIPStatus
}

type VirtualIPMetadata struct {
	InterfaceUID types.UID
}

type VirtualIPSpec struct {
	Address netip.Addr
}

type VirtualIPStatus struct {
	UnderlayRoute netip.Addr
}

type NATLocal struct {
	NATLocalMetadata
	Spec   NATLocalSpec
	Status NATLocalStatus
}

type NATLocalMetadata struct {
	InterfaceUID types.UID
}

type NATLocalSpec struct {
	Address netip.Addr
	MinPort uint32
	MaxPort uint32
}

type NATLocalStatus struct {
	UnderlayRoute netip.Addr
}

type NATInfo struct {
	Address       *netip.Addr
	UnderlayRoute *netip.Addr
	MinPort       uint32
	MaxPort       uint32
}

type LBTargetIP struct {
	LBTargetIPMetadata
	Spec LBTargetIPSpec
}

type LBTargetIPMetadata struct {
	UID types.UID
}

type LBTargetIPSpec struct {
	Address netip.Addr
}

type LBTargetIPList struct {
	Items []LBTargetIP
}

type Interface struct {
	InterfaceMetadata
	Spec   InterfaceSpec
	Status InterfaceStatus
}

type InterfaceMetadata struct {
	UID types.UID
}

type InterfaceSpec struct {
	VNI                uint32
	Device             string
	PrimaryIPv4Address netip.Addr
	PrimaryIPv6Address netip.Addr
}

type InterfaceStatus struct {
	UnderlayRoute netip.Addr
}

type DpLoadBalancer struct {
	DpLoadBalancerMetadata
	Spec   DpLoadBalancerSpec
	Status DpLoadBalancerStatus
}

type DpLoadBalancerMetadata struct {
	UID types.UID
}

type DpLoadBalancerPort struct {
	Protocol string
	Port     uint32
}

type DpLoadBalancerSpec struct {
	VNI                     uint32
	Ports                   []DpLoadBalancerPort
	LoadBalancerIPv4Address netip.Addr
}

type DpLoadBalancerStatus struct {
	UnderlayRoute netip.Addr
}

func LoadBalancerResponseToDpLoadBalancer(dpLB *dpdkproto.GetLoadBalancerResponse, LBUID types.UID) (*DpLoadBalancer, error) {
	loadBalancerIPv4Address, err := netip.ParseAddr(string(dpLB.GetLbVipIP().Address))
	if err != nil {
		return nil, fmt.Errorf("error parsing loadbalancer ipv4 address: %w", err)
	}

	var ports []DpLoadBalancerPort
	for _, dpdklLBPort := range dpLB.GetLbports() {
		DpPort := DpLoadBalancerPort{
			Port:     dpdklLBPort.GetPort(),
			Protocol: dpdklLBPort.GetProtocol().String(),
		}

		ports = append(ports, DpPort)
	}

	underlayRoute, err := netip.ParseAddr(string(dpLB.GetUnderlayRoute()))
	if err != nil {
		return nil, fmt.Errorf("error parsing underlay route: %w", err)
	}

	return &DpLoadBalancer{
		DpLoadBalancerMetadata: DpLoadBalancerMetadata{
			UID: types.UID(LBUID),
		},
		Spec: DpLoadBalancerSpec{
			VNI:                     dpLB.GetVni(),
			LoadBalancerIPv4Address: loadBalancerIPv4Address,
			Ports:                   ports,
		},
		Status: DpLoadBalancerStatus{
			UnderlayRoute: underlayRoute,
		},
	}, nil
}

func convertProtocolToProtocolType(proto string) (dpdkproto.Protocol, error) {
	protoLower := strings.ToLower(proto)

	switch {
	case strings.Contains(protoLower, "tcp"):
		return dpdkproto.Protocol_TCP, nil
	case strings.Contains(protoLower, "udp"):
		return dpdkproto.Protocol_UDP, nil
	default:
		return dpdkproto.Protocol_Undefined, errors.New("unsupported protocol type")
	}
}

func dpdkInterfaceToInterface(dpdkIface *dpdkproto.Interface) (*Interface, error) {
	primaryIPv4Address, err := netip.ParseAddr(string(dpdkIface.GetPrimaryIPv4Address()))
	if err != nil {
		return nil, fmt.Errorf("error parsing primary ipv4 address: %w", err)
	}

	primaryIPv6Address, err := netip.ParseAddr(string(dpdkIface.GetPrimaryIPv4Address()))
	if err != nil {
		return nil, fmt.Errorf("error parsing primary ipv6 address: %w", err)
	}

	underlayRoute, err := netip.ParseAddr(string(dpdkIface.GetUnderlayRoute()))
	if err != nil {
		return nil, fmt.Errorf("error parsing underlay route: %w", err)
	}

	return &Interface{
		InterfaceMetadata: InterfaceMetadata{
			UID: types.UID(dpdkIface.InterfaceID),
		},
		Spec: InterfaceSpec{
			VNI:                dpdkIface.GetVni(),
			Device:             dpdkIface.GetPciDpName(),
			PrimaryIPv4Address: primaryIPv4Address,
			PrimaryIPv6Address: primaryIPv6Address,
		},
		Status: InterfaceStatus{
			UnderlayRoute: underlayRoute,
		},
	}, nil
}

func netipAddrToDPDKIPVersion(addr netip.Addr) dpdkproto.IPVersion {
	switch {
	case addr.Is4():
		return dpdkproto.IPVersion_IPv4
	case addr.Is6():
		return dpdkproto.IPVersion_IPv6
	default:
		return 0
	}
}

func netipAddrToDPDKIPConfig(addr netip.Addr) *dpdkproto.IPConfig {
	if !addr.IsValid() {
		return nil
	}

	return &dpdkproto.IPConfig{
		IpVersion:      netipAddrToDPDKIPVersion(addr),
		PrimaryAddress: []byte(addr.String()),
	}
}

func netipAddrToLBIPConfig(addr netip.Addr) *dpdkproto.LBIP {
	if !addr.IsValid() {
		return nil
	}

	return &dpdkproto.LBIP{
		IpVersion: netipAddrToDPDKIPVersion(addr),
		Address:   []byte(addr.String()),
	}
}

type client struct {
	dpdkproto.DPDKonmetalClient
}

func NewClient(protoClient dpdkproto.DPDKonmetalClient) Client {
	return &client{protoClient}
}

func (c *client) GetInterface(ctx context.Context, uid types.UID) (*Interface, error) {
	res, err := c.DPDKonmetalClient.GetInterface(ctx, &dpdkproto.InterfaceIDMsg{InterfaceID: []byte(uid)})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetStatus().GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetStatus().GetMessage()}
	}
	return dpdkInterfaceToInterface(res.GetInterface())
}

func (c *client) CreateInterface(ctx context.Context, iface *Interface) (*Interface, error) {
	res, err := c.DPDKonmetalClient.CreateInterface(ctx, &dpdkproto.CreateInterfaceRequest{
		InterfaceType: dpdkproto.InterfaceType_VirtualInterface,
		InterfaceID:   []byte(iface.UID),
		Vni:           iface.Spec.VNI,
		Ipv4Config:    netipAddrToDPDKIPConfig(iface.Spec.PrimaryIPv4Address),
		Ipv6Config:    netipAddrToDPDKIPConfig(iface.Spec.PrimaryIPv6Address),
		DeviceName:    iface.Spec.Device,
	})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetResponse().GetStatus().GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetResponse().GetStatus().GetMessage()}
	}

	underlayRoute, err := netip.ParseAddr(string(res.GetResponse().GetUnderlayRoute()))
	if err != nil {
		return nil, fmt.Errorf("error parsing underlay route: %w", err)
	}

	return &Interface{
		InterfaceMetadata: iface.InterfaceMetadata,
		Spec:              iface.Spec, // TODO: Enable dynamic device allocation
		Status: InterfaceStatus{
			UnderlayRoute: underlayRoute,
		},
	}, nil
}

func (c *client) DeleteInterface(ctx context.Context, uid types.UID) error {
	res, err := c.DPDKonmetalClient.DeleteInterface(ctx, &dpdkproto.InterfaceIDMsg{InterfaceID: []byte(uid)})
	if err != nil {
		return err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}
	return nil
}

func dpdkVirtualIPToVirtualIP(interfaceUID types.UID, dpdkVIP *dpdkproto.InterfaceVIPIP) (*VirtualIP, error) {
	addr, err := netip.ParseAddr(string(dpdkVIP.GetAddress()))
	if err != nil {
		return nil, fmt.Errorf("error parsing virtual ip address: %w", err)
	}

	underlayAddr, err := netip.ParseAddr(string(dpdkVIP.GetUnderlayRoute()))
	if err != nil {
		return nil, fmt.Errorf("error parsing underlay address of virtual ip address: %w", err)
	}

	return &VirtualIP{
		VirtualIPMetadata: VirtualIPMetadata{
			InterfaceUID: interfaceUID,
		},
		Spec: VirtualIPSpec{
			Address: addr,
		},
		Status: VirtualIPStatus{
			UnderlayRoute: underlayAddr,
		},
	}, nil
}

func dpdkNATLocalToNATLocal(interfaceUID types.UID, dpdkNAT *dpdkproto.GetNATResponse) (*NATLocal, error) {
	addr, err := netip.ParseAddr(string(dpdkNAT.NatVIPIP.GetAddress()))
	if err != nil {
		return nil, fmt.Errorf("error parsing NAT local ip address: %w", err)
	}

	underlayAddr, err := netip.ParseAddr(string(dpdkNAT.GetUnderlayRoute()))
	if err != nil {
		return nil, fmt.Errorf("error parsing NAT local underlay address: %w", err)
	}

	return &NATLocal{
		NATLocalMetadata: NATLocalMetadata{
			InterfaceUID: interfaceUID,
		},
		Spec: NATLocalSpec{
			Address: addr,
			MinPort: dpdkNAT.MinPort,
			MaxPort: dpdkNAT.MaxPort,
		},
		Status: NATLocalStatus{
			UnderlayRoute: underlayAddr,
		},
	}, nil
}

func dpdkNATInfoEntryToNATInfo(dpdkNatInfoEntry *dpdkproto.NATInfoEntry) (*NATInfo, error) {
	info := &NATInfo{
		MinPort: dpdkNatInfoEntry.MinPort,
		MaxPort: dpdkNatInfoEntry.MaxPort,
	}

	if string(dpdkNatInfoEntry.GetAddress()) != "" {
		addr, err := netip.ParseAddr(string(dpdkNatInfoEntry.GetAddress()))
		if err != nil {
			return nil, fmt.Errorf("error parsing NATInfoEntry ip address: %w", err)
		}

		info.Address = &addr
	}

	if string(dpdkNatInfoEntry.GetUnderlayRoute()) != "" {
		underlayAddr, err := netip.ParseAddr(string(dpdkNatInfoEntry.GetUnderlayRoute()))
		if err != nil {
			return nil, fmt.Errorf("error parsing NAT info underlay address: %w", err)
		}
		info.UnderlayRoute = &underlayAddr
	}

	return info, nil
}

func (c *client) GetVirtualIP(ctx context.Context, interfaceUID types.UID) (*VirtualIP, error) {
	res, err := c.DPDKonmetalClient.GetInterfaceVIP(ctx, &dpdkproto.InterfaceIDMsg{
		InterfaceID: []byte(interfaceUID),
	})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetStatus().GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetStatus().GetMessage()}
	}

	return dpdkVirtualIPToVirtualIP(interfaceUID, res)
}

func (c *client) CreateVirtualIP(ctx context.Context, virtualIP *VirtualIP) (*VirtualIP, error) {
	res, err := c.DPDKonmetalClient.AddInterfaceVIP(ctx, &dpdkproto.InterfaceVIPMsg{
		InterfaceID: []byte(virtualIP.InterfaceUID),
		InterfaceVIPIP: &dpdkproto.InterfaceVIPIP{
			IpVersion: netipAddrToDPDKIPVersion(virtualIP.Spec.Address),
			Address:   []byte(virtualIP.Spec.Address.String()),
		},
	})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetStatus().GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetStatus().GetMessage()}
	}
	ulRoute, err := netip.ParseAddr(string(res.GetUnderlayRoute()))
	if err != nil {
		return nil, fmt.Errorf("error parsing underlayRoute: %w", err)
	}
	virtualIP.Status.UnderlayRoute = ulRoute

	return virtualIP, nil
}

func (c *client) DeleteVirtualIP(ctx context.Context, interfaceUID types.UID) error {
	res, err := c.DPDKonmetalClient.DeleteInterfaceVIP(ctx, &dpdkproto.InterfaceIDMsg{
		InterfaceID: []byte(interfaceUID),
	})
	if err != nil {
		return err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}
	return nil
}

func (c *client) GetNATLocal(ctx context.Context, interfaceUID types.UID) (*NATLocal, error) {
	res, err := c.DPDKonmetalClient.GetNAT(ctx, &dpdkproto.GetNATRequest{
		InterfaceID: []byte(interfaceUID),
	})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetStatus().GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetStatus().GetMessage()}
	}

	return dpdkNATLocalToNATLocal(interfaceUID, res)
}

func (c *client) GetNATInfoEntries(ctx context.Context, ip netip.Addr, natInfoType dpdkproto.NATInfoType) ([]NATInfo, error) {
	res, err := c.DPDKonmetalClient.GetNATInfo(ctx, &dpdkproto.GetNATInfoRequest{
		NatVIPIP: &dpdkproto.NATIP{
			IpVersion: netipAddrToDPDKIPVersion(ip),
			Address:   []byte(ip.String()),
		},
		NatInfoType: natInfoType,
	})
	if err != nil {
		return nil, err
	}

	natInfos := []NATInfo{}
	if res.GetNatInfoEntries() != nil {
		for _, natInfoEntry := range res.GetNatInfoEntries() {
			natInfo, err := dpdkNATInfoEntryToNATInfo(natInfoEntry)
			if err != nil {
				return nil, err
			}

			natInfos = append(natInfos, *natInfo)
		}
	}

	return natInfos, nil
}

func (c *client) CreateNATLocal(ctx context.Context, nl *NATLocal) (*NATLocal, error) {
	res, err := c.DPDKonmetalClient.AddNAT(ctx, &dpdkproto.AddNATRequest{
		InterfaceID: []byte(nl.InterfaceUID),
		NatVIPIP: &dpdkproto.NATIP{
			IpVersion: netipAddrToDPDKIPVersion(nl.Spec.Address),
			Address:   []byte(nl.Spec.Address.String()),
		},
		MinPort: nl.Spec.MinPort,
		MaxPort: nl.Spec.MaxPort,
	})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetStatus().GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetStatus().GetMessage()}
	}
	ulRoute, err := netip.ParseAddr(string(res.GetUnderlayRoute()))
	if err != nil {
		return nil, fmt.Errorf("error parsing underlayRoute: %w", err)
	}
	nl.Status.UnderlayRoute = ulRoute

	return nl, nil
}

func (c *client) DeleteNATLocal(ctx context.Context, interfaceUID types.UID) error {
	res, err := c.DPDKonmetalClient.DeleteNAT(ctx, &dpdkproto.DeleteNATRequest{
		InterfaceID: []byte(interfaceUID),
	})
	if err != nil {
		return err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}
	return nil
}

func (c *client) CreateLBTargetIP(ctx context.Context, lbtargetIP *LBTargetIP) (*LBTargetIP, error) {
	res, err := c.DPDKonmetalClient.AddLoadBalancerTarget(ctx, &dpdkproto.AddLoadBalancerTargetRequest{
		LoadBalancerID: []byte(lbtargetIP.UID),
		TargetIP: &dpdkproto.LBIP{
			IpVersion: netipAddrToDPDKIPVersion(lbtargetIP.Spec.Address),
			Address:   []byte(lbtargetIP.Spec.Address.String()),
		},
	})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}

	return lbtargetIP, nil
}

func (c *client) ListLBTargetIPs(ctx context.Context, uid types.UID) (*LBTargetIPList, error) {
	res, err := c.DPDKonmetalClient.GetLoadBalancerTargets(ctx, &dpdkproto.GetLoadBalancerTargetsRequest{
		LoadBalancerID: []byte(uid),
	})
	if err != nil {
		return nil, err
	}

	var targets []LBTargetIP
	for _, LBTarget := range res.GetTargetIPs() {
		target, err := dpdkLBIPToTargetIP(uid, LBTarget)
		if err != nil {
			return nil, err
		}

		targets = append(targets, *target)
	}

	return &LBTargetIPList{
		Items: targets,
	}, nil
}

func (c *client) DeleteLBTargetIP(ctx context.Context, lbtargetIP *LBTargetIP) error {
	res, err := c.DPDKonmetalClient.DeleteLoadBalancerTarget(ctx, &dpdkproto.DeleteLoadBalancerTargetRequest{
		LoadBalancerID: []byte(lbtargetIP.UID),
		TargetIP: &dpdkproto.LBIP{
			IpVersion: netipAddrToDPDKIPVersion(lbtargetIP.Spec.Address),
			Address:   []byte(lbtargetIP.Spec.Address.String()),
		},
	})
	if err != nil {
		return err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}
	return nil
}

func dpdkLBIPToTargetIP(uid types.UID, lbip *dpdkproto.LBIP) (*LBTargetIP, error) {
	addr, err := netip.ParseAddr(string(lbip.GetAddress()))
	if err != nil {
		return nil, fmt.Errorf("error parsing lb ip address: %w", err)
	}

	return &LBTargetIP{
		LBTargetIPMetadata: LBTargetIPMetadata{
			UID: uid,
		},
		Spec: LBTargetIPSpec{
			Address: addr,
		},
	}, nil
}

func dpdkPrefixToPrefix(interfaceUID types.UID, dpdkPrefix *dpdkproto.Prefix) (*Prefix, error) {
	addr, err := netip.ParseAddr(string(dpdkPrefix.GetAddress()))
	if err != nil {
		return nil, fmt.Errorf("error parsing dpdk prefix address: %w", err)
	}

	prefix, err := addr.Prefix(int(dpdkPrefix.PrefixLength))
	if err != nil {
		return nil, fmt.Errorf("invalid dpdk prefix length %d for address %s", dpdkPrefix.PrefixLength, addr)
	}

	uladdr, err := netip.ParseAddr(string(dpdkPrefix.UnderlayRoute))
	if err != nil {
		return nil, fmt.Errorf("error parsing dpdk lb prefix ul address: %w", err)
	}

	return &Prefix{
		PrefixMetadata: PrefixMetadata{
			InterfaceUID: interfaceUID,
		},
		Spec: PrefixSpec{
			Prefix:        prefix,
			UnderlayRoute: uladdr,
		},
	}, nil
}

func dpdkLBPrefixToPrefix(interfaceUID types.UID, dpdkPrefix *dpdkproto.LBPrefix) (*Prefix, error) {
	addr, err := netip.ParseAddr(string(dpdkPrefix.GetAddress()))
	if err != nil {
		return nil, fmt.Errorf("error parsing dpdk lb prefix address: %w", err)
	}

	prefix, err := addr.Prefix(int(dpdkPrefix.PrefixLength))
	if err != nil {
		return nil, fmt.Errorf("invalid dpdk lb prefix length %d for address %s", dpdkPrefix.PrefixLength, addr)
	}

	uladdr, err := netip.ParseAddr(string(dpdkPrefix.UnderlayRoute))
	if err != nil {
		return nil, fmt.Errorf("error parsing dpdk lb prefix ul address: %w", err)
	}

	return &Prefix{
		PrefixMetadata: PrefixMetadata{
			InterfaceUID: interfaceUID,
		},
		Spec: PrefixSpec{
			Prefix:        prefix,
			UnderlayRoute: uladdr,
		},
	}, nil
}

func (c *client) ListPrefixes(ctx context.Context, interfaceUID types.UID) (*PrefixList, error) {
	res, err := c.DPDKonmetalClient.ListInterfacePrefixes(ctx, &dpdkproto.InterfaceIDMsg{
		InterfaceID: []byte(interfaceUID),
	})
	if err != nil {
		return nil, err
	}

	var prefixes []Prefix
	for _, dpdkPrefix := range res.GetPrefixes() {
		prefix, err := dpdkPrefixToPrefix(interfaceUID, dpdkPrefix)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, *prefix)
	}

	return &PrefixList{
		Items: prefixes,
	}, nil
}

func (c *client) CreatePrefix(ctx context.Context, prefix *Prefix) (*Prefix, error) {
	res, err := c.DPDKonmetalClient.AddInterfacePrefix(ctx, &dpdkproto.InterfacePrefixMsg{
		InterfaceID: &dpdkproto.InterfaceIDMsg{
			InterfaceID: []byte(prefix.InterfaceUID),
		},
		Prefix: &dpdkproto.Prefix{
			IpVersion:    netipAddrToDPDKIPVersion(prefix.Spec.Prefix.Addr()),
			Address:      []byte(prefix.Spec.Prefix.Addr().String()),
			PrefixLength: uint32(prefix.Spec.Prefix.Bits()),
		},
	})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetStatus().GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetStatus().GetMessage()}
	}
	underlayRoute, err := netip.ParseAddr(string(res.GetUnderlayRoute()))
	if err != nil {
		return nil, fmt.Errorf("error parsing underlay route: %w", err)
	}
	prefix.Spec.UnderlayRoute = underlayRoute
	return prefix, nil
}

func (c *client) DeletePrefix(ctx context.Context, interfaceUID types.UID, prefix netip.Prefix) error {
	res, err := c.DPDKonmetalClient.DeleteInterfacePrefix(ctx, &dpdkproto.InterfacePrefixMsg{
		InterfaceID: &dpdkproto.InterfaceIDMsg{
			InterfaceID: []byte(interfaceUID),
		},
		Prefix: &dpdkproto.Prefix{
			IpVersion:    netipAddrToDPDKIPVersion(prefix.Addr()),
			Address:      []byte(prefix.Addr().String()),
			PrefixLength: uint32(prefix.Bits()),
		},
	})
	if err != nil {
		return err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}
	return nil
}

func (c *client) ListLBPrefixes(ctx context.Context, interfaceUID types.UID) (*PrefixList, error) {
	res, err := c.DPDKonmetalClient.ListInterfaceLoadBalancerPrefixes(ctx, &dpdkproto.ListInterfaceLoadBalancerPrefixesRequest{
		InterfaceID: []byte(interfaceUID),
	})
	if err != nil {
		return nil, err
	}

	var prefixes []Prefix
	for _, dpdkPrefix := range res.GetPrefixes() {
		prefix, err := dpdkLBPrefixToPrefix(interfaceUID, dpdkPrefix)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, *prefix)
	}

	return &PrefixList{
		Items: prefixes,
	}, nil
}

func (c *client) CreateLBPrefix(ctx context.Context, prefix *Prefix) (*Prefix, error) {
	res, err := c.DPDKonmetalClient.CreateInterfaceLoadBalancerPrefix(ctx, &dpdkproto.CreateInterfaceLoadBalancerPrefixRequest{
		InterfaceID: &dpdkproto.InterfaceIDMsg{
			InterfaceID: []byte(prefix.InterfaceUID),
		},
		Prefix: &dpdkproto.Prefix{
			IpVersion:    netipAddrToDPDKIPVersion(prefix.Spec.Prefix.Addr()),
			Address:      []byte(prefix.Spec.Prefix.Addr().String()),
			PrefixLength: uint32(prefix.Spec.Prefix.Bits()),
		},
	})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetStatus().GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetStatus().GetMessage()}
	}
	underlayRoute, err := netip.ParseAddr(string(res.GetUnderlayRoute()))
	if err != nil {
		return nil, fmt.Errorf("error parsing underlay route: %w", err)
	}
	prefix.Spec.UnderlayRoute = underlayRoute
	return prefix, nil
}

func (c *client) DeleteLBPrefix(ctx context.Context, interfaceUID types.UID, prefix netip.Prefix) error {
	res, err := c.DPDKonmetalClient.DeleteInterfaceLoadBalancerPrefix(ctx, &dpdkproto.DeleteInterfaceLoadBalancerPrefixRequest{
		InterfaceID: &dpdkproto.InterfaceIDMsg{
			InterfaceID: []byte(interfaceUID),
		},
		Prefix: &dpdkproto.Prefix{
			IpVersion:    netipAddrToDPDKIPVersion(prefix.Addr()),
			Address:      []byte(prefix.Addr().String()),
			PrefixLength: uint32(prefix.Bits()),
		},
	})
	if err != nil {
		return err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}
	return nil
}

func (c *client) CreateRoute(ctx context.Context, route *Route) (*Route, error) {
	res, err := c.DPDKonmetalClient.AddRoute(ctx, &dpdkproto.VNIRouteMsg{
		Vni: &dpdkproto.VNIMsg{Vni: route.VNI},
		Route: &dpdkproto.Route{
			IpVersion: netipAddrToDPDKIPVersion(route.Spec.NextHop.Address),
			Weight:    100,
			Prefix: &dpdkproto.Prefix{
				IpVersion:    netipAddrToDPDKIPVersion(route.Spec.Prefix.Addr()),
				Address:      []byte(route.Spec.Prefix.Addr().String()),
				PrefixLength: uint32(route.Spec.Prefix.Bits()),
			},
			NexthopVNI:     route.Spec.NextHop.VNI,
			NexthopAddress: []byte(route.Spec.NextHop.Address.String()),
		},
	})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}
	return route, nil
}

func (c *client) DeleteRoute(ctx context.Context, route *Route) error {
	res, err := c.DPDKonmetalClient.DeleteRoute(ctx, &dpdkproto.VNIRouteMsg{
		Vni: &dpdkproto.VNIMsg{Vni: route.VNI},
		Route: &dpdkproto.Route{
			IpVersion: netipAddrToDPDKIPVersion(route.Spec.NextHop.Address),
			Weight:    100,
			Prefix: &dpdkproto.Prefix{
				IpVersion:    netipAddrToDPDKIPVersion(route.Spec.Prefix.Addr()),
				Address:      []byte(route.Spec.Prefix.Addr().String()),
				PrefixLength: uint32(route.Spec.Prefix.Bits()),
			},
			NexthopVNI:     route.Spec.NextHop.VNI,
			NexthopAddress: []byte(route.Spec.NextHop.Address.String()),
		},
	})
	if err != nil {
		return err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}
	return nil
}

func (c *client) CreateNATRoute(ctx context.Context, route *NATRoute) (*NATRoute, error) {
	res, err := c.DPDKonmetalClient.AddNeighborNAT(ctx, &dpdkproto.AddNeighborNATRequest{
		Vni: route.VNI,
		NatVIPIP: &dpdkproto.NATIP{
			IpVersion: netipAddrToDPDKIPVersion(route.Spec.Prefix.Addr()),
			Address:   []byte(route.Spec.Prefix.Addr().String()),
		},
		UnderlayRoute: []byte(route.Spec.NextHop.Address.String()),
		MinPort:       uint32(route.Spec.NextHop.MinPort),
		MaxPort:       uint32(route.Spec.NextHop.MaxPort),
	})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}
	return route, nil
}

func (c *client) DeleteNATRoute(ctx context.Context, route *NATRoute) error {
	res, err := c.DPDKonmetalClient.DeleteNeighborNAT(ctx, &dpdkproto.DeleteNeighborNATRequest{
		Vni: route.VNI,
		NatVIPIP: &dpdkproto.NATIP{
			IpVersion: netipAddrToDPDKIPVersion(route.Spec.Prefix.Addr()),
			Address:   []byte(route.Spec.Prefix.Addr().String()),
		},
		MinPort: uint32(route.Spec.NextHop.MinPort),
		MaxPort: uint32(route.Spec.NextHop.MaxPort),
	})
	if err != nil {
		return err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}
	return nil
}

func (c *client) GetLoadBalancer(ctx context.Context, uid types.UID) (*DpLoadBalancer, error) {
	res, err := c.DPDKonmetalClient.GetLoadBalancer(ctx, &dpdkproto.GetLoadBalancerRequest{LoadBalancerID: []byte(uid)})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetStatus().GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetStatus().GetMessage()}
	}
	return LoadBalancerResponseToDpLoadBalancer(res, uid)
}

func (c *client) CreateLoadBalancer(ctx context.Context, dpLB *DpLoadBalancer) (*DpLoadBalancer, error) {
	var ports []*dpdkproto.LBPort
	for _, LBPort := range dpLB.Spec.Ports {
		dpdkProtocol, _ := convertProtocolToProtocolType(LBPort.Protocol)
		Port := &dpdkproto.LBPort{
			Port:     LBPort.Port,
			Protocol: dpdkProtocol,
		}
		ports = append(ports, Port)
	}
	res, err := c.DPDKonmetalClient.CreateLoadBalancer(ctx, &dpdkproto.CreateLoadBalancerRequest{
		LoadBalancerID: []byte(dpLB.UID),
		Vni:            dpLB.Spec.VNI,
		LbVipIP:        netipAddrToLBIPConfig(dpLB.Spec.LoadBalancerIPv4Address),
		Lbports:        ports,
	})
	if err != nil {
		return nil, err
	}
	if errorCode := res.GetStatus().GetError(); errorCode != 0 {
		return nil, &StatusError{errorCode: errorCode, message: res.GetStatus().GetMessage()}
	}

	underlayRoute, err := netip.ParseAddr(string(res.GetUnderlayRoute()))
	if err != nil {
		return nil, fmt.Errorf("error parsing underlay route: %w", err)
	}

	return &DpLoadBalancer{
		DpLoadBalancerMetadata: dpLB.DpLoadBalancerMetadata,
		Spec:                   dpLB.Spec,
		Status: DpLoadBalancerStatus{
			UnderlayRoute: underlayRoute,
		},
	}, nil
}

func (c *client) DeleteLoadBalancer(ctx context.Context, uid types.UID) error {
	res, err := c.DPDKonmetalClient.DeleteLoadBalancer(ctx, &dpdkproto.DeleteLoadBalancerRequest{
		LoadBalancerID: []byte(uid)})
	if err != nil {
		return err
	}
	if errorCode := res.GetError(); errorCode != 0 {
		return &StatusError{errorCode: errorCode, message: res.GetMessage()}
	}
	return nil
}
