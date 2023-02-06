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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	"github.com/onmetal/controller-utils/clientutils"
	metalnetv1alpha1 "github.com/onmetal/metalnet/api/v1alpha1"
	"github.com/onmetal/metalnet/dpdk"
	"github.com/onmetal/metalnet/dpdkmetalbond"
	"github.com/onmetal/metalnet/metalbond"
)

const (
	loadBalancerFinalizer = "networking.metalnet.onmetal.de/loadBalancer"
)

// LoadBalancerReconciler reconciles a LoadBalancer object
type LoadBalancerReconciler struct {
	client.Client
	record.EventRecorder
	Scheme *runtime.Scheme

	DPDK      dpdk.Client
	LBServer  dpdkmetalbond.LBServerAccess
	Metalbond metalbond.Client
	NodeName  string
	PublicVNI int
}

//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=loadbalancers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=loadbalancers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=loadbalancers/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *LoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	lb := &metalnetv1alpha1.LoadBalancer{}

	if err := r.Get(ctx, req.NamespacedName, lb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	//if nodeName := lb.Spec.NodeName; nodeName == nil || *nodeName != r.NodeName {
	log.V(1).Info("#####Reconcile", "Spec", lb.Spec, "NodeName", r.NodeName)
	if lb.Spec.NodeName != nil && *lb.Spec.NodeName == r.NodeName {
		return r.reconcileExists(ctx, log, lb)
	} else {
		log.V(1).Info("LoadBalancer is not assigned to this node", "NodeName", lb.Spec.NodeName)
		return ctrl.Result{}, nil
	}
}

func (r *LoadBalancerReconciler) reconcileExists(ctx context.Context, log logr.Logger, lb *metalnetv1alpha1.LoadBalancer) (ctrl.Result, error) {
	if !lb.DeletionTimestamp.IsZero() {
		return r.delete(ctx, log, lb)
	}

	return r.reconcile(ctx, log, lb)
}

func (r *LoadBalancerReconciler) delete(ctx context.Context, log logr.Logger, lb *metalnetv1alpha1.LoadBalancer) (ctrl.Result, error) {
	log.V(1).Info("Delete")

	if !controllerutil.ContainsFinalizer(lb, loadBalancerFinalizer) {
		log.V(1).Info("No finalizer present, nothing to do")
		return ctrl.Result{}, nil
	}
	log.V(1).Info("Finalizer present, cleaning up")

	log.V(1).Info("Getting dpdk loadbalancer")
	dpdkLoadBalancer, err := r.DPDK.GetLoadBalancer(ctx, lb.UID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting dpdk loadbalancer: %w", err)
	}

	vni := dpdkLoadBalancer.Spec.VNI
	underlayRoute := dpdkLoadBalancer.Status.UnderlayRoute
	log.V(1).Info("Got dpdk LoadBalancer", "VNI", vni, "UnderlayRoute", underlayRoute)

	if err != nil {
		if !dpdk.IsStatusErrorCode(err, dpdk.GET_LB_ID_ERR) {
			return ctrl.Result{}, fmt.Errorf("error getting dpdk loadbalancer: %w", err)
		}

		if err := r.LBServer.RemoveLoadBalancerServer(vni, lb.UID); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting dpdk loadbalancer from internal cache: %w", err)
		}

		log.V(1).Info("No dpdk loadbalancer, removing finalizer")
		if err := clientutils.PatchRemoveFinalizer(ctx, r.Client, lb, loadBalancerFinalizer); err != nil {
			return ctrl.Result{}, fmt.Errorf("error removing finalizer: %w", err)
		}
		log.V(1).Info("Removed finalizer")

		return ctrl.Result{}, nil
	}

	log.V(1).Info("Deleting LoadBalancer")
	if err := r.deleteLoadBalancer(ctx, log, lb, vni, underlayRoute); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting underlay route: %w", err)
	}
	log.V(1).Info("Deleted Loadbalancer")
	if err := r.LBServer.RemoveLoadBalancerServer(vni, lb.UID); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting dpdk loadbalancer from internal cache: %w", err)
	}

	log.V(1).Info("Removing finalizer")
	if err := clientutils.PatchRemoveFinalizer(ctx, r.Client, lb, loadBalancerFinalizer); err != nil {
		return ctrl.Result{}, fmt.Errorf("error removing finalizer: %w", err)
	}
	log.V(1).Info("Removed finalizer")
	return ctrl.Result{}, nil
}

