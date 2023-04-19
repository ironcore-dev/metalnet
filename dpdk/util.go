// Copyright 2023 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dpdk

import (
	"encoding/json"
	"fmt"

	mb "github.com/onmetal/metalbond"
)

type MBRouteSet struct {
	set map[string]struct{}
}

type MBRoute struct {
	Dest    mb.Destination
	NextHop mb.NextHop
}

func MBRouteToString(c MBRoute) (string, error) {
	bytes, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to convert custom type to string: %v", err)
	}
	return string(bytes), nil
}

func stringToMBRoute(s string) (MBRoute, error) {
	var c MBRoute
	err := json.Unmarshal([]byte(s), &c)
	if err != nil {
		return MBRoute{}, fmt.Errorf("failed to convert string to custom type: %v", err)
	}
	return c, nil
}

func NewMBRouteSet() *MBRouteSet {
	return &MBRouteSet{set: make(map[string]struct{})}
}

func (cts *MBRouteSet) Insert(c MBRoute) error {
	s, err := MBRouteToString(c)
	if err != nil {
		return err
	}
	cts.set[s] = struct{}{}
	return nil
}

func (cts *MBRouteSet) Delete(c MBRoute) error {
	s, err := MBRouteToString(c)
	if err != nil {
		return err
	}
	delete(cts.set, s)
	return nil
}

func (cts *MBRouteSet) Has(c MBRoute) (bool, error) {
	s, err := MBRouteToString(c)
	if err != nil {
		return false, err
	}
	_, exists := cts.set[s]
	return exists, nil
}

func (cts *MBRouteSet) List() ([]MBRoute, error) {
	result := make([]MBRoute, 0, len(cts.set))
	for s := range cts.set {
		c, err := stringToMBRoute(s)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}
