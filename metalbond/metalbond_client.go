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
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/go-logr/logr"
	dpdk "github.com/ironcore-dev/dpservice-go/api"
	dpdkclient "github.com/ironcore-dev/dpservice-go/client"
	dpdkerrors "github.com/ironcore-dev/dpservice-go/errors"
	mb "github.com/ironcore-dev/metalbond"
	mbproto "github.com/ironcore-dev/metalbond/pb"
	"github.com/ironcore-dev/metalnet/internal"
)

type ClientOptions struct {
	IPv4Only         bool
	PreferredNetwork *net.IPNet
}

type MetalnetClient struct {
	dpdk                 dpdkclient.Client
	config               ClientOptions
	metalnetCache        *internal.MetalnetCache
	mbInstance           *mb.MetalBond
	DefaultRouterAddress *DefaultRouterAddress

	log *logr.Logger
}

func NewMetalnetClient(log *logr.Logger, dpdkClient dpdkclient.Client, metalnetCache *internal.MetalnetCache, routerAddr *DefaultRouterAddress, opts ClientOptions) *MetalnetClient {
	return &MetalnetClient{
		dpdk:                 dpdkClient,
		metalnetCache:        metalnetCache,
		DefaultRouterAddress: routerAddr,
		config:               opts,
		log:                  log,
	}
}

func (c *MetalnetClient) SetMetalBond(mb *mb.MetalBond) {
	c.mbInstance = mb
}

func (c *MetalnetClient) addLocalRoute(destVni mb.VNI, vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	ctx := context.TODO()

	if c.config.IPv4Only && dest.IPVersion != mb.IPV4 {
		// log.Infof("Received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
		return fmt.Errorf("received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
	}

	if hop.Type == mbproto.NextHopType_LOADBALANCER_TARGET {
		ip := dest.Prefix.Addr().String()
		uid, ok := c.metalnetCache.GetLoadBalancerServer(uint32(vni), ip)
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
				LoadbalancerID: string(uid),
			},
			Spec: dpdk.LoadBalancerTargetSpec{
				TargetIP: &hop.TargetAddress,
			},
		}, dpdkerrors.Ignore(dpdkerrors.ALREADY_EXISTS),
		); err != nil {
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
				Vni:           uint32(vni),
				MinPort:       uint32(hop.NATPortRangeFrom),
				MaxPort:       uint32(hop.NATPortRangeTo),
				UnderlayRoute: &hop.TargetAddress,
			},
		}, dpdkerrors.Ignore(dpdkerrors.ALREADY_EXISTS),
		); err != nil {
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
	}, dpdkerrors.Ignore(dpdkerrors.ROUTE_EXISTS),
	); err != nil {
		return fmt.Errorf("error creating route: %w", err)
	}
	return nil
}

func (c *MetalnetClient) removeLocalRoute(destVni mb.VNI, vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	ctx := context.TODO()

	if c.config.IPv4Only && dest.IPVersion != mb.IPV4 {
		// log.Infof("Received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
		return fmt.Errorf("received non-IPv4 route will not be installed in kernel route table (IPv4-only mode)")
	}

	if hop.Type == mbproto.NextHopType_LOADBALANCER_TARGET {
		ip := dest.Prefix.Addr().String()
		uid, ok := c.metalnetCache.GetLoadBalancerServer(uint32(vni), ip)
		if !ok {
			return fmt.Errorf("no registered LoadBalancer on this client for vni %d and ip %s", vni, ip)
		}
		if _, err := c.dpdk.DeleteLoadBalancerTarget(
			ctx,
			string(uid),
			&hop.TargetAddress,
			dpdkerrors.Ignore(dpdkerrors.NOT_FOUND, dpdkerrors.NO_BACKIP, dpdkerrors.NO_LB),
		); err != nil {
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
				Vni:           uint32(vni),
				MinPort:       uint32(hop.NATPortRangeFrom),
				MaxPort:       uint32(hop.NATPortRangeTo),
				UnderlayRoute: &hop.TargetAddress,
			},
		}, dpdkerrors.Ignore(dpdkerrors.NOT_FOUND),
		); err != nil {
			return fmt.Errorf("error deleting nat route: %w", err)
		}
		return nil
	}

	if _, err := c.dpdk.DeleteRoute(
		ctx,
		uint32(vni),
		&dest.Prefix,
		dpdkerrors.Ignore(dpdkerrors.NO_VNI, dpdkerrors.ROUTE_NOT_FOUND),
	); err != nil {
		return fmt.Errorf("error deleting route: %w", err)
	}
	return nil
}

