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

package internal

import (
	"net/netip"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type MetalnetCache struct {
	lbServerMap    map[uint32]map[string]types.UID
	peeredPrefixes map[uint32]map[uint32][]netip.Prefix
	peeredVnis     map[uint32]sets.Set[uint32]
	mtxPeeredVnis  sync.RWMutex
	log            *logr.Logger
}

func NewMetalnetCache(log *logr.Logger) *MetalnetCache {
	return &MetalnetCache{
		lbServerMap:    make(map[uint32]map[string]types.UID),
		peeredPrefixes: make(map[uint32]map[uint32][]netip.Prefix),
		peeredVnis:     make(map[uint32]sets.Set[uint32]),
		log:            log,
	}
}

func (c *MetalnetCache) SetPeeredPrefixes(vni uint32, peeredPrefixes map[uint32][]netip.Prefix) {
	c.peeredPrefixes[vni] = peeredPrefixes
}

func (c *MetalnetCache) GetPeeredPrefixes(vni uint32) (map[uint32][]netip.Prefix, bool) {
	prefixes, ok := c.peeredPrefixes[vni]
	if !ok {
		return nil, false
	}

	// Create a new map and avoid modification from outside
	copiedPrefixes := make(map[uint32][]netip.Prefix)
	for k, v := range prefixes {
		copiedPrefixes[k] = append([]netip.Prefix(nil), v...)
	}

	return copiedPrefixes, true
}

func (c *MetalnetCache) IsVniPeered(vni uint32) bool {
	c.mtxPeeredVnis.RLock()
	defer c.mtxPeeredVnis.RUnlock()
	for _, peeredVnis := range c.peeredVnis {
		if peeredVnis.Has(vni) {
			return true
		}
	}
	return false
}

func (c *MetalnetCache) GetPeerVnis(vni uint32) (sets.Set[uint32], bool) {
	c.mtxPeeredVnis.RLock()
	defer c.mtxPeeredVnis.RUnlock()
	vnis, ok := c.peeredVnis[vni]
	if !ok {
		return sets.New[uint32](), false
	}
	return vnis, true
}

func (c *MetalnetCache) AddVniToPeerVnis(vni, peeredVNI uint32) error {
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

func (c *MetalnetCache) RemoveVniFromPeerVnis(vni, peeredVNI uint32) error {
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

func (c *MetalnetCache) AddLoadBalancerServer(vni uint32, ip string, uid types.UID) error {
	if _, exists := c.lbServerMap[vni]; !exists {
		c.lbServerMap[vni] = make(map[string]types.UID)
	}
	c.lbServerMap[vni][ip] = uid
	return nil
}

func (c *MetalnetCache) RemoveLoadBalancerServer(ip string, uid types.UID) error {
	for _, innerMap := range c.lbServerMap {
		for keyIp, value := range innerMap {
			if ip == keyIp && value == uid {
				delete(innerMap, ip)
			}
		}
	}
	return nil
}

func (c *MetalnetCache) GetLoadBalancerServer(vni uint32, ip string) (types.UID, bool) {
	innerMap, exists := c.lbServerMap[vni]
	if !exists {
		return "", false
	}
	uid, exists := innerMap[ip]
	return uid, exists
}
