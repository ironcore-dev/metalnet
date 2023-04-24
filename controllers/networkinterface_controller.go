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
	crand "crypto/rand"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/jaypipes/ghw"
	"github.com/onmetal/controller-utils/clientutils"
	"github.com/onmetal/metalbond/pb"
	metalnetv1alpha1 "github.com/onmetal/metalnet/api/v1alpha1"
	metalnetclient "github.com/onmetal/metalnet/client"
	"github.com/onmetal/metalnet/dpdk"
	"github.com/onmetal/metalnet/metalbond"
	"github.com/onmetal/metalnet/netfns"
	"github.com/onmetal/metalnet/sysfs"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	networkInterfaceFinalizer = "networking.metalnet.onmetal.de/networkInterface"
)

// workaroundNoNetworkInterfaceIPV6 is a workaround to only use ipv4 addresses.
// remove as soon as bug is fixed.
func workaroundNoNetworkInterfaceIPV6(ips []netip.Addr) []netip.Addr {
	var res []netip.Addr
	for _, ip := range ips {
		if ip.Is4() {
			res = append(res, ip)
		}
	}
	return res
}

func workaroundRequiredIPv6Address(ip netip.Addr) netip.Addr {
	if ip.IsValid() {
		return ip
	}

	var data [16]byte
	_, err := crand.Read(data[:])
	utilruntime.Must(err)

	return netip.AddrFrom16(data)
}

func getIP(ipFamily corev1.IPFamily, ipFamilies []corev1.IPFamily, ips []metalnetv1alpha1.IP) netip.Addr {
	for i, family := range ipFamilies {
		if ipFamily == family {
			return ips[i].Addr
		}
	}
	return netip.Addr{}
}

func getNetworkInterfaceIP(ipFamily corev1.IPFamily, nic *metalnetv1alpha1.NetworkInterface) netip.Addr {
	return getIP(ipFamily, nic.Spec.IPFamilies, nic.Spec.IPs)
}

func getNetworkInterfaceIPs(nic *metalnetv1alpha1.NetworkInterface) []netip.Addr {
	res := make([]netip.Addr, len(nic.Spec.IPs))
	for i, ip := range nic.Spec.IPs {
		res[i] = ip.Addr
	}
	return res
}

// NetworkInterfaceReconciler reconciles a NetworkInterface object
type NetworkInterfaceReconciler struct {
	client.Client
	record.EventRecorder

	Scheme *runtime.Scheme

	DPDK      dpdk.Client
	Metalbond metalbond.Client

	NetFnsManager *netfns.Manager
	SysFS         sysfs.FS

	NodeName    string
	PublicVNI   int
	LBServerMap map[uint32]types.UID
}

//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=networkinterfaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=networkinterfaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=networkinterfaces/finalizers,verbs=update
//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=networks,verbs=get;list;watch
//+kubebuilder:rbac:groups=networking.metalnet.onmetal.de,resources=loadbalancers,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NetworkInterfaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	nic := &metalnetv1alpha1.NetworkInterface{}

	if err := r.Get(ctx, req.NamespacedName, nic); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if nic.Spec.NodeName != nil && *nic.Spec.NodeName == r.NodeName {
		return r.reconcileExists(ctx, log, nic)
	} else {
		log.V(1).Info("Network interface is not assigned to this node", "NodeName", nic.Spec.NodeName)
		return ctrl.Result{}, nil
	}
}

func (r *NetworkInterfaceReconciler) reconcileExists(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface) (ctrl.Result, error) {
	if !nic.DeletionTimestamp.IsZero() {
		return r.delete(ctx, log, nic)
	}

	return r.reconcile(ctx, log, nic)
}

func NetIPAddrPrefix(addr netip.Addr) netip.Prefix {
	return netip.PrefixFrom(addr, addr.BitLen())
}

