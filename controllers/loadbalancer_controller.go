// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

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
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	"github.com/ironcore-dev/controller-utils/clientutils"
	dpdk "github.com/ironcore-dev/dpservice-go/api"
	dpdkclient "github.com/ironcore-dev/dpservice-go/client"
	dpdkerrors "github.com/ironcore-dev/dpservice-go/errors"
	dpdkproto "github.com/ironcore-dev/dpservice-go/proto"
	metalnetv1alpha1 "github.com/ironcore-dev/metalnet/api/v1alpha1"
	metalnetclient "github.com/ironcore-dev/metalnet/client"
	"github.com/ironcore-dev/metalnet/internal"
	"github.com/ironcore-dev/metalnet/metalbond"
)

const (
	loadBalancerFinalizer = "networking.metalnet.ironcore.dev/loadBalancer"
)

// LoadBalancerReconciler reconciles a LoadBalancer object
type LoadBalancerReconciler struct {
	client.Client
	record.EventRecorder
	Scheme *runtime.Scheme

	DPDK          dpdkclient.Client
	MetalnetCache *internal.MetalnetCache
	RouteUtil     metalbond.RouteUtil

	NodeName          string
	PublicVNI         int
	EnableIPv6Support bool
}

//+kubebuilder:rbac:groups=networking.metalnet.ironcore.dev,resources=loadbalancers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.metalnet.ironcore.dev,resources=loadbalancers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.metalnet.ironcore.dev,resources=loadbalancers/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *LoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	lb := &metalnetv1alpha1.LoadBalancer{}

	if err := r.Get(ctx, req.NamespacedName, lb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if nodeName := lb.Spec.NodeName; nodeName == nil || *nodeName != r.NodeName {
		log.V(1).Info("LoadBalancer is not assigned to this node", "NodeName", lb.Spec.NodeName)
		return ctrl.Result{}, nil
	}

	return r.reconcileExists(ctx, log, lb)
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
	dpdkLoadBalancer, err := r.DPDK.GetLoadBalancer(ctx, string(lb.UID))
	ip := lb.Spec.IP.Addr.String()
	if err != nil {
		if !dpdkerrors.IsStatusErrorCode(err, dpdkerrors.NOT_FOUND) {
			return ctrl.Result{}, fmt.Errorf("error getting dpdk loadbalancer: %w", err)
		}
		log.V(1).Info("Remove LoadBalancer server", "ip", ip)
		if err := r.MetalnetCache.RemoveLoadBalancerServer(ip, lb.UID); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting dpdk loadbalancer from internal cache: %w", err)
		}
		log.V(1).Info("No dpdk loadbalancer, removing finalizer")
		if err := clientutils.PatchRemoveFinalizer(ctx, r.Client, lb, loadBalancerFinalizer); err != nil {
			return ctrl.Result{}, fmt.Errorf("error removing finalizer: %w", err)
		}
		log.V(1).Info("Removed finalizer")

		return ctrl.Result{}, nil
	}

	vni := dpdkLoadBalancer.Spec.VNI
	underlayRoute := dpdkLoadBalancer.Spec.UnderlayRoute
	log.V(1).Info("Got dpdk LoadBalancer", "VNI", vni, "UnderlayRoute", underlayRoute)

	log.V(1).Info("Deleting LoadBalancer")
	if err := r.deleteLoadBalancer(ctx, log, lb, vni, *underlayRoute); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting underlay route: %w", err)
	}
	log.V(1).Info("Deleted Loadbalancer")
	log.V(1).Info("Remove LoadBalancer server", "vni", vni, "ip", ip)
	if err := r.MetalnetCache.RemoveLoadBalancerServer(ip, lb.UID); err != nil {
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
	if err := r.removeLoadBalancerRouteIfExists(ctx, lb, underlayRoute, vni); err != nil {
		return fmt.Errorf("[Loadbalancer IP %s] %w", lb.Spec.IP.Addr, err)
	}
	log.V(1).Info("Removed loadbalancer route if existed")

	log.V(1).Info("Deleting dpdk loadbalancer if exists")
	if _, err := r.DPDK.DeleteLoadBalancer(
		ctx,
		string(lb.UID),
		dpdkerrors.Ignore(dpdkerrors.NOT_FOUND),
	); err != nil {
		return fmt.Errorf("error deleting loadbalancer: %w", err)
	}

	log.V(1).Info("Deleted dpdk loadbalancer if existed")

	return nil
}

func (r *LoadBalancerReconciler) removeLoadBalancerRouteIfExists(ctx context.Context, lb *metalnetv1alpha1.LoadBalancer, underlayRoute netip.Addr, vni uint32) error {
	var localVni uint32

	if lb.Spec.LBtype == metalnetv1alpha1.LoadBalancerTypeInternal {
		localVni = vni
	} else {
		localVni = uint32(r.PublicVNI)
	}
	if err := r.RouteUtil.WithdrawRoute(ctx, metalbond.VNI(localVni), metalbond.Destination{
		Prefix: NetIPAddrPrefix(lb.Spec.IP.Addr),
	}, metalbond.NextHop{
		TargetVNI:     0,
		TargetAddress: underlayRoute,
	}); metalbond.IgnoreNextHopNotFoundError(err) != nil {
		return fmt.Errorf("error removing loadbalancer route: %w", err)
	}
	return nil
}

