/*
Copyright 2022 The Metal Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/go-logr/logr"
	"github.com/onmetal/controller-utils/clientutils"
	metalnetv1alpha1 "github.com/onmetal/metalnet/api/v1alpha1"
	"github.com/onmetal/metalnet/dpdk"
	"github.com/onmetal/metalnet/metalbond"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	networkFinalizer = "networking.metalnet.onmetal.de/network"
)

// NetworkReconciler reconciles metalnetv1alpha1.Network.
type NetworkReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	DPDK      dpdk.Client
	Metalbond metalbond.Client

	RouterAddress netip.Addr
	NodeName      string
}

//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=networks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=networks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=networks/finalizers,verbs=update;patch
//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=networkinterfaces,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	network := &metalnetv1alpha1.Network{}
	if err := r.Get(ctx, req.NamespacedName, network); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return r.reconcileExists(ctx, log, network)
}

func (r *NetworkReconciler) reconcileExists(ctx context.Context, log logr.Logger, network *metalnetv1alpha1.Network) (ctrl.Result, error) {
	log = log.WithValues("VNI", network.Spec.ID)
	if !network.DeletionTimestamp.IsZero() {
		return r.delete(ctx, log, network)
	}
	return r.reconcile(ctx, log, network)
}

func (r *NetworkReconciler) delete(ctx context.Context, log logr.Logger, network *metalnetv1alpha1.Network) (ctrl.Result, error) {
	log.V(1).Info("Delete")

	if !controllerutil.ContainsFinalizer(network, r.networkFinalizer()) {
		log.V(1).Info("No finalizer present, nothing to do.")
		return ctrl.Result{}, nil
	}

	log.V(1).Info("Finalizer present, doing cleanup")

	vni := uint32(network.Spec.ID)

	log.V(1).Info("Unsubscribing from metalbond if not subscribed")
	if err := r.unsubscribeIfSubscribed(ctx, vni); err != nil {
		return ctrl.Result{}, err
	}
	log.V(1).Info("Unsubscribed from metalbond if subscribed")

	log.V(1).Info("Deleting default route if exists")
	if err := r.deleteDefaultRouteIfExists(ctx, vni); err != nil {
		return ctrl.Result{}, err
	}
	log.V(1).Info("Deleted default route if existed")

	log.V(1).Info("Cleanup done, removing finalizer")
	if err := clientutils.PatchRemoveFinalizer(ctx, r.Client, network, r.networkFinalizer()); err != nil {
		return ctrl.Result{}, fmt.Errorf("error removing finalizer: %w", err)
	}

	log.V(1).Info("Removed finalizer")
	return ctrl.Result{}, nil
}

func (r *NetworkReconciler) reconcile(ctx context.Context, log logr.Logger, network *metalnetv1alpha1.Network) (ctrl.Result, error) {
	log.V(1).Info("Reconcile")

	log.V(1).Info("Ensuring finalizer")
	modified, err := clientutils.PatchEnsureFinalizer(ctx, r.Client, network, r.networkFinalizer())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error ensuring finalizer: %w", err)
	}
	if modified {
		log.V(1).Info("Added finalizer, requeueing")
		return ctrl.Result{Requeue: true}, nil
	}
	log.V(1).Info("Ensured finalizer")

	vni := uint32(network.Spec.ID)

	log.V(1).Info("Checking existence of the VNI")
	vniAvail, err := r.DPDK.IsVniAvailable(ctx, vni)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !vniAvail {
		log.V(1).Info("VNI doesn't exist in dp-service, unsubscribe from it")
		if err := r.unsubscribeIfSubscribed(ctx, vni); err != nil {
			return ctrl.Result{}, err
		}
		log.V(1).Info("VNI doesn't exist in dp-service, unsubscribed from it")
		return ctrl.Result{}, nil
	}
	log.V(1).Info("Checked existence of the VNI")

	log.V(1).Info("Creating dpdk default route if not exists")
	if err := r.createDefaultRouteIfNotExists(ctx, vni); err != nil {
		return ctrl.Result{}, err
	}
	log.V(1).Info("Created dpdk default route if not existed")

	log.V(1).Info("Subscribing to metalbond if not subscribed")
	if err := r.subscribeIfNotSubscribed(ctx, vni); err != nil {
		return ctrl.Result{}, err
	}
	log.V(1).Info("Subscribed to metalbond if not subscribed")

	return ctrl.Result{}, nil
}

func (r *NetworkReconciler) createDefaultRouteIfNotExists(ctx context.Context, vni uint32) error {
	if _, err := r.DPDK.CreateRoute(ctx, &dpdk.Route{
		RouteMetadata: dpdk.RouteMetadata{
			VNI: vni,
		},
		Spec: dpdk.RouteSpec{
			Prefix: netip.MustParsePrefix("0.0.0.0/0"),
			NextHop: dpdk.RouteNextHop{
				VNI:     vni,
				Address: r.RouterAddress,
			},
		},
	}); dpdk.IgnoreStatusErrorCode(err, dpdk.ADD_ROUTE_FAIL4) != nil {
		return fmt.Errorf("error creating route: %w", err)
	}
	return nil
}

func (r *NetworkReconciler) deleteDefaultRouteIfExists(ctx context.Context, vni uint32) error {
	if err := r.DPDK.DeleteRoute(ctx, &dpdk.Route{
		RouteMetadata: dpdk.RouteMetadata{
			VNI: vni,
		},
		Spec: dpdk.RouteSpec{
			Prefix: netip.MustParsePrefix("0.0.0.0/0"),
			NextHop: dpdk.RouteNextHop{
				VNI:     vni,
				Address: r.RouterAddress,
			},
		},
	}); dpdk.IgnoreStatusErrorCode(err, dpdk.DEL_ROUTE) != nil && dpdk.IgnoreStatusErrorCode(err, dpdk.DEL_IFACE_NOT_FOUND) != nil {
		return fmt.Errorf("error deleting route: %w", err)
	}
	return nil
}

func (r *NetworkReconciler) subscribeIfNotSubscribed(ctx context.Context, vni uint32) error {
	if err := r.Metalbond.Subscribe(ctx, metalbond.VNI(vni)); metalbond.IgnoreAlreadySubscribedToVNIError(err) != nil {
		return fmt.Errorf("error subscribing to vni: %w", err)
	}
	return nil
}

func (r *NetworkReconciler) unsubscribeIfSubscribed(ctx context.Context, vni uint32) error {
	if err := r.Metalbond.Unsubscribe(ctx, metalbond.VNI(vni)); metalbond.IgnoreNotSubscribedToVNIError(err) != nil {
		return fmt.Errorf("error subscribing to vni: %w", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metalnetv1alpha1.Network{}).
		WithEventFilter(predicate.ResourceVersionChangedPredicate{}).
		Watches(
			&source.Kind{Type: &metalnetv1alpha1.NetworkInterface{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForNetworkInterface),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&source.Kind{Type: &metalnetv1alpha1.LoadBalancer{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForLoadBalancer),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *NetworkReconciler) findObjectsForNetworkInterface(obj client.Object) []reconcile.Request {
	networkInterface, ok := obj.(*metalnetv1alpha1.NetworkInterface)
	if !ok {
		return []reconcile.Request{}
	}

	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name:      networkInterface.Spec.NetworkRef.Name,
			Namespace: networkInterface.GetNamespace(),
		},
	}}
}

func (r *NetworkReconciler) networkFinalizer() string {
	return fmt.Sprintf("%s-%s", networkFinalizer, r.NodeName)
}

func (r *NetworkReconciler) findObjectsForLoadBalancer(obj client.Object) []reconcile.Request {
	loadBalancer, ok := obj.(*metalnetv1alpha1.LoadBalancer)
	if !ok {
		return []reconcile.Request{}
	}

	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name:      loadBalancer.Spec.NetworkRef.Name,
			Namespace: loadBalancer.GetNamespace(),
		},
	}}
}
