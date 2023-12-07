// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

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