func (r *LoadBalancerReconciler) addLoadBalancerRouteIfNotExists(ctx context.Context, lb *metalnetv1alpha1.LoadBalancer, underlayRoute netip.Addr, vni uint32) error {
	var localVni uint32

	if lb.Spec.LBtype == metalnetv1alpha1.LoadBalancerTypeInternal {
		localVni = vni
	} else {
		localVni = uint32(r.PublicVNI)
	}
	if err := r.RouteUtil.AnnounceRoute(ctx, metalbond.VNI(localVni), metalbond.Destination{
		Prefix: NetIPAddrPrefix(lb.Spec.IP.Addr),
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

	if !r.EnableIPv6Support && lb.Spec.IP.Addr.Is6() {
		if err := r.patchStatus(ctx, lb, func() {
			lb.Status = metalnetv1alpha1.LoadBalancerStatus{
				State: metalnetv1alpha1.LoadBalancerStateError,
			}
		}); err != nil {
			log.Error(err, "Error patching loadbalancer status")
		}
		return ctrl.Result{}, fmt.Errorf("ipv6 flag not enabled but ipv6 address set on loadbalancer")
	}

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
	ip := lb.Spec.IP.Addr.String()
	lbalancer, err := r.DPDK.GetLoadBalancer(ctx, string(lb.UID))
	if err != nil {
		if !dpdkerrors.IsStatusErrorCode(err, dpdkerrors.NOT_FOUND) {
			return netip.Addr{}, fmt.Errorf("error getting dpdk loadbalancer: %w", err)
		}

		var ports []dpdk.LBPort
		for _, LBPort := range lb.Spec.Ports {
			port := dpdk.LBPort{
				Port:     uint32(LBPort.Port),
				Protocol: uint32(dpdkproto.Protocol_value[LBPort.Protocol]),
			}
			ports = append(ports, port)
		}

		log.V(1).Info("DPDK loadbalancer does not yet exist, creating it")

		lbalancer, err := r.DPDK.CreateLoadBalancer(ctx, &dpdk.LoadBalancer{
			LoadBalancerMeta: dpdk.LoadBalancerMeta{ID: string(lb.UID)},
			Spec: dpdk.LoadBalancerSpec{
				VNI:     vni,
				LbVipIP: &lb.Spec.IP.Addr,
				Lbports: ports,
			},
		})
		if err != nil {
			return netip.Addr{}, fmt.Errorf("error creating dpdk loadbalancer: %w", err)
		}
		log.V(1).Info("Adding loadbalancer server", "vni", vni, "ip", ip)
		if err := r.MetalnetCache.AddLoadBalancerServer(vni, ip, lb.UID); err != nil {
			return netip.Addr{}, fmt.Errorf("error adding dpdk loadbalancer to internal cache: %w", err)
		}
		log.V(1).Info("Adding loadbalancer route if not exists")
		if err := r.addLoadBalancerRouteIfNotExists(ctx, lb, *lbalancer.Spec.UnderlayRoute, vni); err != nil {
			return netip.Addr{}, err
		}
		log.V(1).Info("Added loadbalancer route if not existed")
		return *lbalancer.Spec.UnderlayRoute, nil
	}

	log.V(1).Info("DPDK loadbalancer exists")
	log.V(1).Info("Adding loadbalancer server", "vni", vni, "ip", ip)
	if err := r.MetalnetCache.AddLoadBalancerServer(vni, ip, lb.UID); err != nil {
		return netip.Addr{}, fmt.Errorf("error adding dpdk loadbalancer to internal cache: %w", err)
	}
	log.V(1).Info("Adding loadbalancer route if not exists")
	if err := r.addLoadBalancerRouteIfNotExists(ctx, lb, *lbalancer.Spec.UnderlayRoute, vni); err != nil {
		return netip.Addr{}, err
	}
	log.V(1).Info("Added loadbalancer route if not existed")
	return *lbalancer.Spec.UnderlayRoute, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager, metalnetCache cache.Cache) error {
	log := ctrl.Log.WithName("loadbalancer").WithName("setup")
	ctx := ctrl.LoggerInto(context.TODO(), log)

	return ctrl.NewControllerManagedBy(mgr).
		For(&metalnetv1alpha1.LoadBalancer{}).
		WatchesRawSource(
			source.Kind(metalnetCache, &metalnetv1alpha1.Network{}),
			r.enqueueLoadBalancersReferencingNetwork(ctx, log),
		).
		Complete(r)
}

func (r *LoadBalancerReconciler) enqueueLoadBalancersReferencingNetwork(ctx context.Context, log logr.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		network := obj.(*metalnetv1alpha1.Network)
		lbList := &metalnetv1alpha1.LoadBalancerList{}
		if err := r.List(ctx, lbList,
			client.InNamespace(network.Namespace),
			client.MatchingFields{metalnetclient.LoadBalancerNetworkRefNameField: network.Name},
		); err != nil {
			log.Error(err, "Error listing loadbalancers referencing network", "NetworkKey", client.ObjectKeyFromObject(network))
			return nil
		}

		reqs := make([]ctrl.Request, len(lbList.Items))
		for i, lb := range lbList.Items {
			reqs[i] = ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&lb)}
		}
		return reqs
	})
}