func (r *NetworkInterfaceReconciler) releaseNetFnIfClaimExists(uid types.UID) error {
	if err := r.NetFnsManager.Release(uid); err != nil && !errors.Is(err, netfns.ErrClaimNotFound) {
		return fmt.Errorf("error releasing claim: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) deleteDPDKVirtualIPIfExists(ctx context.Context, nic *metalnetv1alpha1.NetworkInterface) error {
	if err := r.DPDK.DeleteVirtualIP(ctx, nic.UID); dpdk.IgnoreStatusErrorCode(err, dpdk.DEL_NAT) != nil {
		return fmt.Errorf("error deleting dpdk virtual ip: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) removeVirtualIPRouteIfExists(ctx context.Context, virtualIP netip.Addr, underlayRoute netip.Addr) error {
	if err := r.Metalbond.RemoveRoute(ctx, metalbond.VNI(r.PublicVNI), metalbond.Destination{
		Prefix: NetIPAddrPrefix(virtualIP),
	}, metalbond.NextHop{
		TargetAddress: underlayRoute,
		TargetVNI:     0,
		TargetHopType: pb.NextHopType_STANDARD,
	}); metalbond.IgnoreNextHopNotFoundError(err) != nil {
		return fmt.Errorf("error removing metalbond route: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) addVirtualIPRouteIfNotExists(ctx context.Context, virtualIP netip.Addr, underlayRoute netip.Addr) error {
	if err := r.Metalbond.AddRoute(ctx, metalbond.VNI(r.PublicVNI), metalbond.Destination{
		Prefix: NetIPAddrPrefix(virtualIP),
	}, metalbond.NextHop{
		TargetAddress: underlayRoute,
		TargetVNI:     0,
		TargetHopType: pb.NextHopType_STANDARD,
	}); metalbond.IgnoreNextHopAlreadyExistsError(err) != nil {
		return fmt.Errorf("error adding metalbond route: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) addInterfaceRouteIfNotExists(ctx context.Context, vni uint32, ip, underlayRoute netip.Addr) error {
	if err := r.Metalbond.AddRoute(ctx, metalbond.VNI(vni), metalbond.Destination{
		Prefix: NetIPAddrPrefix(ip),
	}, metalbond.NextHop{
		TargetVNI:     0,
		TargetAddress: underlayRoute,
	}); metalbond.IgnoreNextHopAlreadyExistsError(err) != nil {
		return fmt.Errorf("error adding interface route: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) addInterfaceRoutesIfNotExist(ctx context.Context, vni uint32, ips []netip.Addr, underlayRoute netip.Addr) error {
	for _, localAddr := range ips {
		if err := r.addInterfaceRouteIfNotExists(ctx, vni, localAddr, underlayRoute); err != nil {
			return fmt.Errorf("[local address %s] %w", localAddr, err)
		}
	}
	return nil
}

func (r *NetworkInterfaceReconciler) removeInterfaceRouteIfExists(ctx context.Context, vni uint32, ip, underlayRoute netip.Addr) error {
	if err := r.Metalbond.RemoveRoute(ctx, metalbond.VNI(vni), metalbond.Destination{
		Prefix: NetIPAddrPrefix(ip),
	}, metalbond.NextHop{
		TargetVNI:     0,
		TargetAddress: underlayRoute,
	}); metalbond.IgnoreNextHopNotFoundError(err) != nil {
		return fmt.Errorf("error removing interface route: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) removeInterfaceRoutesIfExist(ctx context.Context, vni uint32, ips []netip.Addr, underlayRoute netip.Addr) error {
	for _, localAddr := range ips {
		if err := r.removeInterfaceRouteIfExists(ctx, vni, localAddr, underlayRoute); err != nil {
			return fmt.Errorf("[local address %s] %w", localAddr, err)
		}
	}
	return nil
}

func (r *NetworkInterfaceReconciler) addPrefixRouteIfNotExists(ctx context.Context, vni uint32, prefix netip.Prefix, underlayRoute netip.Addr) error {
	if err := r.Metalbond.AddRoute(ctx, metalbond.VNI(vni), metalbond.Destination{
		Prefix: prefix,
	}, metalbond.NextHop{
		TargetVNI:     0,
		TargetAddress: underlayRoute,
		TargetHopType: pb.NextHopType_STANDARD,
	}); metalbond.IgnoreNextHopAlreadyExistsError(err) != nil {
		return fmt.Errorf("error adding prefix route: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) removePrefixRouteIfExists(ctx context.Context, vni uint32, prefix netip.Prefix, underlayRoute netip.Addr) error {
	if err := r.Metalbond.RemoveRoute(ctx, metalbond.VNI(vni), metalbond.Destination{
		Prefix: prefix,
	}, metalbond.NextHop{
		TargetVNI:     0,
		TargetAddress: underlayRoute,
		TargetHopType: pb.NextHopType_STANDARD,
	}); metalbond.IgnoreNextHopNotFoundError(err) != nil {
		return fmt.Errorf("error removing prefix route: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) addLBTargetRouteIfNotExists(ctx context.Context, vni uint32, prefix netip.Prefix, underlayRoute netip.Addr) error {
	if err := r.Metalbond.AddRoute(ctx, metalbond.VNI(vni), metalbond.Destination{
		Prefix: prefix,
	}, metalbond.NextHop{
		TargetVNI:     0,
		TargetAddress: underlayRoute,
		TargetHopType: pb.NextHopType_LOADBALANCER_TARGET,
	}); metalbond.IgnoreNextHopAlreadyExistsError(err) != nil {
		return fmt.Errorf("error adding lb target route: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) removeLBTargetRouteIfExists(ctx context.Context, vni uint32, prefix netip.Prefix, underlayRoute netip.Addr) error {
	if err := r.Metalbond.RemoveRoute(ctx, metalbond.VNI(vni), metalbond.Destination{
		Prefix: prefix,
	}, metalbond.NextHop{
		TargetVNI:     0,
		TargetAddress: underlayRoute,
		TargetHopType: pb.NextHopType_LOADBALANCER_TARGET,
	}); metalbond.IgnoreNextHopNotFoundError(err) != nil {
		return fmt.Errorf("error removing prefix route: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) deleteDPDKLBTargetIfExists(ctx context.Context, nicUID types.UID, prefix netip.Prefix) error {
	if err := r.DPDK.DeleteLBPrefix(ctx, nicUID, prefix); dpdk.IgnoreStatusErrorCode(err, dpdk.DEL_PFX_NO_VM) != nil {
		return fmt.Errorf("error deleting lb target: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) deleteDPDKPrefixIfExists(ctx context.Context, nicUID types.UID, prefix netip.Prefix) error {
	if err := r.DPDK.DeletePrefix(ctx, nicUID, prefix); dpdk.IgnoreStatusErrorCode(err, dpdk.DEL_PFX_NO_VM) != nil {
		return fmt.Errorf("error deleting prefix: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) deleteDPDKInterfaceIfExists(ctx context.Context, uid types.UID) error {
	if err := r.DPDK.DeleteInterface(ctx, uid); dpdk.IgnoreStatusErrorCode(err, dpdk.DEL_PFX_NO_VM) != nil {
		return fmt.Errorf("error deleting interface: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) reconcileNATIP(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface, vni uint32) error {
	if nic.Spec.NAT != nil && nic.Spec.NAT.IP != nil {
		natIP := nic.Spec.NAT.IP.Addr
		log = log.WithValues("NatIP", natIP)
		log.V(1).Info("Apply nat ip")
		return r.applyNATIP(ctx, log, nic, natIP, vni)
	}

	log.V(1).Info("Delete nat ip")
	return r.deleteNATIP(ctx, log, nic, vni)
}

func (r *NetworkInterfaceReconciler) applyNATIP(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface, natIP netip.Addr, vni uint32) error {
	log.V(1).Info("Getting dpdk nat ip")
	dpdkNAT, err := r.DPDK.GetNATLocal(ctx, nic.UID)
	if err != nil {
		if !dpdk.IsStatusErrorCode(err, dpdk.GET_NAT, dpdk.GET_NAT_NO_IP_SET) {
			return fmt.Errorf("error getting dpdk nat ip: %w", err)
		}

		log.V(1).Info("DPDK nat ip does not exist, creating it")
		return r.createNATIP(ctx, log, nic, natIP, vni)
	}

	underlayRoute := dpdkNAT.Status.UnderlayRoute
	existingNATIP := dpdkNAT.Spec.Address
	if existingNATIP == natIP && dpdkNAT.Spec.MinPort == uint32(nic.Spec.NAT.Port) && dpdkNAT.Spec.MaxPort == uint32(nic.Spec.NAT.EndPort) {
		log.V(1).Info("DPDK nat ip is up-to-date, adding metalbond route if not exists")
		if err := r.addNATIPRouteIfNotExists(ctx, dpdkNAT, underlayRoute, vni); err != nil {
			return err
		}
		log.V(1).Info("Ensured metalbond route exists")
		return nil
	}

	log.V(1).Info("NAT ip and/or NAT ports are not up-to-date", "ExistingNATIP", existingNATIP, "MinPort", dpdkNAT.Spec.MinPort, "MaxPort", dpdkNAT.Spec.MaxPort)

	log.V(1).Info("Delete existing nat ip")
	if err := r.deleteExistingNATIP(ctx, log, nic, dpdkNAT, underlayRoute, vni); err != nil {
		return err
	}
	log.V(1).Info("Deleted existing nat ip")

	log.V(1).Info("Creating nat ip")
	if err := r.createNATIP(ctx, log, nic, natIP, vni); err != nil {
		return err
	}
	log.V(1).Info("Created nat ip")
	return nil
}

func (r *NetworkInterfaceReconciler) createNATIP(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface, natIP netip.Addr, vni uint32) error {
	natLocal, err := r.DPDK.CreateNATLocal(ctx, &dpdk.NATLocal{
		NATLocalMetadata: dpdk.NATLocalMetadata{InterfaceUID: nic.UID},
		Spec: dpdk.NATLocalSpec{Address: natIP,
			MinPort: uint32(nic.Spec.NAT.Port),
			MaxPort: uint32(nic.Spec.NAT.EndPort)},
	})
	if err != nil {
		return fmt.Errorf("error creating dpdk nat ip: %w", err)
	}
	log.V(1).Info("Adding nat ip route if not exists")
	if err := r.addNATIPRouteIfNotExists(ctx, natLocal, natLocal.Status.UnderlayRoute, vni); err != nil {
		return err
	}
	log.V(1).Info("Added nat ip route if not existed")
	return nil
}

func (r *NetworkInterfaceReconciler) deleteNATIP(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface, vni uint32) error {
	log.V(1).Info("Getting dpdk nat ip if exists")
	dpdkVIP, err := r.DPDK.GetNATLocal(ctx, nic.UID)
	if err != nil {
		if !dpdk.IsStatusErrorCode(err, dpdk.GET_NAT, dpdk.GET_NAT_NO_IP_SET) {
			return fmt.Errorf("error getting dpdk nat ip: %w", err)
		}

		log.V(1).Info("DPDK nat ip does not exist")
		return nil
	}

	underlayRoute := dpdkVIP.Status.UnderlayRoute
	natIP := dpdkVIP.Spec.Address
	log.V(1).Info("NAT ip exists", "ExistingNATIP", natIP)
	return r.deleteExistingNATIP(ctx, log, nic, dpdkVIP, underlayRoute, vni)
}

func (r *NetworkInterfaceReconciler) deleteExistingNATIP(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface, natLocal *dpdk.NATLocal, underlayRoute netip.Addr, vni uint32) error {
	log.V(1).Info("Removing nat ip route if exists")
	if err := r.removeNATIPRouteIfExists(ctx, natLocal, underlayRoute, vni); err != nil {
		return err
	}
	log.V(1).Info("Removed nat ip route fi existed")

	log.V(1).Info("Deleting dpdk nat ip if exists")
	if err := r.deleteDPDKNATIPIfExists(ctx, nic); err != nil {
		return err
	}
	log.V(1).Info("Deleted dpdk nat ip if existed")
	return nil
}

func (r *NetworkInterfaceReconciler) deleteDPDKNATIPIfExists(ctx context.Context, nic *metalnetv1alpha1.NetworkInterface) error {
	if err := r.DPDK.DeleteNATLocal(ctx, nic.UID); dpdk.IgnoreStatusErrorCode(err, dpdk.DEL_NAT) != nil {
		return fmt.Errorf("error deleting dpdk nat ip: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) removeNATIPRouteIfExists(ctx context.Context, natLocal *dpdk.NATLocal, underlayRoute netip.Addr, vni uint32) error {
	if err := r.Metalbond.RemoveRoute(ctx, metalbond.VNI(r.PublicVNI), metalbond.Destination{
		Prefix: NetIPAddrPrefix(natLocal.Spec.Address),
	}, metalbond.NextHop{
		TargetAddress: underlayRoute,
		TargetVNI:     0,
		TargetHopType: pb.NextHopType_STANDARD,
	}); metalbond.IgnoreNextHopNotFoundError(err) != nil {
		return fmt.Errorf("error removing metalbond route: %w", err)
	}
	if err := r.Metalbond.RemoveRoute(ctx, metalbond.VNI(vni), metalbond.Destination{
		Prefix: NetIPAddrPrefix(natLocal.Spec.Address),
	}, metalbond.NextHop{
		TargetAddress:    underlayRoute,
		TargetVNI:        0,
		TargetHopType:    pb.NextHopType_NAT,
		TargetNATMinPort: uint16(natLocal.Spec.MinPort),
		TargetNATMaxPort: uint16(natLocal.Spec.MaxPort),
	}); metalbond.IgnoreNextHopNotFoundError(err) != nil {
		return fmt.Errorf("error removing metalbond route: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) addNATIPRouteIfNotExists(ctx context.Context, natLocal *dpdk.NATLocal, underlayRoute netip.Addr, vni uint32) error {
	if err := r.Metalbond.AddRoute(ctx, metalbond.VNI(r.PublicVNI), metalbond.Destination{
		Prefix: NetIPAddrPrefix(natLocal.Spec.Address),
	}, metalbond.NextHop{
		TargetAddress: underlayRoute,
		TargetVNI:     0,
		TargetHopType: pb.NextHopType_STANDARD,
	}); metalbond.IgnoreNextHopAlreadyExistsError(err) != nil {
		return fmt.Errorf("error adding metalbond route: %w", err)
	}
	if err := r.Metalbond.AddRoute(ctx, metalbond.VNI(vni), metalbond.Destination{
		Prefix: NetIPAddrPrefix(natLocal.Spec.Address),
	}, metalbond.NextHop{
		TargetAddress:    underlayRoute,
		TargetVNI:        0,
		TargetHopType:    pb.NextHopType_NAT,
		TargetNATMinPort: uint16(natLocal.Spec.MinPort),
		TargetNATMaxPort: uint16(natLocal.Spec.MaxPort),
	}); metalbond.IgnoreNextHopAlreadyExistsError(err) != nil {
		return fmt.Errorf("error adding metalbond route: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) reconcileVirtualIP(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface) error {
	if nic.Spec.VirtualIP != nil {
		virtualIP := nic.Spec.VirtualIP.Addr
		log = log.WithValues("VirtualIP", virtualIP)
		log.V(1).Info("Apply virtual ip")
		return r.applyVirtualIP(ctx, log, nic, virtualIP)
	}

	log.V(1).Info("Delete virtual ip")
	return r.deleteVirtualIP(ctx, log, nic)
}

func (r *NetworkInterfaceReconciler) applyVirtualIP(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface, virtualIP netip.Addr) error {
	log.V(1).Info("Getting dpdk virtual ip")
	dpdkVIP, err := r.DPDK.GetVirtualIP(ctx, nic.UID)
	if err != nil {
		if !dpdk.IsStatusErrorCode(err, dpdk.GET_NAT, dpdk.GET_NAT_NO_IP_SET) {
			return fmt.Errorf("error getting dpdk virtual ip: %w", err)
		}

		log.V(1).Info("DPDK virtual ip does not exist, creating it")
		return r.createVirtualIP(ctx, log, nic, virtualIP)
	}
	underlayRoute := dpdkVIP.Status.UnderlayRoute
	existingVirtualIP := dpdkVIP.Spec.Address
	if existingVirtualIP == virtualIP {
		log.V(1).Info("DPDK virtual ip is up-to-date, adding metalbond route if not exists")
		if err := r.addVirtualIPRouteIfNotExists(ctx, virtualIP, underlayRoute); err != nil {
			return err
		}
		log.V(1).Info("Ensured metalbond route exists")
		return nil
	}

	log.V(1).Info("Virtual ip is not up-to-date", "ExistingVirtualIP", existingVirtualIP)

	log.V(1).Info("Delete existing virtual ip")
	if err := r.deleteExistingVirtualIP(ctx, log, nic, existingVirtualIP, underlayRoute); err != nil {
		return err
	}
	log.V(1).Info("Deleted existing virtual ip")

	log.V(1).Info("Creating virtual ip")
	if err := r.createVirtualIP(ctx, log, nic, virtualIP); err != nil {
		return err
	}
	log.V(1).Info("Created virtual ip")
	return nil
}

func (r *NetworkInterfaceReconciler) createVirtualIP(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface, virtualIP netip.Addr) error {
	dpdkVIP, err := r.DPDK.CreateVirtualIP(ctx, &dpdk.VirtualIP{
		VirtualIPMetadata: dpdk.VirtualIPMetadata{InterfaceUID: nic.UID},
		Spec:              dpdk.VirtualIPSpec{Address: virtualIP},
	})
	if err != nil {
		return fmt.Errorf("error creating dpdk virtual ip: %w", err)
	}
	log.V(1).Info("Adding virtual ip route if not exists")
	if err := r.addVirtualIPRouteIfNotExists(ctx, virtualIP, dpdkVIP.Status.UnderlayRoute); err != nil {
		return err
	}
	log.V(1).Info("Added virtual ip route if not existed")
	return nil
}

func (r *NetworkInterfaceReconciler) deleteVirtualIP(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface) error {
	log.V(1).Info("Getting dpdk virtual ip if exists")
	dpdkVIP, err := r.DPDK.GetVirtualIP(ctx, nic.UID)
	if err != nil {
		if !dpdk.IsStatusErrorCode(err, dpdk.GET_NAT, dpdk.GET_NAT_NO_IP_SET) {
			return fmt.Errorf("error getting dpdk virtual ip: %w", err)
		}

		log.V(1).Info("DPDK virtual ip does not exist")
		return nil
	}

	virtualIP := dpdkVIP.Spec.Address
	underlayRoute := dpdkVIP.Status.UnderlayRoute
	log.V(1).Info("Virtual ip exists", "ExistingVirtualIP", virtualIP, "UnderlayRoute", underlayRoute)
	return r.deleteExistingVirtualIP(ctx, log, nic, virtualIP, underlayRoute)
}

func (r *NetworkInterfaceReconciler) deleteExistingVirtualIP(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface, virtualIP, underlayRoute netip.Addr) error {
	log.V(1).Info("Removing virtual ip route if exists")
	if err := r.removeVirtualIPRouteIfExists(ctx, virtualIP, underlayRoute); err != nil {
		return err
	}
	log.V(1).Info("Removed virtual ip route fi existed")

	log.V(1).Info("Deleting dpdk virtual ip if exists")
	if err := r.deleteDPDKVirtualIPIfExists(ctx, nic); err != nil {
		return err
	}
	log.V(1).Info("Deleted dpdk virtual ip if existed")
	return nil
}

func (r *NetworkInterfaceReconciler) reconcile(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface) (ctrl.Result, error) {
	log.V(1).Info("Reconcile")

	log.V(1).Info("Ensuring finalizer")
	modified, err := clientutils.PatchEnsureFinalizer(ctx, r.Client, nic, networkInterfaceFinalizer)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error ensuring finalizer: %w", err)
	}
	if modified {
		log.V(1).Info("Added finalizer")
		return ctrl.Result{Requeue: true}, nil
	}
	log.V(1).Info("Ensured finalizer")

	network := &metalnetv1alpha1.Network{}
	networkKey := client.ObjectKey{Namespace: nic.Namespace, Name: nic.Spec.NetworkRef.Name}
	log.V(1).Info("Getting network", "NetworkKey", networkKey)
	if err := r.Get(ctx, networkKey, network); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting network %s: %w", networkKey, err)
		}

		r.Eventf(nic, corev1.EventTypeWarning, "NetworkNotFound", "Network %s could not be found", networkKey.Name)
		if err := r.patchStatus(ctx, nic, func() {
			nic.Status = metalnetv1alpha1.NetworkInterfaceStatus{
				State: metalnetv1alpha1.NetworkInterfaceStatePending,
			}
		}); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	vni := uint32(network.Spec.ID)
	log.V(1).Info("Got network", "NetworkKey", networkKey, "VNI", vni)

	log.V(1).Info("Applying interface")
	pciAddr, underlayRoute, err := r.applyInterface(ctx, log, nic, vni)
	if err != nil {
		if err := r.patchStatus(ctx, nic, func() {
			nic.Status = metalnetv1alpha1.NetworkInterfaceStatus{
				State: metalnetv1alpha1.NetworkInterfaceStateError,
			}
		}); err != nil {
			log.Error(err, "Error patching network interface status")
		}
		return ctrl.Result{}, fmt.Errorf("error applying interface: %w", err)
	}
	log.V(1).Info("Applied interface", "PCIAddress", pciAddr, "UnderlayRoute", underlayRoute)

	var errs []error

	log.V(1).Info("Reconciling virtual ip")
	virtualIPErr := r.reconcileVirtualIP(ctx, log, nic)
	if virtualIPErr != nil {
		errs = append(errs, fmt.Errorf("error reconciling virtual ip: %w", virtualIPErr))
		log.Error(virtualIPErr, "Error reconciling virtual ip")
		r.Eventf(nic, corev1.EventTypeWarning, "ErrorReconcilingVirtualIP", "Error reconciling virtual ip: %v", err)
	} else {
		log.V(1).Info("Reconciled virtual ip")
	}

	log.V(1).Info("Reconciling nat ip")
	natIPErr := r.reconcileNATIP(ctx, log, nic, vni)
	if natIPErr != nil {
		errs = append(errs, fmt.Errorf("error reconciling nat ip: %w", natIPErr))
		log.Error(natIPErr, "Error reconciling nat ip")
		r.Eventf(nic, corev1.EventTypeWarning, "ErrorReconcilingNATIP", "Error reconciling nat ip: %v", err)
	} else {
		log.V(1).Info("Reconciled nat ip")
	}

	log.V(1).Info("Reconciling lb targets")
	lbTargetErr := r.reconcileLBTargets(ctx, log, vni, nic)
	if lbTargetErr != nil {
		errs = append(errs, fmt.Errorf("error reconciling lb target: %w", lbTargetErr))
		log.Error(lbTargetErr, "Error reconciling lb targets")
		r.Eventf(nic, corev1.EventTypeWarning, "ErrorReconcilingPrefixes", "Error reconciling prefixes: %v", err)
	} else {
		log.V(1).Info("Reconciled prefixes")
	}

	log.V(1).Info("Reconciling prefixes")
	prefixesErr := r.reconcilePrefixes(ctx, log, vni, nic)
	if prefixesErr != nil {
		errs = append(errs, fmt.Errorf("error reconciling prefixes: %w", prefixesErr))
		log.Error(prefixesErr, "Error reconciling prefixes")
		r.Eventf(nic, corev1.EventTypeWarning, "ErrorReconcilingPrefixes", "Error reconciling prefixes: %v", err)
	} else {
		log.V(1).Info("Reconciled prefixes")
	}

	log.V(1).Info("Patching status")
	if err := r.patchStatus(ctx, nic, func() {
		nic.Status.State = metalnetv1alpha1.NetworkInterfaceStateReady
		nic.Status.PCIAddress = &metalnetv1alpha1.PCIAddress{
			Bus:      pciAddr.Bus,
			Domain:   pciAddr.Domain,
			Slot:     pciAddr.Device,
			Function: pciAddr.Function,
		}
		if virtualIPErr != nil {
			nic.Status.VirtualIP = nic.Spec.VirtualIP
		}
		if natIPErr != nil {
			nic.Status.NatIP = nic.Spec.NAT.IP
		}
		if prefixesErr != nil {
			nic.Status.Prefixes = nic.Spec.Prefixes
		}
		if lbTargetErr != nil {
			nic.Status.LoadBalancerTargets = nic.Spec.LoadBalancerTargets
		}
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching status: %w", err)
	}

	if len(errs) > 0 {
		return ctrl.Result{}, fmt.Errorf("error applying network interface parts: %v", errs)
	}
	return ctrl.Result{}, nil
}

func (r *NetworkInterfaceReconciler) reconcilePrefixes(ctx context.Context, log logr.Logger, vni uint32, nic *metalnetv1alpha1.NetworkInterface) error {
	log.V(1).Info("Listing alias prefixes")
	list, err := r.DPDK.ListPrefixes(ctx, nic.UID)
	if err != nil {
		return fmt.Errorf("error listing alias prefixes: %w", err)
	}

	dpdkPrefixes := sets.New[netip.Prefix]()
	for _, dpdkPrefix := range list.Items {
		dpdkPrefixes.Insert(dpdkPrefix.Spec.Prefix)
	}

	specPrefixes := sets.New[netip.Prefix]()
	for _, specPrefix := range nic.Spec.Prefixes {
		specPrefixes.Insert(specPrefix.Prefix)
	}

	// Sort prefixes to have deterministic error event output
	allPrefixes := dpdkPrefixes.UnsortedList()
	sort.Slice(allPrefixes, func(i, j int) bool {
		return allPrefixes[i].String() < allPrefixes[j].String()
	})

	if dpdkPrefixes.Len() < specPrefixes.Len() {
		allPrefixes = specPrefixes.UnsortedList()
		sort.Slice(allPrefixes, func(i, j int) bool {
			return allPrefixes[i].String() < allPrefixes[j].String()
		})
	}
	var errs []error
	for _, prefix := range allPrefixes {
		if err := func() error {
			log := log.WithValues("Prefix", prefix)
			switch {
			case dpdkPrefixes.Has(prefix) && !specPrefixes.Has(prefix):
				log.V(1).Info("Delete prefix")
				underlayRoute, err := getUnderlayRouteFromPrefixesList(list.Items, prefix)
				if err != nil {
					return err
				}
				log.V(1).Info("Ensuring metalbond prefix route does not exist")
				if err := r.removePrefixRouteIfExists(ctx, vni, prefix, underlayRoute); err != nil {
					return err
				}
				log.V(1).Info("Ensured metalbond prefix route does not exist")

				log.V(1).Info("Ensuring dpdk prefix does not exist")
				if err := r.deleteDPDKPrefixIfExists(ctx, nic.UID, prefix); err != nil {
					return err
				}
				log.V(1).Info("Ensured dpdk prefix does not exist")
				return nil
			case specPrefixes.Has(prefix) && !dpdkPrefixes.Has(prefix):
				log.V(1).Info("Create prefix")

				log.V(1).Info("Creating dpdk prefix")
				resPrefix, err := r.DPDK.CreatePrefix(ctx, &dpdk.Prefix{
					PrefixMetadata: dpdk.PrefixMetadata{InterfaceUID: nic.UID},
					Spec:           dpdk.PrefixSpec{Prefix: prefix},
				})
				if err != nil {
					return err
				}
				log.V(1).Info("Ensured dpdk prefix exists")

				log.V(1).Info("Ensuring metalbond prefix route exists")
				if err := r.addPrefixRouteIfNotExists(ctx, vni, prefix, resPrefix.Spec.UnderlayRoute); err != nil {
					return err
				}
				log.V(1).Info("Ensured metalbond prefix route exists")
				return nil
			default:
				log.V(1).Info("Update prefix")
				underlayRoute, err := getUnderlayRouteFromPrefixesList(list.Items, prefix)
				if err != nil {
					return err
				}
				log.V(1).Info("Ensuring metalbond prefix route exists")
				if err := r.addPrefixRouteIfNotExists(ctx, vni, prefix, underlayRoute); err != nil {
					return err
				}
				log.V(1).Info("Ensured metalbond prefix route exists")
				return nil
			}
		}(); err != nil {
			errs = append(errs, fmt.Errorf("[prefix %s] %w", prefix, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("error(s) reconciling prefix(es): %v", errs)
	}
	return nil
}

func getUnderlayRouteFromPrefixesList(list []dpdk.Prefix, searchPrefix netip.Prefix) (netip.Addr, error) {
	for _, dpdkPrefix := range list {
		if dpdkPrefix.Spec.Prefix.Addr() == searchPrefix.Addr() {
			return dpdkPrefix.Spec.UnderlayRoute, nil
		}
	}
	return netip.Addr{}, fmt.Errorf("no underlayroute for lb prefix %v", searchPrefix.Addr())
}

func (r *NetworkInterfaceReconciler) reconcileLBTargets(ctx context.Context, log logr.Logger, vni uint32, nic *metalnetv1alpha1.NetworkInterface) error {
	log.V(1).Info("Listing lb targets")
	list, err := r.DPDK.ListLBPrefixes(ctx, nic.UID)
	if err != nil {
		return fmt.Errorf("error listing lb targets: %w", err)
	}

	dpdkPrefixes := sets.New[netip.Prefix]()
	for _, dpdkPrefix := range list.Items {
		dpdkPrefixes.Insert(dpdkPrefix.Spec.Prefix)
	}

	specPrefixes := sets.New[netip.Prefix]()
	for _, specPrefix := range nic.Spec.LoadBalancerTargets {
		specPrefixes.Insert(specPrefix.Prefix)
	}

	// Sort prefixes to have deterministic error event output
	allPrefixes := dpdkPrefixes.UnsortedList()
	sort.Slice(allPrefixes, func(i, j int) bool {
		return allPrefixes[i].String() < allPrefixes[j].String()
	})

	if dpdkPrefixes.Len() < specPrefixes.Len() {
		allPrefixes = specPrefixes.UnsortedList()
		sort.Slice(allPrefixes, func(i, j int) bool {
			return allPrefixes[i].String() < allPrefixes[j].String()
		})
	}
	var errs []error
	for _, prefix := range allPrefixes {
		if err := func() error {
			log := log.WithValues("LB Target", prefix)
			switch {
			case dpdkPrefixes.Has(prefix) && !specPrefixes.Has(prefix):
				log.V(1).Info("Delete lb target")
				underlayRoute, err := getUnderlayRouteFromPrefixesList(list.Items, prefix)
				if err != nil {
					return err
				}
				log.V(1).Info("Ensuring metalbond prefix route does not exist")
				if err := r.removeLBTargetRouteIfExists(ctx, vni, prefix, underlayRoute); err != nil {
					return err
				}
				log.V(1).Info("Ensured metalbond prefix route does not exist")

				log.V(1).Info("Ensuring dpdk lb target does not exist")
				if err := r.deleteDPDKLBTargetIfExists(ctx, nic.UID, prefix); err != nil {
					return err
				}
				log.V(1).Info("Ensured dpdk lb target does not exist")
				return nil
			case specPrefixes.Has(prefix) && !dpdkPrefixes.Has(prefix):
				log.V(1).Info("Create lb target")

				log.V(1).Info("Creating dpdk lb target")
				resPrefix, err := r.DPDK.CreateLBPrefix(ctx, &dpdk.Prefix{
					PrefixMetadata: dpdk.PrefixMetadata{InterfaceUID: nic.UID},
					Spec:           dpdk.PrefixSpec{Prefix: prefix},
				})
				if err != nil {
					return err
				}
				log.V(1).Info("Ensured dpdk lb target exists")

				log.V(1).Info("Ensuring metalbond lb target route exists")
				if err := r.addLBTargetRouteIfNotExists(ctx, vni, prefix, resPrefix.Spec.UnderlayRoute); err != nil {
					return err
				}
				log.V(1).Info("Ensured metalbond lb target route exists")
				return nil
			default:
				log.V(1).Info("Update lb target")
				underlayRoute, err := getUnderlayRouteFromPrefixesList(list.Items, prefix)
				if err != nil {
					return err
				}
				if err := r.addLBTargetRouteIfNotExists(ctx, vni, prefix, underlayRoute); err != nil {
					return err
				}
				log.V(1).Info("Ensured metalbond lb target route exists")
				return nil
			}
		}(); err != nil {
			errs = append(errs, fmt.Errorf("[lb target %s] %w", prefix, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("error(s) reconciling lb target: %v", errs)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) applyInterface(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface, vni uint32) (*ghw.PCIAddress, netip.Addr, error) {
	log.V(1).Info("Getting dpdk interface")
	iface, err := r.DPDK.GetInterface(ctx, nic.UID)
	if err != nil {
		if !dpdk.IsStatusErrorCode(err, dpdk.GET_VM_NOT_FND) {
			return nil, netip.Addr{}, fmt.Errorf("error getting dpdk interface: %w", err)
		}

		log.V(1).Info("DPDK getting interface", "error", err)
		log.V(1).Info("DPDK interface does not yet exist, creating it")

		log.V(1).Info("Getting or claiming pci address")
		addr, err := r.NetFnsManager.GetOrClaim(nic.UID)
		if err != nil {
			return nil, netip.Addr{}, fmt.Errorf("error claiming address: %w", err)
		}
		log.V(1).Info("Got pci address", "Address", addr)

		log.V(1).Info("Converting to dpdk device")
		dpdkDevice, err := r.convertToDPDKDevice(*addr)
		if err != nil {
			return nil, netip.Addr{}, fmt.Errorf("error converting %s to dpdk device: %w", addr, err)
		}
		log.V(1).Info("Converted to dpdk device", "DPDKDevice", dpdkDevice)

		log.V(1).Info("Creating dpdk interface")
		iface, err := r.DPDK.CreateInterface(ctx, &dpdk.Interface{
			InterfaceMetadata: dpdk.InterfaceMetadata{UID: nic.UID},
			Spec: dpdk.InterfaceSpec{
				VNI:                vni,
				Device:             dpdkDevice,
				PrimaryIPv4Address: getNetworkInterfaceIP(corev1.IPv4Protocol, nic),
				PrimaryIPv6Address: workaroundRequiredIPv6Address(getNetworkInterfaceIP(corev1.IPv6Protocol, nic)),
			},
		})
		if err != nil {
			return nil, netip.Addr{}, fmt.Errorf("error creating dpdk interface: %w", err)
		}
		log.V(1).Info("Adding interface routes if not exist")
		ips := workaroundNoNetworkInterfaceIPV6(getNetworkInterfaceIPs(nic))
		if err := r.addInterfaceRoutesIfNotExist(ctx, vni, ips, iface.Status.UnderlayRoute); err != nil {
			return nil, netip.Addr{}, err
		}
		log.V(1).Info("Added interface routes if not existed")
		return addr, iface.Status.UnderlayRoute, nil
	}

	log.V(1).Info("DPDK interface exists")

	log.V(1).Info("Getting pci device for uid")
	addr, err := r.NetFnsManager.Get(nic.UID)
	if err != nil {
		return nil, netip.Addr{}, fmt.Errorf("error getting pci address: %w", err)
	}
	log.V(1).Info("Got pci device for uid", "PCIDevice", addr)

	log.V(1).Info("Adding interface route if not exists")
	ips := workaroundNoNetworkInterfaceIPV6(getNetworkInterfaceIPs(nic))
	if err := r.addInterfaceRoutesIfNotExist(ctx, vni, ips, iface.Status.UnderlayRoute); err != nil {
		return nil, netip.Addr{}, err
	}
	log.V(1).Info("Added interface route if not existed")
	return addr, iface.Status.UnderlayRoute, nil
}

func (r *NetworkInterfaceReconciler) convertToDPDKDevice(addr ghw.PCIAddress) (string, error) {
	pciFunction, err := strconv.ParseUint(addr.Function, 8, 64)
	if err != nil {
		return "", fmt.Errorf("error parsing address function %s: %w", addr.Function, err)
	}

	pciDevice, err := strconv.ParseUint(addr.Device, 16, 64)
	if err != nil {
		return "", fmt.Errorf("error parsing address device %s: %w", addr.Device, err)
	}

	pciDev, err := r.SysFS.PCIDevice(addr)
	if err != nil {
		return "", fmt.Errorf("error getting sysfs pci device: %w", err)
	}

	physFn, err := pciDev.Physfn()
	if err != nil {
		return "", fmt.Errorf("error getting sysfs physfn: %w", err)
	}

	physFnAddr, err := physFn.Address()
	if err != nil {
		return "", fmt.Errorf("error getting physfn details: %w", err)
	}

	sriov, err := physFn.SRIOV()
	if err != nil {
		return "", fmt.Errorf("error getting sysfs sriov: %w", err)
	}

	pciFunction = pciDevice*8 + pciFunction
	return fmt.Sprintf("%s:%s:%s.0_representor_vf%d", physFnAddr.Domain, physFnAddr.Bus, physFnAddr.Device, pciFunction-sriov.Offset), nil
}

func (r *NetworkInterfaceReconciler) patchStatus(
	ctx context.Context,
	nic *metalnetv1alpha1.NetworkInterface,
	mutate func(),
) error {
	base := nic.DeepCopy()

	mutate()

	if err := r.Status().Patch(ctx, nic, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("error patching status: %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) delete(ctx context.Context, log logr.Logger, nic *metalnetv1alpha1.NetworkInterface) (ctrl.Result, error) {
	log.V(1).Info("Delete")

	if !controllerutil.ContainsFinalizer(nic, networkInterfaceFinalizer) {
		log.V(1).Info("No finalizer present, nothing to do")
		return ctrl.Result{}, nil
	}

	log.V(1).Info("Finalizer present, cleaning up")

	log.V(1).Info("Getting dpdk interface")
	dpdkIface, err := r.DPDK.GetInterface(ctx, nic.UID)
	if err != nil {
		if !dpdk.IsStatusErrorCode(err, dpdk.GET_VM_NOT_FND) {
			return ctrl.Result{}, fmt.Errorf("error getting dpdk interface: %w", err)
		}

		log.V(1).Info("Releasing device if existed")
		if err := r.releaseNetFnIfClaimExists(nic.UID); err != nil {
			return ctrl.Result{}, fmt.Errorf("error removing claim: %w", err)
		}
		log.V(1).Info("Released device if existed")

		log.V(1).Info("No dpdk interface, removing finalizer")
		if err := clientutils.PatchRemoveFinalizer(ctx, r.Client, nic, networkInterfaceFinalizer); err != nil {
			return ctrl.Result{}, fmt.Errorf("error removing finalizer: %w", err)
		}
		log.V(1).Info("Removed finalizer")
		return ctrl.Result{}, nil
	}

	vni := dpdkIface.Spec.VNI
	underlayRoute := dpdkIface.Status.UnderlayRoute
	log.V(1).Info("Got dpdk interface", "VNI", vni, "UnderlayRoute", underlayRoute)

	log.V(1).Info("Deleting prefixes")
	if err := r.deletePrefixes(ctx, log, nic, vni); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting prefixes: %w", err)
	}
	log.V(1).Info("Deleted prefixes")

	log.V(1).Info("Deleting lb targets")
	if err := r.deleteLBTargets(ctx, log, nic, vni); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting lb targets: %w", err)
	}
	log.V(1).Info("Deleted lb targets")

	log.V(1).Info("Deleting nat ip")
	if err := r.deleteNATIP(ctx, log, nic, vni); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting nat ip: %w", err)
	}
	log.V(1).Info("Deleted nat ip")

	log.V(1).Info("Deleting virtual ip")
	if err := r.deleteVirtualIP(ctx, log, nic); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting virtual ip: %w", err)
	}
	log.V(1).Info("Deleted virtual ip")

	log.V(1).Info("Deleting interface")
	if err := r.deleteInterface(ctx, log, nic, vni, underlayRoute); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting underlay route: %w", err)
	}
	log.V(1).Info("Deleted interface")

	log.V(1).Info("Removing finalizer")
	if err := clientutils.PatchRemoveFinalizer(ctx, r.Client, nic, networkInterfaceFinalizer); err != nil {
		return ctrl.Result{}, fmt.Errorf("error removing finalizer: %w", err)
	}
	log.V(1).Info("Removed finalizer")
	return ctrl.Result{}, nil
}

func (r *NetworkInterfaceReconciler) deleteLBTargets(
	ctx context.Context,
	log logr.Logger,
	nic *metalnetv1alpha1.NetworkInterface,
	vni uint32,
) error {
	log.V(1).Info("Listing lb targets")
	prefixes, err := r.DPDK.ListLBPrefixes(ctx, nic.UID)
	if err != nil {
		if !dpdk.IsStatusErrorCode(err, dpdk.GET_VM_NOT_FND) {
			return fmt.Errorf("error listing lb targets: %w", err)
		}

		log.V(1).Info("Interface already gone")
		return nil
	}

	var errs []error
	for _, prefixItem := range prefixes.Items {
		prefix := prefixItem.Spec.Prefix
		log := log.WithValues("LB Target", prefix)
		if err := func() error {
			log.V(1).Info("Removing lb targets route if exists")
			if err := r.removeLBTargetRouteIfExists(ctx, vni, prefix, prefixItem.Spec.UnderlayRoute); err != nil {
				return err
			}
			log.V(1).Info("Removed lb target route if existed")

			log.V(1).Info("Removing dpdk lb target if exists")
			if err := r.deleteDPDKLBTargetIfExists(ctx, nic.UID, prefix); err != nil {
				return err
			}
			log.V(1).Info("Removed dpdk lb target if existed")
			return nil
		}(); err != nil {
			errs = append(errs, fmt.Errorf("[lb target %s] %w", prefix, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("error(s) deleting lb target(s): %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) deletePrefixes(
	ctx context.Context,
	log logr.Logger,
	nic *metalnetv1alpha1.NetworkInterface,
	vni uint32,
) error {
	log.V(1).Info("Listing prefixes")
	prefixes, err := r.DPDK.ListPrefixes(ctx, nic.UID)
	if err != nil {
		if !dpdk.IsStatusErrorCode(err, dpdk.GET_VM_NOT_FND) {
			return fmt.Errorf("error listing prefixes: %w", err)
		}

		log.V(1).Info("Interface already gone")
		return nil
	}

	var errs []error
	for _, prefix := range prefixes.Items {
		pfx := prefix.Spec.Prefix
		log := log.WithValues("Prefix", pfx)
		if err := func() error {
			log.V(1).Info("Removing prefix route if exists")
			if err := r.removePrefixRouteIfExists(ctx, vni, pfx, prefix.Spec.UnderlayRoute); err != nil {
				return err
			}
			log.V(1).Info("Removed prefix route if existed")

			log.V(1).Info("Removing dpdk prefix if exists")
			if err := r.deleteDPDKPrefixIfExists(ctx, nic.UID, pfx); err != nil {
				return err
			}
			log.V(1).Info("Removed dpdk prefix if existed")
			return nil
		}(); err != nil {
			errs = append(errs, fmt.Errorf("[prefix %s] %w", pfx, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("error(s) deleting prefix(es): %w", err)
	}
	return nil
}

func (r *NetworkInterfaceReconciler) deleteInterface(
	ctx context.Context,
	log logr.Logger,
	nic *metalnetv1alpha1.NetworkInterface,
	vni uint32,
	underlayRoute netip.Addr,
) error {
	log.V(1).Info("Removing interface route if exists")
	ips := workaroundNoNetworkInterfaceIPV6(getNetworkInterfaceIPs(nic))
	if err := r.removeInterfaceRoutesIfExist(ctx, vni, ips, underlayRoute); err != nil {
		return err
	}
	log.V(1).Info("Removed interface route if existed")

	log.V(1).Info("Deleting dpdk interface if exists")
	if err := r.deleteDPDKInterfaceIfExists(ctx, nic.UID); err != nil {
		return err
	}
	log.V(1).Info("Deleted dpdk interface if existed")

	log.V(1).Info("Releasing device if existed")
	if err := r.releaseNetFnIfClaimExists(nic.UID); err != nil {
		return err
	}
	log.V(1).Info("Released device if existed")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkInterfaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	log := ctrl.Log.WithName("networkinterface").WithName("setup")
	ctx := ctrl.LoggerInto(context.TODO(), log)

	return ctrl.NewControllerManagedBy(mgr).
		For(&metalnetv1alpha1.NetworkInterface{}).
		Watches(
			&source.Kind{Type: &metalnetv1alpha1.Network{}},
			r.enqueueNetworkInterfacesReferencingNetwork(ctx, log),
		).
		Watches(
			&source.Kind{Type: &metalnetv1alpha1.LoadBalancer{}},
			r.enqueueNetworkInterfacesReferencingLoadBalancer(ctx, log),
		).
		Complete(r)
}

func (r *NetworkInterfaceReconciler) enqueueNetworkInterfacesReferencingNetwork(ctx context.Context, log logr.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []ctrl.Request {
		network := obj.(*metalnetv1alpha1.Network)
		nicList := &metalnetv1alpha1.NetworkInterfaceList{}
		if err := r.List(ctx, nicList,
			client.InNamespace(network.Namespace),
			client.MatchingFields{metalnetclient.NetworkInterfaceNetworkRefNameField: network.Name},
		); err != nil {
			log.Error(err, "Error listing network interfaces referencing network", "NetworkKey", client.ObjectKeyFromObject(network))
			return nil
		}

		reqs := make([]ctrl.Request, len(nicList.Items))
		for i, nic := range nicList.Items {
			reqs[i] = ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&nic)}
		}
		return reqs
	})
}

func (r *NetworkInterfaceReconciler) enqueueNetworkInterfacesReferencingLoadBalancer(ctx context.Context, log logr.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []ctrl.Request {
		loadBalancer := obj.(*metalnetv1alpha1.LoadBalancer)
		nicList := &metalnetv1alpha1.NetworkInterfaceList{}
		if err := r.List(ctx, nicList,
			client.InNamespace(loadBalancer.Namespace),
			client.MatchingFields{metalnetclient.LoadBalancerNetworkRefNameField: loadBalancer.Spec.NetworkRef.Name},
		); err != nil {
			log.Error(err, "Error listing network interfaces referencing loadbalancer", "NetworkKey", client.ObjectKeyFromObject(loadBalancer))
			return nil
		}

		reqs := make([]ctrl.Request, len(nicList.Items))
		for i, nic := range nicList.Items {
			reqs[i] = ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&nic)}
		}
		return reqs
	})
}
