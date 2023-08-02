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
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	mb "github.com/onmetal/metalbond"
	mbproto "github.com/onmetal/metalbond/pb"
	dpdk "github.com/onmetal/net-dpservice-go/api"
	dpdkclient "github.com/onmetal/net-dpservice-go/client"
	dpdkerrors "github.com/onmetal/net-dpservice-go/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type MbInternalAccess interface {
	AddRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error
	RemoveRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error
	AddLoadBalancerServer(vni uint32, ip string, uid types.UID) error
	RemoveLoadBalancerServer(ip string, uid types.UID) error
	IsVniPeered(vni uint32) bool
	GetPeerVnis(vni uint32) sets.Set[uint32]
	AddVniToPeerVnis(vni, peeredVNI uint32) error
	RemoveVniFromPeerVnis(vni, peeredVNI uint32) error
	CleanupNotPeeredRoutes(vni uint32) error
	SetPeeredPrefixes(vni uint32, peeredPrefixes map[uint32][]netip.Prefix)
}

type Client struct {
	dpdk           dpdkclient.Client
	config         ClientOptions
	lbServerMap    map[uint32]map[string]types.UID
	peeredPrefixes map[uint32]map[uint32][]netip.Prefix
	peeredVnis     map[uint32]sets.Set[uint32]
	mtxPeeredVnis  sync.RWMutex
	log            *logr.Logger
}

type ClientOptions struct {
	IPv4Only         bool
	PreferredNetwork *net.IPNet
}

func NewClient(log *logr.Logger, dpdkClient dpdkclient.Client, opts ClientOptions) (*Client, error) {
	return &Client{
		dpdk:           dpdkClient,
		config:         opts,
		lbServerMap:    make(map[uint32]map[string]types.UID),
		peeredPrefixes: make(map[uint32]map[uint32][]netip.Prefix),
		peeredVnis:     make(map[uint32]sets.Set[uint32]),
		log:            log,
	}, nil
}

func (c *Client) SetPeeredPrefixes(vni uint32, peeredPrefixes map[uint32][]netip.Prefix) {
	c.peeredPrefixes[vni] = peeredPrefixes
}

func (c *Client) IsVniPeered(vni uint32) bool {
	c.mtxPeeredVnis.RLock()
	defer c.mtxPeeredVnis.RUnlock()
	for _, peeredVnis := range c.peeredVnis {
		if peeredVnis.Has(vni) {
			return true
		}
	}
	return false
}

func (c *Client) GetPeerVnis(vni uint32) sets.Set[uint32] {
	c.mtxPeeredVnis.RLock()
	defer c.mtxPeeredVnis.RUnlock()
	vnis, ok := c.peeredVnis[vni]
	if !ok {
		return sets.New[uint32]()
	}
	return vnis
}

func (c *Client) AddVniToPeerVnis(vni, peeredVNI uint32) error {
	c.mtxPeeredVnis.Lock()
	defer c.mtxPeeredVnis.Unlock()
	c.log.V(1).Info("Adding to peered VNI list", "VNI", vni, "peeredVNI", peeredVNI)
	set, ok := c.peeredVnis[vni]
	if !ok {
		set = sets.New[uint32]()
		c.peeredVnis[vni] = set
	}
	set.Insert(peeredVNI)
	c.log.V(1).Info("Added to peered VNI list", "VNI", vni, "peeredVNI", peeredVNI)
	return nil
}

func (c *Client) RemoveVniFromPeerVnis(vni, peeredVNI uint32) error {
	c.mtxPeeredVnis.Lock()
	defer c.mtxPeeredVnis.Unlock()
	c.log.V(1).Info("Removing from peered VNI list", "VNI", vni, "peeredVNI", peeredVNI)
	set, ok := c.peeredVnis[vni]
	if !ok {
		return nil
	}
	set.Delete(peeredVNI)
	c.log.V(1).Info("Removed from peered VNI list", "VNI", vni, "peeredVNI", peeredVNI)
	return nil
}

func (c *Client) AddLoadBalancerServer(vni uint32, ip string, uid types.UID) error {
	if _, exists := c.lbServerMap[vni]; !exists {
		c.lbServerMap[vni] = make(map[string]types.UID)
	}
	c.lbServerMap[vni][ip] = uid
	return nil
}

