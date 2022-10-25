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

package metalbond

import (
	"context"
	"net/netip"

	"github.com/onmetal/metalbond"
	"github.com/onmetal/metalbond/pb"
)

type Client interface {
	AddRoute(ctx context.Context, vni VNI, destination Destination, nextHop NextHop) error
	RemoveRoute(ctx context.Context, vni VNI, destination Destination, nextHop NextHop) error
	Subscribe(ctx context.Context, vni VNI) error
	Unsubscribe(ctx context.Context, vni VNI) error
}

type client struct {
	metalbond *metalbond.MetalBond
}

func NewClient(mb *metalbond.MetalBond) Client {
	return &client{mb}
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
	TargetAddress netip.Addr
	TargetVNI     VNI
	TargetHopType pb.NextHopType
}

func (c *client) AddRoute(_ context.Context, vni VNI, destination Destination, nextHop NextHop) error {
	return c.metalbond.AnnounceRoute(vni, metalbond.Destination{
		IPVersion: netIPAddrIPVersion(destination.Prefix.Addr()),
		Prefix:    destination.Prefix,
	}, metalbond.NextHop{
		TargetAddress: nextHop.TargetAddress,
		TargetVNI:     uint32(nextHop.TargetVNI),
		Type:          nextHop.TargetHopType,
	})
}

func (c *client) RemoveRoute(_ context.Context, vni VNI, destination Destination, nextHop NextHop) error {
	return c.metalbond.WithdrawRoute(vni, metalbond.Destination{
		IPVersion: netIPAddrIPVersion(destination.Prefix.Addr()),
		Prefix:    destination.Prefix,
	}, metalbond.NextHop{
		TargetAddress: nextHop.TargetAddress,
		TargetVNI:     uint32(nextHop.TargetVNI),
		Type:          nextHop.TargetHopType,
	})
}

func (c *client) Subscribe(_ context.Context, vni VNI) error {
	return c.metalbond.Subscribe(vni)
}

func (c *client) Unsubscribe(_ context.Context, vni VNI) error {
	// TODO: Check error as soon as it is implemented.
	_ = c.metalbond.Unsubscribe(vni)
	return nil
}
