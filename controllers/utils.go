// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	metalnetv1alpha1 "github.com/ironcore-dev/metalnet/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func filterByNodePredicate(nodeName string) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isResourceAssignedToNode(e.Object, nodeName)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isResourceAssignedToNode(e.ObjectNew, nodeName)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isResourceAssignedToNode(e.Object, nodeName)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return isResourceAssignedToNode(e.Object, nodeName)
		},
	}
}

func isResourceAssignedToNode(obj client.Object, nodeName string) bool {
	if obj == nil {
		return false
	}

	switch resource := obj.(type) {
	case *metalnetv1alpha1.NetworkInterface:
		return isNetworkInterfaceAssignedToNode(resource, nodeName)
	case *metalnetv1alpha1.LoadBalancer:
		return isLoadBalancerAssignedToNode(resource, nodeName)
	default:
		return false
	}
}

func isNetworkInterfaceAssignedToNode(nic *metalnetv1alpha1.NetworkInterface, nodeName string) bool {
	return nic.Spec.NodeName != nil && *nic.Spec.NodeName == nodeName
}

func isLoadBalancerAssignedToNode(lb *metalnetv1alpha1.LoadBalancer, nodeName string) bool {
	return lb.Spec.NodeName != nil && *lb.Spec.NodeName == nodeName
}
