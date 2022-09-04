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
	"math/rand"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	mb "github.com/onmetal/metalbond"
	dpdkproto "github.com/onmetal/net-dpservice-go/proto"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "github.com/onmetal/metalnet/api/v1alpha1"
)

const (
	NetworkInterfaceFinalizerName = "networking.metalnet.onmetal.de/networkInterface"
	UnderlayRoute                 = "dpdk.metalnet.onmetal.de/underlayRoute"
	DpPciAddr                     = "dpdk.metalnet.onmetal.de/dpPciAddr"
	DpRouteAlreadyAddedError      = 251
)

type NodeDevPCIInfo func(string, int) (map[string]string, error)

// NetworkInterfaceReconciler reconciles a NetworkInterface object
type NetworkInterfaceReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	DPDKClient      dpdkproto.DPDKonmetalClient
	HostName        string
	RouterAddress   string
	MbInstance      *mb.MetalBond
	DeviceAllocator DeviceAllocator
}

//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=networkinterfaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=networkinterfaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=networkinterfaces/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NetworkInterfaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	ni := &networkingv1alpha1.NetworkInterface{}

	if err := r.Get(ctx, req.NamespacedName, ni); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Info("unable to fetch NetworkInterface", "NetworkInterface", req, "Error", err)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	network := &networkingv1alpha1.Network{}
	networkKey := client.ObjectKey{
		Namespace: req.NamespacedName.Namespace,
		Name:      ni.Spec.NetworkRef.Name,
	}
	if err := r.Get(ctx, networkKey, network); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Info("unable to fetch Network", "Network", req, "Error", err)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// delete flow
	if !ni.DeletionTimestamp.IsZero() {
		if ni.Spec.NodeName != nil && *ni.Spec.NodeName != r.HostName {
			return ctrl.Result{}, nil
		}

		log.Info("Delete flow")
		clone := ni.DeepCopy()

		if ni.Status.Access != nil {
			interfaceID := string(ni.Status.Access.UID)
			if err := r.deleteInterfaceDPSKServerCall(ctx, interfaceID); err != nil {
				ni.Status.State = networkingv1alpha1.NetworkInterfaceStateError
				if err := r.Status().Patch(ctx, clone, client.MergeFrom(ni)); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
			r.DeviceAllocator.FreeDevice(ni.Status.Access.NetworkAttributes[DpPciAddr])
			ni.Status.Access.NetworkAttributes[DpPciAddr] = ""
		}

		log.V(1).Info("Withdrawing Private route", "NIC", ni.Name, "PublicIP", ni.Spec.IP, "VNI", network.Spec.ID)
		if err := r.announceInterfaceLocalRoute(ctx, ni.Spec, network.Spec, ni.Status.Access, networkingv1alpha1.ROUTEREMOVE); err != nil {
			if !strings.Contains(fmt.Sprint(err), "Nexthop does not exist") {
				return ctrl.Result{}, fmt.Errorf("failed to withdraw a route. %v", err)
			} else {
				log.Info("Tried to remove the same route for the same VM.")
			}
		}

		controllerutil.RemoveFinalizer(clone, NetworkInterfaceFinalizerName)
		if err := r.Patch(ctx, clone, client.MergeFrom(ni)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if ni.Status.Phase == networkingv1alpha1.NetworkInterfacePhaseBound {
		return ctrl.Result{}, nil
	}

	if ni.Status.Phase == networkingv1alpha1.NetworkInterfacePhaseUnbound {
		// Are we still synchron with dp-service internal state ?
		if ni.Status.Access != nil {
			interfaceID := string(ni.Status.Access.UID)
			_, err := r.getInterfaceDPSKServerCall(ctx, interfaceID)
			if err != nil {
				clone := ni.DeepCopy()
				clone.Status.Phase = ""
				if err := r.Status().Patch(ctx, clone, client.MergeFrom(ni)); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
		return ctrl.Result{}, nil
	}

	if ni.Status.Phase == networkingv1alpha1.NetworkInterfacePhasePending {
		return ctrl.Result{}, nil
	}

	n := &networkingv1alpha1.Network{}
	key := types.NamespacedName{
		Namespace: req.NamespacedName.Namespace,
		Name:      ni.Spec.NetworkRef.Name,
	}
	if err := r.Get(ctx, key, n); err != nil {
		log.Info("unable to fetch Network", "Error", err)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, client.IgnoreNotFound(err)
	}

	dpPci := ""

	if ni.Status.Access != nil {
		dpPci = ni.Status.Access.NetworkAttributes[DpPciAddr]
	}
	if dpPci == "" {
		newDevice, err := r.DeviceAllocator.ReserveDevice()
		if err != nil {
			log.V(1).Error(err, "PCI device reservation error")
			return ctrl.Result{}, err
		}
		dpPci = newDevice
		log.V(1).Info("Assigning new Network PCI Device", "PCI:", newDevice)
	} else {
		log.V(1).Info("Using assigned Network PCI Device", "PCI:", dpPci)
		r.DeviceAllocator.ReserveDeviceWithName(dpPci)
	}

	interfaceID, resp, err := r.addInterfaceDPSKServerCall(ctx, ni.Spec, n.Spec, dpPci)
	if err != nil {
		r.DeviceAllocator.FreeDevice(dpPci)
		return ctrl.Result{}, err
	}
	log.Info("AddInterface GRPC call", "resp", resp)

	clone := ni.DeepCopy()

	clone.Status.Phase = networkingv1alpha1.NetworkInterfacePhasePending
	clone.Status.LastPhaseTransitionTime = &v1.Time{Time: v1.Now().Time}

	if err := r.Status().Patch(ctx, clone, client.MergeFrom(ni)); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	ni = clone
	na := &networkingv1alpha1.NetworkInterfaceAccess{}
	na.UID = types.UID(interfaceID)
	na.NetworkAttributes = map[string]string{
		UnderlayRoute: "",
	}
	na.NetworkAttributes[UnderlayRoute] = string(resp.Response.UnderlayRoute)
	na.NetworkAttributes[DpPciAddr] = string(resp.Vf.Name)
	if err := r.MbInstance.Subscribe(mb.VNI(n.Spec.ID)); err != nil {
		log.Info("duplicate subscription, IGNORED for now due to boostrap of virt networks")
	}

	if err := r.announceInterfaceLocalRoute(ctx, ni.Spec, n.Spec, na, networkingv1alpha1.ROUTEADD); err != nil {
		if !strings.Contains(fmt.Sprint(err), "Nexthop already exists") {
			log.Error(err, "failed to announce route")
			return ctrl.Result{}, err
		} else {
			log.Info("Tried to announce the same route for the same VM.")
		}
	}

	if err := r.insertDefaultVNIPublicRoute(ctx, n.Spec.ID); err != nil {
		log.Error(err, "failed to add default route to vni %d", n.Spec.ID)
		return ctrl.Result{}, err
	}

	clone = ni.DeepCopy()

	if clone.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(clone, NetworkInterfaceFinalizerName) {
		controllerutil.AddFinalizer(clone, NetworkInterfaceFinalizerName)
	}
	clone.Spec.NodeName = &r.HostName

	if err := r.Patch(ctx, clone, client.MergeFrom(ni)); err != nil {
		log.Info("unable to update NetworkInterface", "NetworkInterface", req, "Error", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	ni = clone

	clone = ni.DeepCopy()
	clone.Status.Phase = networkingv1alpha1.NetworkInterfacePhaseUnbound
	clone.Status.LastPhaseTransitionTime = &v1.Time{Time: v1.Now().Time}
	clone.Status.Access = na
	clone.Status.State = networkingv1alpha1.NetworkInterfaceStateReady

	if err := r.Status().Patch(ctx, clone, client.MergeFrom(ni)); err != nil {
		log.Info("unable to update NetworkInterface", "NetworkInterface", req, "Error", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, nil
}

func (r *NetworkInterfaceReconciler) deleteInterfaceDPSKServerCall(ctx context.Context, interfaceID string) error {
	delInterfaceReq := &dpdkproto.InterfaceIDMsg{
		InterfaceID: []byte(interfaceID),
	}
	status, err := r.DPDKClient.DeleteInterface(ctx, delInterfaceReq)
	if err != nil {
		return err
	}
	if status.Error != 0 && status.Error != 151 { // 151 - interface not found
		return fmt.Errorf("eror during Grpc call, DeleteInterface, code=%v", status.Error)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) getInterfaceDPSKServerCall(ctx context.Context, interfaceID string) (*dpdkproto.GetInterfaceResponse, error) {
	getInterfaceReq := &dpdkproto.InterfaceIDMsg{
		InterfaceID: []byte(interfaceID),
	}
	resp, err := r.DPDKClient.GetInterface(ctx, getInterfaceReq)
	if err != nil {
		return nil, err
	}
	if resp.Status.Error != 0 {
		return nil, fmt.Errorf("eror during Grpc call, GetInterface, code=%v", resp.Status.Error)
	}
	return resp, nil
}

func (r *NetworkInterfaceReconciler) addInterfaceDPSKServerCall(ctx context.Context, niSpec networkingv1alpha1.NetworkInterfaceSpec, nSpec networkingv1alpha1.NetworkSpec, pciAddr string) (string, *dpdkproto.CreateInterfaceResponse, error) {
	interfaceID := uuid.New().String()
	ip := niSpec.IP.String()
	createInterfaceReq := &dpdkproto.CreateInterfaceRequest{
		InterfaceType: dpdkproto.InterfaceType_VirtualInterface,
		InterfaceID:   []byte(interfaceID),
		Vni:           uint32(nSpec.ID),
		DeviceName:    pciAddr,
		Ipv4Config: &dpdkproto.IPConfig{
			IpVersion:      dpdkproto.IPVersion_IPv4,
			PrimaryAddress: []byte(ip),
		},
		Ipv6Config: &dpdkproto.IPConfig{
			IpVersion:      dpdkproto.IPVersion_IPv6,
			PrimaryAddress: []byte(RandomIpV6Address()),
		},
	}
	resp, err := r.DPDKClient.CreateInterface(ctx, createInterfaceReq)

	if err != nil {
		return "", nil, err
	}
	if resp.Response.Status.Error != 0 && resp.Response.Status.Error != 106 {
		return "", nil, fmt.Errorf("eror during Grpc call, CreateInterface, code=%v", resp.Response.Status.Error)
	}

	return interfaceID, resp, nil
}

func (r *NetworkInterfaceReconciler) announceInterfaceLocalRoute(ctx context.Context, niSpec networkingv1alpha1.NetworkInterfaceSpec, nSpec networkingv1alpha1.NetworkSpec, na *networkingv1alpha1.NetworkInterfaceAccess, action int) error {

	if niSpec.IP == nil || na == nil {
		return nil
	}

	ip := niSpec.IP.String() + "/32"
	prefix, err := netip.ParsePrefix(ip)
	if err != nil {
		return fmt.Errorf("failed to convert interface ip to prefix version, reson=%v", err)
	}

	var ipversion mb.IPVersion
	if prefix.Addr().Is4() {
		ipversion = mb.IPV4
	} else {
		ipversion = mb.IPV6
	}

	dest := mb.Destination{
		IPVersion: ipversion,
		Prefix:    prefix,
	}

	hopIP, err := netip.ParseAddr(na.NetworkAttributes[UnderlayRoute])
	if err != nil {
		return fmt.Errorf("invalid nexthop address: %s - %v", na.NetworkAttributes[UnderlayRoute], err)
	}

	hop := mb.NextHop{
		TargetAddress: hopIP,
		TargetVNI:     0,
		NAT:           false,
	}

	if action == networkingv1alpha1.ROUTEADD {
		if err = r.MbInstance.AnnounceRoute(mb.VNI(nSpec.ID), dest, hop); err != nil {
			return fmt.Errorf("failed to announce a local route, reason: %v", err)
		}
	} else {
		if err = r.MbInstance.WithdrawRoute(mb.VNI(nSpec.ID), dest, hop); err != nil {
			return fmt.Errorf("failed to withdraw a local route, reason: %v", err)
		}
	}

	return nil
}

func (r *NetworkInterfaceReconciler) insertDefaultVNIPublicRoute(ctx context.Context, vni int32) error {

	prefix := &dpdkproto.Prefix{
		PrefixLength: uint32(0),
	}

	prefix.IpVersion = dpdkproto.IPVersion_IPv4 //only ipv4 in overlay is supported so far
	prefix.Address = []byte("0.0.0.0")

	req := &dpdkproto.VNIRouteMsg{
		Vni: &dpdkproto.VNIMsg{Vni: uint32(vni)},
		Route: &dpdkproto.Route{
			IpVersion:      dpdkproto.IPVersion_IPv6, //only ipv4 in overlay is supported so far
			Weight:         100,                      // this field is ignored in dp-service
			Prefix:         prefix,
			NexthopVNI:     uint32(vni),
			NexthopAddress: []byte(r.RouterAddress),
		},
	}

	status, err := r.DPDKClient.AddRoute(ctx, req)
	if err != nil || (status.Error != 0 && status.Error != DpRouteAlreadyAddedError) {
		return fmt.Errorf("cannot add route to dpdk service: %v Status from DPDKClient: %d", err, status.Error)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkInterfaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.NetworkInterface{}).
		Complete(r)
}

func RandomIpV6Address() string {
	// TODO: delete after close https://github.com/onmetal/net-dpservice/issues/71
	var ip net.IP
	for i := 0; i < net.IPv6len; i++ {
		number := uint8(rand.Intn(255))
		ip = append(ip, number)
	}
	return ip.String()
}