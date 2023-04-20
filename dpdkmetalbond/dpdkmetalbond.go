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

package dpdkmetalbond

import (
	"context"
	"fmt"

	mb "github.com/onmetal/metalbond"
	mbproto "github.com/onmetal/metalbond/pb"
	"github.com/onmetal/metalnet/dpdk"
	dpdkproto "github.com/onmetal/net-dpservice-go/proto"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type MbInternalAccess interface {
	AddRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error
	RemoveRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error
	AddLoadBalancerServer(vni uint32, uid types.UID) error
	RemoveLoadBalancerServer(vni uint32, uid types.UID) error
	AddPeerVnis(vni uint32, peeredVnis sets.Set[uint32]) error
	GetPeerVnis(vni uint32) (sets.Set[uint32], error)
	AddVniToPeerVnis(vni, value uint32) error
	RemoveVniFromPeerVnis(vni, value uint32) error
}

type Client struct {
	dpdk        dpdk.Client
	config      ClientOptions
	lbServerMap map[uint32]types.UID
	vniMap      map[uint32]sets.Set[uint32]
	vniRouteMap map[uint32]*dpdk.RouteSpecSet
}

type ClientOptions struct {
	IPv4Only bool
}

func NewClient(dpdkClient dpdk.Client, opts ClientOptions) (*Client, error) {
	return &Client{
		dpdk:        dpdkClient,
		config:      opts,
		lbServerMap: make(map[uint32]types.UID),
		vniMap:      make(map[uint32]sets.Set[uint32]),
		vniRouteMap: make(map[uint32]*dpdk.RouteSpecSet),
	}, nil
}

func (c *Client) AddPeerVnis(vni uint32, peeredVnis sets.Set[uint32]) error {
	c.vniMap[vni] = peeredVnis
	return nil
}

func (c *Client) GetPeerVnis(vni uint32) (sets.Set[uint32], error) {
	vnis, ok := c.vniMap[vni]

	if !ok {
		return sets.New[uint32](), nil
	}

	return vnis, nil
}

func (c *Client) AddVniToPeerVnis(vni, value uint32) error {
	set, ok := c.vniMap[vni]
	if !ok {
		set = sets.New[uint32]()
		c.vniMap[vni] = set
		c.vniRouteMap[vni] = dpdk.NewRouteSpecSet()
	}
	set.Insert(value)
	return nil
}

func (c *Client) RemoveVniFromPeerVnis(vni, value uint32) error {
	set, ok := c.vniMap[vni]
	if !ok {
		return nil
	}
	set.Delete(value)
	return nil
}

func (c *Client) AddLoadBalancerServer(vni uint32, uid types.UID) error {
	c.lbServerMap[vni] = uid
	return nil
}

func (c *Client) RemoveLoadBalancerServer(vni uint32, uid types.UID) error {
	delete(c.lbServerMap, vni)
	return nil
}

func (c *Client) AddRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	ctx := context.TODO()

	if c.config.IPv4Only && dest.IPVersion != mb.IPV4 {
		// log.Infof("Received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
		return fmt.Errorf("received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
	}
	if hop.Type == mbproto.NextHopType_LOADBALANCER_TARGET {
		_, ok := c.lbServerMap[uint32(vni)]
		if !ok {
			return fmt.Errorf("no registered LoadBalancer on this client for vni %d", vni)
		}
		if _, err := c.dpdk.CreateLBTargetIP(ctx, &dpdk.LBTargetIP{
			LBTargetIPMetadata: dpdk.LBTargetIPMetadata{
				UID: c.lbServerMap[uint32(vni)],
			},
			Spec: dpdk.LBTargetIPSpec{
				Address: hop.TargetAddress,
			},
		}); dpdk.IgnoreStatusErrorCode(err, dpdk.ADD_RT_FAIL4) != nil {
			return fmt.Errorf("error creating lb route: %w", err)
		}
		return nil
	}

	if hop.Type == mbproto.NextHopType_NAT {
		if _, err := c.dpdk.CreateNATRoute(ctx, &dpdk.NATRoute{
			NATRouteMetadata: dpdk.NATRouteMetadata{
				VNI: uint32(vni),
			},
			Spec: dpdk.NATRouteSpec{
				Prefix: dest.Prefix,
				NextHop: dpdk.NATRouteNextHop{
					VNI:     uint32(vni),
					Address: hop.TargetAddress,
					MinPort: hop.NATPortRangeFrom,
					MaxPort: hop.NATPortRangeTo,
				},
			},
		}); dpdk.IgnoreStatusErrorCode(err, dpdk.ADD_NEIGHNAT_EXIST) != nil {
			return fmt.Errorf("error nat route: %w", err)
		}
		return nil
	}

	prefix := &dpdkproto.Prefix{
		PrefixLength: uint32(dest.Prefix.Bits()),
	}

	prefix.IpVersion = dpdkproto.IPVersion_IPv4 //only ipv4 in overlay is supported so far
	prefix.Address = []byte(dest.Prefix.Addr().String())

	if _, err := c.dpdk.CreateRoute(ctx, &dpdk.Route{
		RouteMetadata: dpdk.RouteMetadata{
			VNI: uint32(vni),
		},
		Spec: dpdk.RouteSpec{
			Prefix: dest.Prefix,
			NextHop: dpdk.RouteNextHop{
				VNI:     uint32(vni),
				Address: hop.TargetAddress,
			},
		},
	}); dpdk.IgnoreStatusErrorCode(err, dpdk.ADD_RT_FAIL4) != nil {
		return fmt.Errorf("error creating route: %w", err)
	}
	_ = c.vniRouteMap[uint32(vni)].Insert(dpdk.RouteSpec{
		Prefix: dest.Prefix,
		NextHop: dpdk.RouteNextHop{
			VNI:     uint32(vni),
			Address: hop.TargetAddress,
		},
	})
	return nil
}

func (c *Client) RemoveRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	ctx := context.TODO()

	if c.config.IPv4Only && dest.IPVersion != mb.IPV4 {
		// log.Infof("Received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
		return fmt.Errorf("received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
	}
	if hop.Type == mbproto.NextHopType_LOADBALANCER_TARGET {
		_, ok := c.lbServerMap[uint32(vni)]
		if !ok {
			return fmt.Errorf("no registered LoadBalancer on this client for vni %d", vni)
		}
		if err := c.dpdk.DeleteLBTargetIP(ctx, &dpdk.LBTargetIP{
			LBTargetIPMetadata: dpdk.LBTargetIPMetadata{
				UID: c.lbServerMap[uint32(vni)],
			},
			Spec: dpdk.LBTargetIPSpec{
				Address: hop.TargetAddress,
			},
		}); dpdk.IgnoreStatusErrorCode(err, dpdk.ADD_RT_FAIL4) != nil {
			return fmt.Errorf("error deleting lb route: %w", err)
		}
		return nil
	}

	if hop.Type == mbproto.NextHopType_NAT {
		if err := c.dpdk.DeleteNATRoute(ctx, &dpdk.NATRoute{
			NATRouteMetadata: dpdk.NATRouteMetadata{
				VNI: uint32(vni),
			},
			Spec: dpdk.NATRouteSpec{
				Prefix: dest.Prefix,
				NextHop: dpdk.NATRouteNextHop{
					VNI:     uint32(vni),
					Address: hop.TargetAddress,
					MinPort: hop.NATPortRangeFrom,
					MaxPort: hop.NATPortRangeTo,
				},
			},
		}); dpdk.IgnoreStatusErrorCode(err, dpdk.DEL_NEIGHNAT_NOFOUND) != nil {
			return fmt.Errorf("error deleting nat route: %w", err)
		}
		return nil
	}

	if err := c.dpdk.DeleteRoute(ctx, &dpdk.Route{
		RouteMetadata: dpdk.RouteMetadata{
			VNI: uint32(vni),
		},
		Spec: dpdk.RouteSpec{
			Prefix: dest.Prefix,
			NextHop: dpdk.RouteNextHop{
				VNI:     uint32(vni),
				Address: hop.TargetAddress,
			},
		},
	}); dpdk.IgnoreStatusErrorCode(err, dpdk.DEL_RT) != nil {
		return fmt.Errorf("error deleting route: %w", err)
	}
	return nil
}
