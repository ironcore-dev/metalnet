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

const localCall = true

type MbInternalAccess interface {
	AddRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error
	RemoveRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error
	AddLoadBalancerServer(vni uint32, uid types.UID) error
	RemoveLoadBalancerServer(vni uint32, uid types.UID) error
	GetPeerVnis(vni uint32) (sets.Set[uint32], error)
	AddVniToPeerVnis(vni, peeredVNI uint32) error
	RemoveVniFromPeerVnis(vni, peeredVNI uint32) error
}

type Client struct {
	dpdk        dpdk.Client
	config      ClientOptions
	lbServerMap map[uint32]types.UID
	vniMap      map[uint32]sets.Set[uint32]
	vniRouteMap map[uint32]*dpdk.MBRouteSet
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
		vniRouteMap: make(map[uint32]*dpdk.MBRouteSet),
	}, nil
}

func (c *Client) GetPeerVnis(vni uint32) (sets.Set[uint32], error) {
	fmt.Printf("GetPeerVnis vni:%v\n", vni)
	vnis, ok := c.vniMap[vni]

	if !ok {
		return sets.New[uint32](), nil
	}

	return vnis, nil
}

func (c *Client) AddVniToPeerVnis(vni, peeredVNI uint32) error {
	fmt.Printf("AddVniToPeerVnis vni:%v peeredVni %v\n", vni, peeredVNI)
	set, ok := c.vniMap[vni]
	if !ok {
		set = sets.New[uint32]()
		c.vniMap[vni] = set
	}
	set.Insert(peeredVNI)
	_, ok = c.vniRouteMap[peeredVNI]
	if !ok {
		c.vniRouteMap[peeredVNI] = dpdk.NewMBRouteSet()
	}
	return nil
}

func (c *Client) RemoveVniFromPeerVnis(vni, peeredVNI uint32) error {
	fmt.Printf("RemoveVniFromPeerVnis vni:%v peeredVni %v\n", vni, peeredVNI)
	set, ok := c.vniMap[vni]
	if !ok {
		return nil
	}
	peeredRoutes, _ := c.vniRouteMap[peeredVNI].List()
	for _, peeredRoute := range peeredRoutes {
		_ = c.removeLocalRoute(mb.VNI(vni), peeredRoute.Dest, peeredRoute.NextHop, localCall)
	}
	set.Delete(peeredVNI)
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

func (c *Client) addToPeeredVniRouteCache(vni uint32, route dpdk.MBRoute) error {
	fmt.Println("addToPeeredVniRouteCache vni:", vni)
	peeredVnis, _ := c.GetPeerVnis(vni)
	for peeredVni := range peeredVnis {
		fmt.Printf("addToPeeredVniRouteCache vni: %v peeredVNI: %v \n", vni, peeredVni)
		_ = c.addLocalRoute(mb.VNI(peeredVni), route.Dest, route.NextHop, localCall)
		_ = c.vniRouteMap[uint32(peeredVni)].Insert(route)
	}
	return fmt.Errorf("not a peered VNI %d not caching", vni)
}

func (c *Client) removeFromPeeredVniRouteCache(vni uint32, route dpdk.MBRoute) error {
	fmt.Println("removeFromPeeredVniRouteCache vni:", vni)
	peeredVnis, _ := c.GetPeerVnis(vni)
	for peeredVni := range peeredVnis {
		_ = c.removeLocalRoute(mb.VNI(peeredVni), route.Dest, route.NextHop, localCall)
		_ = c.vniRouteMap[uint32(peeredVni)].Delete(route)
	}
	return fmt.Errorf("not a peered VNI %d not caching", vni)
}

func (c *Client) addLocalRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop, local bool) error {
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
	if !local {
		_ = c.addToPeeredVniRouteCache(uint32(vni), dpdk.MBRoute{
			Dest:    dest,
			NextHop: hop,
		})
	}
	return nil
}

func (c *Client) removeLocalRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop, local bool) error {
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

	if !local {
		_ = c.removeFromPeeredVniRouteCache(uint32(vni), dpdk.MBRoute{
			Dest:    dest,
			NextHop: hop,
		})
	}
	return nil
}

func (c *Client) AddRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	return c.addLocalRoute(vni, dest, hop, !localCall)
}

func (c *Client) RemoveRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	return c.removeLocalRoute(vni, dest, hop, !localCall)
}