func (r *LoadBalancerReconciler) deleteLoadBalancer(
	ctx context.Context,
	log logr.Logger,
	lb *metalnetv1alpha1.LoadBalancer,
	vni uint32,
	underlayRoute netip.Addr,
) error {
	log.V(1).Info("Removing loadbalancer route if exists")
	if err := r.removeLoadBalancerRouteIfExists(ctx, lb.Spec.IP.Addr, underlayRoute); err != nil {
		return fmt.Errorf("[Loadbalancer IP %s] %w", lb.Spec.IP.Addr, err)
	}
	log.V(1).Info("Removed loadbalancer route if existed")

	log.V(1).Info("Deleting dpdk loadbalancer if exists")
	if err := r.DPDK.DeleteLoadBalancer(ctx, lb.UID); dpdk.IgnoreStatusErrorCode(err, dpdk.DEL_LB_ID_ERR) != nil {
		return fmt.Errorf("error deleting loadbalancer: %w", err)
	}

	log.V(1).Info("Deleted dpdk loadbalancer if existed")

	return nil
}

func (r *LoadBalancerReconciler) removeLoadBalancerRouteIfExists(ctx context.Context, ip, underlayRoute netip.Addr) error {
	if err := r.Metalbond.RemoveRoute(ctx, metalbond.VNI(r.PublicVNI), metalbond.Destination{
		Prefix: NetIPAddrPrefix(ip),
	}, metalbond.NextHop{
		TargetVNI:     0,
		TargetAddress: underlayRoute,
	}); metalbond.IgnoreNextHopNotFoundError(err) != nil {
		return fmt.Errorf("error removing loadbalancer route: %w", err)
	}
	return nil
}

func (r *LoadBalancerReconciler) addLoadBalancerRouteIfNotExists(ctx context.Context, ip, underlayRoute netip.Addr) error {
	if err := r.Metalbond.AddRoute(ctx, metalbond.VNI(r.PublicVNI), metalbond.Destination{
		Prefix: NetIPAddrPrefix(ip),
	}, metalbond.NextHop{
		TargetVNI:     0,
		TargetAddress: underlayRoute,
	}); metalbond.IgnoreNextHopAlreadyExistsError(err) != nil {
		return fmt.Errorf("error adding loadbalancer route: %w", err)
	}
	return nil
}

func (r *LoadBalancerReconciler) patchStatus(
	ctx context.Context,
	lb *metalnetv1alpha1.LoadBalancer,
	mutate func(),
) error {
	base := lb.DeepCopy()

	mutate()

	if err := r.Status().Patch(ctx, lb, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("error patching status: %w", err)
	}
	return nil
}