func (c *MetalnetClient) AddRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	c.log.V(1).Info("AddRoute", "VNI", vni, "dest", dest, "hop", hop)
	var errStrs []string

	isDefaultRoute, err := c.FilterDefaultRoute(AddDefaultRoute, vni, dest, hop)
	if err != nil {
		return fmt.Errorf("error handling default router change: %w", err)
	} else if isDefaultRoute {
		return nil
	}

	if err := c.addLocalRoute(vni, vni, dest, hop); err != nil {
		errStrs = append(errStrs, err.Error())
	}

	if hop.Type == mbproto.NextHopType_STANDARD {
		// the ok flag is ignored because an empty set is returned if the VNI doesn't exist, and the loop below is skipped
		mbPeerVnis, _ := c.metalnetCache.GetPeerVnis(uint32(vni))
		peeredPrefixes, ok := c.metalnetCache.GetPeeredPrefixes(uint32(vni))
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

func (c *MetalnetClient) RemoveRoute(vni mb.VNI, dest mb.Destination, hop mb.NextHop) error {
	c.log.V(1).Info("RemoveRoute", "VNI", vni, "dest", dest, "hop", hop)
	var errStrs []string

	isDefaultRoute, err := c.FilterDefaultRoute(RemoveDefaultRoute, vni, dest, hop)
	if err != nil {
		return fmt.Errorf("error handling default router change: %w", err)
	} else if isDefaultRoute {
		return nil
	}

	if err := c.removeLocalRoute(vni, vni, dest, hop); err != nil {
		errStrs = append(errStrs, err.Error())
	}

	mbPeerVnis, _ := c.metalnetCache.GetPeerVnis(uint32(vni))

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

func (c *MetalnetClient) CleanupNotPeeredRoutes(vni uint32) error {
	ctx := context.TODO()

	routes, err := c.dpdk.ListRoutes(ctx, vni)
	if err != nil {
		return fmt.Errorf("error listing dpdk routes for vni %d: %w", vni, err)
	}

	set, ok := c.metalnetCache.GetPeerVnis(uint32(vni))

	// loop over all routes and delete the ones that are not peered
	for _, route := range routes.Items {
		// only delete route if it is not the local vni and not peered
		if route.Spec.NextHop.VNI != vni && (ok && !set.Has(route.Spec.NextHop.VNI)) {
			if _, err := c.dpdk.DeleteRoute(
				ctx,
				vni,
				route.Spec.Prefix,
				dpdkerrors.Ignore(dpdkerrors.NO_VNI, dpdkerrors.ROUTE_NOT_FOUND),
			); err != nil {
				return fmt.Errorf("error deleting route: %w", err)
			}
		}
	}

	return nil
}

func (c *MetalnetClient) SetDefaultRouterAddress(address netip.Addr) {
	c.DefaultRouterAddress.RouterAddress = address
	c.DefaultRouterAddress.SetBySubsciption = true
}

func (c *MetalnetClient) handleDefaultRouterChange(operation DefaultRouteOperation) error {

	defaultRoutePrefix := netip.MustParsePrefix("0.0.0.0/0")
	ctx := context.TODO()

	existingVNIs := c.mbInstance.GetSubscribedVnis()

	for _, vni := range existingVNIs {
		if uint32(vni) == c.DefaultRouterAddress.PublicVNI {
			continue
		}

		if operation == RemoveDefaultRoute {
			if _, err := c.dpdk.DeleteRoute(
				ctx,
				uint32(vni),
				&defaultRoutePrefix,
				dpdkerrors.Ignore(dpdkerrors.NO_VNI, dpdkerrors.ROUTE_NOT_FOUND),
			); err != nil {
				return fmt.Errorf("error deleting default route: %w", err)
			}
		}

		if operation == AddDefaultRoute {
			if _, err := c.dpdk.CreateRoute(ctx, &dpdk.Route{
				RouteMeta: dpdk.RouteMeta{
					VNI: uint32(vni),
				},
				Spec: dpdk.RouteSpec{
					Prefix: &defaultRoutePrefix,
					NextHop: &dpdk.RouteNextHop{
						VNI: uint32(vni),
						IP:  &c.DefaultRouterAddress.RouterAddress,
					},
				},
			},
				dpdkerrors.Ignore(dpdkerrors.ROUTE_EXISTS),
			); err != nil {
				return fmt.Errorf("error creating default route: %w", err)
			}

		}

	}

	return nil
}

func (c *MetalnetClient) FilterDefaultRoute(operation DefaultRouteOperation, vni mb.VNI, dest mb.Destination, hop mb.NextHop) (bool, error) {
	if uint32(vni) != c.DefaultRouterAddress.PublicVNI {
		return false, nil
	}

	if dest.Prefix.String() != "0.0.0.0/0" {
		return false, nil
	}

	c.DefaultRouterAddress.RWMutex.Lock()
	defer c.DefaultRouterAddress.RWMutex.Unlock()

	if operation == AddDefaultRoute {
		c.SetDefaultRouterAddress(hop.TargetAddress)
	} else {
		c.SetDefaultRouterAddress(netip.Addr{})
	}

	if err := c.handleDefaultRouterChange(operation); err != nil {
		return true, fmt.Errorf("error handling default router change: %w, operation %d", err, operation)
	}

	return true, nil
}
