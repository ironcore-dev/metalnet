// Copyright 2022 IronCore authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metalbond

import (
	"context"
	"net/netip"
	"sync"

	"github.com/onmetal/metalbond"
	"github.com/onmetal/metalbond/pb"
)

type DefaultRouteOperation int

const (
	AddDefaultRoute DefaultRouteOperation = iota + 1
	RemoveDefaultRoute
)

type DefaultRouterAddress struct {
	RouterAddress    netip.Addr
	PublicVNI        uint32
	SetBySubsciption bool
	RWMutex          sync.RWMutex
}

type RouteUtil interface {
	AnnounceRoute(ctx context.Context, vni VNI, destination Destination, nextHop NextHop) error
	WithdrawRoute(ctx context.Context, vni VNI, destination Destination, nextHop NextHop) error
	Subscribe(ctx context.Context, vni VNI) error
	Unsubscribe(ctx context.Context, vni VNI) error
	IsSubscribed(ctx context.Context, vni VNI) bool
	GetRoutesForVni(ctx context.Context, vni VNI) error
}

type MBRouteUtil struct {
	metalbond *metalbond.MetalBond
}

func NewMBRouteUtil(mb *metalbond.MetalBond) *MBRouteUtil {
	return &MBRouteUtil{mb}
}

type VNI = metalbond.VNI

func netIPAddrIPVersion(addr netip.Addr) metalbond.IPVersion {
	switch {
	case addr.Is4():
		return metalbond.IPV4
	case addr.Is6():
		return metalbond.IPV6
	default:
		return 0
	}
}

type Destination struct {
	Prefix netip.Prefix
}

type NextHop struct {
	TargetAddress    netip.Addr
	TargetVNI        VNI
	TargetHopType    pb.NextHopType
	TargetNATMinPort uint16
	TargetNATMaxPort uint16
}

func (c *MBRouteUtil) AnnounceRoute(_ context.Context, vni VNI, destination Destination, nextHop NextHop) error {
	return c.metalbond.AnnounceRoute(vni, metalbond.Destination{
		IPVersion: netIPAddrIPVersion(destination.Prefix.Addr()),
		Prefix:    destination.Prefix,
	}, metalbond.NextHop{
		TargetAddress:    nextHop.TargetAddress,
		TargetVNI:        uint32(nextHop.TargetVNI),
		Type:             nextHop.TargetHopType,
		NATPortRangeFrom: nextHop.TargetNATMinPort,
		NATPortRangeTo:   nextHop.TargetNATMaxPort,
	})
}

func (c *MBRouteUtil) WithdrawRoute(_ context.Context, vni VNI, destination Destination, nextHop NextHop) error {
	return c.metalbond.WithdrawRoute(vni, metalbond.Destination{
		IPVersion: netIPAddrIPVersion(destination.Prefix.Addr()),
		Prefix:    destination.Prefix,
	}, metalbond.NextHop{
		TargetAddress:    nextHop.TargetAddress,
		TargetVNI:        uint32(nextHop.TargetVNI),
		Type:             nextHop.TargetHopType,
		NATPortRangeFrom: nextHop.TargetNATMinPort,
		NATPortRangeTo:   nextHop.TargetNATMaxPort,
	})
}

func (c *MBRouteUtil) Subscribe(_ context.Context, vni VNI) error {
	return c.metalbond.Subscribe(vni)
}

func (c *MBRouteUtil) Unsubscribe(_ context.Context, vni VNI) error {
	return c.metalbond.Unsubscribe(vni)
}

func (c *MBRouteUtil) IsSubscribed(_ context.Context, vni VNI) bool {
	return c.metalbond.IsSubscribed(vni)
}

func (c *MBRouteUtil) GetRoutesForVni(_ context.Context, vni VNI) error {
	return c.metalbond.GetRoutesForVni(vni)
}