func (r *LoadBalancerReconciler) reconcile(ctx context.Context, log logr.Logger, lb *metalnetv1alpha1.LoadBalancer) (ctrl.Result, error) {
	log.V(1).Info("Reconcile")

	log.V(1).Info("Ensuring finalizer")
	modified, err := clientutils.PatchEnsureFinalizer(ctx, r.Client, lb, loadBalancerFinalizer)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error ensuring finalizer: %w", err)
	}
	if modified {
		log.V(1).Info("Added finalizer")
		return ctrl.Result{Requeue: true}, nil
	}
	log.V(1).Info("Ensured finalizer")

	network := &metalnetv1alpha1.Network{}
	networkKey := client.ObjectKey{Namespace: lb.Namespace, Name: lb.Spec.NetworkRef.Name}
	log.V(1).Info("Getting network", "NetworkKey", networkKey)
	if err := r.Get(ctx, networkKey, network); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting network %s: %w", networkKey, err)
		}

		r.Eventf(lb, corev1.EventTypeWarning, "NetworkNotFound", "Network %s could not be found", networkKey.Name)
		if err := r.patchStatus(ctx, lb, func() {
			lb.Status = metalnetv1alpha1.LoadBalancerStatus{
				State: metalnetv1alpha1.LoadBalancerStatePending,
			}
		}); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	vni := uint32(network.Spec.ID)
	log.V(1).Info("Got network", "NetworkKey", networkKey, "VNI", vni)

	log.V(1).Info("Applying loadbalancer")
	underlayRoute, err := r.applyLoadBalancer(ctx, log, lb, vni)
	if err != nil {
		if err := r.patchStatus(ctx, lb, func() {
			lb.Status = metalnetv1alpha1.LoadBalancerStatus{
				State: metalnetv1alpha1.LoadBalancerStateError,
			}
		}); err != nil {
			log.Error(err, "Error patching loadbalancer status")
		}
		return ctrl.Result{}, fmt.Errorf("error applying loadbalancer: %w", err)
	}
	log.V(1).Info("Applied loadbalancer", "UnderlayRoute", underlayRoute)

	log.V(1).Info("Subscribing to metalbond if not subscribed")
	if err := r.subscribeIfNotSubscribed(ctx, vni); err != nil {
		return ctrl.Result{}, err
	}
	log.V(1).Info("Subscribed to metalbond if not subscribed")

	log.V(1).Info("Adding loadbalancer route if not exists")
	if err := r.addLoadBalancerRouteIfNotExists(ctx, lb.Spec.IP.Addr, underlayRoute); err != nil {
		return ctrl.Result{}, err
	}
	log.V(1).Info("Added loadbalancer route if not existed")

	log.V(1).Info("Patching status")
	if err := r.patchStatus(ctx, lb, func() {
		lb.Status.State = metalnetv1alpha1.LoadBalancerStateReady
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching status: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *LoadBalancerReconciler) applyLoadBalancer(ctx context.Context, log logr.Logger, lb *metalnetv1alpha1.LoadBalancer, vni uint32) (netip.Addr, error) {
	log.V(1).Info("Getting dpdk loadbalancer")
	lbalancer, err := r.DPDK.GetLoadBalancer(ctx, lb.UID)
	if err != nil {
		if !dpdk.IsStatusErrorCode(err, dpdk.GET_LB_ID_ERR) {
			return netip.Addr{}, fmt.Errorf("error getting dpdk loadbalancer: %w", err)
		}

		var ports []dpdk.DpLoadBalancerPort
		for _, LBPort := range lb.Spec.Ports {
			port := dpdk.DpLoadBalancerPort{
				Port:     uint32(LBPort.Port),
				Protocol: LBPort.Protocol,
			}
			ports = append(ports, port)
		}

		log.V(1).Info("DPDK loadbalancer does not yet exist, creating it")

		lbalancer, err := r.DPDK.CreateLoadBalancer(ctx, &dpdk.DpLoadBalancer{
			DpLoadBalancerMetadata: dpdk.DpLoadBalancerMetadata{UID: lb.UID},
			Spec: dpdk.DpLoadBalancerSpec{
				VNI:                     vni,
				LoadBalancerIPv4Address: lb.Spec.IP.Addr,
				Ports:                   ports,
			},
		})
		if err != nil {
			return netip.Addr{}, fmt.Errorf("error creating dpdk loadbalancer: %w", err)
		}
		if err := r.LBServer.AddLoadBalancerServer(vni, lb.UID); err != nil {
			return netip.Addr{}, fmt.Errorf("error adding dpdk loadbalancer to internal cache: %w", err)
		}

		return lbalancer.Status.UnderlayRoute, nil
	}

	log.V(1).Info("DPDK loadbalancer exists")
	if err := r.LBServer.AddLoadBalancerServer(vni, lb.UID); err != nil {
		return netip.Addr{}, fmt.Errorf("error adding dpdk loadbalancer to internal cache: %w", err)
	}

	return lbalancer.Status.UnderlayRoute, nil
}

func (r *LoadBalancerReconciler) subscribeIfNotSubscribed(ctx context.Context, vni uint32) error {
	if err := r.Metalbond.Subscribe(ctx, metalbond.VNI(vni)); metalbond.IgnoreAlreadySubscribedToVNIError(err) != nil {
		return fmt.Errorf("error subscribing to vni: %w", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metalnetv1alpha1.LoadBalancer{}).
		Complete(r)
}
