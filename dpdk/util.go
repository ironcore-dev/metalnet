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
)

type RouteSpecSet struct {
	set map[string]struct{}
}

func RouteSpecToString(c RouteSpec) (string, error) {
	bytes, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to convert custom type to string: %v", err)
	}
	return string(bytes), nil
}

func stringToRouteSpec(s string) (RouteSpec, error) {
	var c RouteSpec
	err := json.Unmarshal([]byte(s), &c)
	if err != nil {
		return RouteSpec{}, fmt.Errorf("failed to convert string to custom type: %v", err)
	}
	return c, nil
}

func NewRouteSpecSet() *RouteSpecSet {
	return &RouteSpecSet{set: make(map[string]struct{})}
}

func (cts *RouteSpecSet) Insert(c RouteSpec) error {
	s, err := RouteSpecToString(c)
	if err != nil {
		return err
	}
	cts.set[s] = struct{}{}
	return nil
}

func (cts *RouteSpecSet) Delete(c RouteSpec) error {
	s, err := RouteSpecToString(c)
	if err != nil {
		return err
	}
	delete(cts.set, s)
	return nil
}

func (cts *RouteSpecSet) Has(c RouteSpec) (bool, error) {
	s, err := RouteSpecToString(c)
	if err != nil {
		return false, err
	}
	_, exists := cts.set[s]
	return exists, nil
}

func (cts *RouteSpecSet) List() ([]RouteSpec, error) {
	result := make([]RouteSpec, 0, len(cts.set))
	for s := range cts.set {
		c, err := stringToRouteSpec(s)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}
