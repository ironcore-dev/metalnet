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
	"fmt"
	"sync"

	"github.com/onmetal/metalbond"
)

type RouteCache struct {
	rwmtx  sync.RWMutex
	routes map[VNI]map[metalbond.Destination]map[metalbond.NextHop]bool
}

func NewRouteCache() RouteCache {
	return RouteCache{
		routes: make(map[VNI]map[metalbond.Destination]map[metalbond.NextHop]bool),
	}
}

func (rt *RouteCache) GetVNIs() []VNI {
	rt.rwmtx.RLock()
	defer rt.rwmtx.RUnlock()

	vnis := []VNI{}
	for k := range rt.routes {
		vnis = append(vnis, k)
	}
	return vnis
}

func (rt *RouteCache) GetDestinationsByVNI(vni VNI) map[metalbond.Destination][]metalbond.NextHop {
	rt.rwmtx.RLock()
	defer rt.rwmtx.RUnlock()

	ret := make(map[metalbond.Destination][]metalbond.NextHop)

	if _, exists := rt.routes[vni]; !exists {
		return ret
	}

	for dest, nhm := range rt.routes[vni] {
		nhs := []metalbond.NextHop{}

		for nh := range nhm {
			nhs = append(nhs, nh)
		}

		ret[dest] = nhs
	}

	return ret
}

func (rt *RouteCache) GetNextHopsByDestination(vni VNI, dest metalbond.Destination) []metalbond.NextHop {
	rt.rwmtx.RLock()
	defer rt.rwmtx.RUnlock()

	nh := []metalbond.NextHop{}

	if _, exists := rt.routes[vni]; !exists {
		return nh
	}

	if _, exists := rt.routes[vni][dest]; !exists {
		return nh
	}

	for k := range rt.routes[vni][dest] {
		nh = append(nh, k)
	}

	return nh
}

func (rt *RouteCache) RemoveNextHop(vni metalbond.VNI, dest metalbond.Destination, nh metalbond.NextHop) error {
	rt.rwmtx.Lock()
	defer rt.rwmtx.Unlock()

	if rt.routes == nil {
		rt.routes = make(map[metalbond.VNI]map[metalbond.Destination]map[metalbond.NextHop]bool)
	}

	if _, exists := rt.routes[vni]; !exists {
		return fmt.Errorf("VNI does not exist")
	}

	if _, exists := rt.routes[vni][dest]; !exists {
		return fmt.Errorf("Destination does not exist")
	}

	if _, exists := rt.routes[vni][dest][nh]; !exists {
		return fmt.Errorf("Nexthop does not exist")
	}

	delete(rt.routes[vni][dest], nh)

	if len(rt.routes[vni][dest]) == 0 {
		delete(rt.routes[vni], dest)
	}

	if len(rt.routes[vni]) == 0 {
		delete(rt.routes, vni)
	}

	return nil
}

func (rt *RouteCache) AddNextHop(vni metalbond.VNI, dest metalbond.Destination, nh metalbond.NextHop) error {
	rt.rwmtx.Lock()
	defer rt.rwmtx.Unlock()

	if _, exists := rt.routes[vni]; !exists {
		rt.routes[vni] = make(map[metalbond.Destination]map[metalbond.NextHop]bool)
	}

	if _, exists := rt.routes[vni][dest]; !exists {
		rt.routes[vni][dest] = make(map[metalbond.NextHop]bool)
	}

	if _, exists := rt.routes[vni][dest][nh]; exists {
		return fmt.Errorf("Nexthop already exists")
	}

	rt.routes[vni][dest][nh] = true

	return nil
}

func (rt *RouteCache) NextHopExists(vni metalbond.VNI, dest metalbond.Destination, nh metalbond.NextHop) bool {
	rt.rwmtx.Lock()
	defer rt.rwmtx.Unlock()

	if _, exists := rt.routes[vni]; !exists {
		rt.routes[vni] = make(map[metalbond.Destination]map[metalbond.NextHop]bool)
	}

	if _, exists := rt.routes[vni][dest]; !exists {
		rt.routes[vni][dest] = make(map[metalbond.NextHop]bool)
	}

	if _, exists := rt.routes[vni][dest][nh]; exists {
		return true
	}

	return false
}
