// Copyright 2022 IronCore authors
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

package client

import (
	"context"

	metalnetv1alpha1 "github.com/ironcore-dev/metalnet/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	NetworkInterfaceNetworkRefNameField = ".spec.networkRef.name"
	LoadBalancerNetworkRefNameField     = ".spec.networkRef.name"
)

func SetupNetworkInterfaceNetworkRefNameFieldIndexer(ctx context.Context, indexer client.FieldIndexer) error {
	return indexer.IndexField(ctx, &metalnetv1alpha1.NetworkInterface{}, NetworkInterfaceNetworkRefNameField, func(obj client.Object) []string {
		nic := obj.(*metalnetv1alpha1.NetworkInterface)
		return []string{nic.Spec.NetworkRef.Name}
	})
}

func SetupLoadBalancerNetworkRefNameFieldIndexer(ctx context.Context, indexer client.FieldIndexer) error {
	return indexer.IndexField(ctx, &metalnetv1alpha1.LoadBalancer{}, LoadBalancerNetworkRefNameField, func(obj client.Object) []string {
		lb := obj.(*metalnetv1alpha1.LoadBalancer)
		return []string{lb.Spec.NetworkRef.Name}
	})
}