func (c *Client) RemoveLoadBalancerServer(ip string, uid types.UID) error {
	for _, innerMap := range c.lbServerMap {
		for keyIp, value := range innerMap {
			if ip == keyIp && value == uid {
				delete(innerMap, ip)
			}
		}
	}
	return nil
}

func (c *Client) addLocalRoute(destVni mb.VNI, vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	ctx := context.TODO()

	if c.config.IPv4Only && dest.IPVersion != mb.IPV4 {
		// log.Infof("Received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
		return fmt.Errorf("received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
	}

	if hop.Type == mbproto.NextHopType_LOADBALANCER_TARGET {
		ip := dest.Prefix.Addr().String()
		_, ok := c.lbServerMap[uint32(vni)][ip]
		if !ok {
			return fmt.Errorf("no registered LoadBalancer on this client for vni %d and ip %s", vni, ip)
		}

		if c.config.PreferredNetwork != nil {
			targetAddress := net.ParseIP(hop.TargetAddress.String())
			if !c.config.PreferredNetwork.Contains(targetAddress) {
				c.log.V(1).Info(fmt.Sprintf("LB target %s is not in preferred network %s, ignoring...", targetAddress, c.config.PreferredNetwork))
				return nil
			}
		}

		if _, err := c.dpdk.CreateLoadBalancerTarget(ctx, &dpdk.LoadBalancerTarget{
			LoadBalancerTargetMeta: dpdk.LoadBalancerTargetMeta{
				LoadbalancerID: string(c.lbServerMap[uint32(vni)][ip]),
			},
			Spec: dpdk.LoadBalancerTargetSpec{
				TargetIP: &hop.TargetAddress,
			},
		}); dpdkerrors.IgnoreStatusErrorCode(err, dpdkerrors.ALREADY_EXISTS) != nil {
			return fmt.Errorf("error creating lb target: %w", err)
		}
		return nil
	}

	if hop.Type == mbproto.NextHopType_NAT {
		natIP := dest.Prefix.Addr()
		if _, err := c.dpdk.CreateNeighborNat(ctx, &dpdk.NeighborNat{
			NeighborNatMeta: dpdk.NeighborNatMeta{
				NatIP: &natIP,
			},
			Spec: dpdk.NeighborNatSpec{
				Vni:     uint32(vni),
				MinPort: uint32(hop.NATPortRangeFrom),
				MaxPort: uint32(hop.NATPortRangeTo),
			},
		}); dpdkerrors.IgnoreStatusErrorCode(err, dpdkerrors.ALREADY_EXISTS) != nil {
			return fmt.Errorf("error nat route: %w", err)
		}
		return nil
	}

	if _, err := c.dpdk.CreateRoute(ctx, &dpdk.Route{
		RouteMeta: dpdk.RouteMeta{
			VNI: uint32(vni),
		},
		Spec: dpdk.RouteSpec{
			Prefix: &dest.Prefix,
			NextHop: &dpdk.RouteNextHop{
				VNI: uint32(destVni),
				IP:  &hop.TargetAddress,
			},
		},
	}); dpdkerrors.IgnoreStatusErrorCode(err, dpdkerrors.ROUTE_EXISTS) != nil {
		return fmt.Errorf("error creating route: %w", err)
	}
	return nil
}

func (c *Client) removeLocalRoute(destVni mb.VNI, vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	ctx := context.TODO()

	if c.config.IPv4Only && dest.IPVersion != mb.IPV4 {
		// log.Infof("Received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
		return fmt.Errorf("received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
	}

	if hop.Type == mbproto.NextHopType_LOADBALANCER_TARGET {
		ip := dest.Prefix.Addr().String()
		_, ok := c.lbServerMap[uint32(vni)][ip]
		if !ok {
			return fmt.Errorf("no registered LoadBalancer on this client for vni %d and ip %s", vni, ip)
		}
		if _, err := c.dpdk.DeleteLoadBalancerTarget(
			ctx,
			string(c.lbServerMap[uint32(vni)][ip]),
			&hop.TargetAddress,
		); dpdkerrors.IgnoreStatusErrorCode(err, dpdkerrors.NOT_FOUND, dpdkerrors.NO_BACKIP, dpdkerrors.NO_LB) != nil {
			return fmt.Errorf("error deleting lb target: %w", err)
		}
		return nil
	}

	if hop.Type == mbproto.NextHopType_NAT {
		natIP := dest.Prefix.Addr()
		if _, err := c.dpdk.DeleteNeighborNat(ctx, &dpdk.NeighborNat{
			NeighborNatMeta: dpdk.NeighborNatMeta{
				NatIP: &natIP,
			},
			Spec: dpdk.NeighborNatSpec{
				Vni:     uint32(vni),
				MinPort: uint32(hop.NATPortRangeFrom),
				MaxPort: uint32(hop.NATPortRangeTo),
			},
		}); dpdkerrors.IgnoreStatusErrorCode(err, dpdkerrors.NOT_FOUND) != nil {
			return fmt.Errorf("error deleting nat route: %w", err)
		}
		return nil
	}

	if _, err := c.dpdk.DeleteRoute(
		ctx,
		uint32(vni),
		&dest.Prefix,
	); dpdkerrors.IgnoreStatusErrorCode(err, dpdkerrors.NO_VNI, dpdkerrors.ROUTE_NOT_FOUND) != nil {
		return fmt.Errorf("error deleting route: %w", err)
	}
	return nil
}

func (c *Client) AddRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	c.log.V(1).Info("AddRoute", "VNI", vni, "dest", dest, "hop", hop)
	var errStrs []string

	if err := c.addLocalRoute(vni, vni, dest, hop); err != nil {
		errStrs = append(errStrs, err.Error())
	}

	if hop.Type == mbproto.NextHopType_STANDARD {
		mbPeerVnis := c.GetPeerVnis(uint32(vni))
		peeredPrefixes, ok := c.peeredPrefixes[uint32(vni)]
		c.log.V(1).Info("GetPeerVnis", "VNI", vni, "mbPeerVnis", mbPeerVnis, "peeredPrefixes", peeredPrefixes)

		for _, peeredVNI := range mbPeerVnis.UnsortedList() {
			// by default, we add the route if no peered prefixes are set
			addRoute := true
			if ok {
				allowedPeeredPrefixes, exists := peeredPrefixes[peeredVNI]
				// if we have set peered prefixes for this VNI, we need to check if the destination is in the list
				if exists {
					// if the destination is not in the list of peered prefixes, we don't add the route
					addRoute = false
					for _, peeredPrefix := range allowedPeeredPrefixes {
						if peeredPrefix.Contains(dest.Prefix.Addr()) {
							addRoute = true
							break
						}
					}
				}
			}

			if addRoute {
				if err := c.addLocalRoute(vni, mb.VNI(peeredVNI), dest, hop); err != nil {
					errStrs = append(errStrs, err.Error())
				}
			}
		}
	}

	if len(errStrs) > 0 {
		return errors.New(strings.Join(errStrs, "\n"))
	}

	return nil
}

func (c *Client) RemoveRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	c.log.V(1).Info("RemoveRoute", "VNI", vni, "dest", dest, "hop", hop)
	var errStrs []string

	if err := c.removeLocalRoute(vni, vni, dest, hop); err != nil {
		errStrs = append(errStrs, err.Error())
	}
	mbPeerVnis := c.GetPeerVnis(uint32(vni))
	for _, peeredVNI := range mbPeerVnis.UnsortedList() {
		if hop.Type == mbproto.NextHopType_STANDARD {
			if err := c.removeLocalRoute(vni, mb.VNI(peeredVNI), dest, hop); err != nil {
				errStrs = append(errStrs, err.Error())
			}
		}
	}

	if len(errStrs) > 0 {
		return errors.New(strings.Join(errStrs, "\n"))
	}

	return nil
}

func (c *Client) CleanupNotPeeredRoutes(vni uint32) error {
	ctx := context.TODO()

	routes, err := c.dpdk.ListRoutes(ctx, vni)
	if err != nil {
		return fmt.Errorf("error listing dpdk routes for vni %d: %w", vni, err)
	}

	set, ok := c.peeredVnis[vni]

	// loop over all routes and delete the ones that are not peered
	for _, route := range routes.Items {
		// only delete route if it is not the local vni and not peered
		if route.Spec.NextHop.VNI != vni && (ok && !set.Has(route.Spec.NextHop.VNI)) {
			if _, err := c.dpdk.DeleteRoute(ctx, vni, route.Spec.Prefix); dpdkerrors.IgnoreStatusErrorCode(err, dpdkerrors.NO_VNI, dpdkerrors.ROUTE_NOT_FOUND) != nil {
				return fmt.Errorf("error deleting route: %w", err)
			}
		}
	}

	return nil
}
