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

	. "github.com/ironcore-dev/ironcore/utils/testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpdkerrors "github.com/ironcore-dev/dpservice-go/errors"
	dpdk "github.com/ironcore-dev/dpservice-go/proto"
	metalnetv1alpha1 "github.com/ironcore-dev/metalnet/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Network Controller", Label("network"), Ordered, func() {
	ctx := SetupContext()
	ns := SetupTest(ctx)

	Context("When creating a Network", Ordered, func() {
		It("should create successfully", func() {
			network = &metalnetv1alpha1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-network",
					Namespace: ns.Name,
				},
				Spec: metalnetv1alpha1.NetworkSpec{
					ID:        111,
					PeeredIDs: []int32{2, 3},
					PeeredPrefixes: []metalnetv1alpha1.PeeredPrefix{
						{
							ID:       2,
							Prefixes: []metalnetv1alpha1.IPPrefix{}, // Add desired IPPrefixes here
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, network)).To(Succeed())

			// Ensure it's created
			createdNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "test-network",
			}, createdNetwork)).To(Succeed())

			// Equal() uses reflect.DeepEqual to compare (it compares whole structs recursively)
			Expect(createdNetwork).To(Equal(network))

			vniAvail, err := dpdkClient.GetVni(ctx, 111, uint8(dpdk.VniType_VNI_IPV4))
			Expect(err).NotTo(HaveOccurred())
			Expect(vniAvail.Spec.InUse).To(BeFalse())
		})

		It("should reconcile successfully", func() {
			// Create and initialize network reconciler
			Expect(networkReconcile(ctx, *network)).To(Succeed())

			// Fetch the updated Network object from k8s
			fetchedNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Name:      network.Name,
				Namespace: network.Namespace,
			}, fetchedNetwork)).To(Succeed())

			Expect(fetchedNetwork.Spec.ID).To(Equal(int32(111)))
		})

		It("should fail when already existing", func() {
			Expect(k8sClient.Create(ctx, network)).ToNot(Succeed())
		})

		It("should update successfully", func() {
			// Update the k8s network object
			patchNetwork := network.DeepCopy()
			patchNetwork.Spec.PeeredIDs = []int32{4, 5}
			Expect(k8sClient.Patch(ctx, patchNetwork, client.MergeFrom(network))).To(Succeed())

			Expect(networkReconcile(ctx, *network)).To(Succeed())

			// Fetch updated k8s network object
			updatedNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Name:      network.Name,
				Namespace: network.Namespace,
			}, updatedNetwork)).To(Succeed())

			Expect(updatedNetwork.Spec.PeeredIDs).To(Equal([]int32{4, 5}))
		})
	})

	Context("When deleting a Network", Ordered, func() {
		It("should delete successfully", func() {
			// Delete the Network object
			Expect(k8sClient.Delete(ctx, network)).To(Succeed())

			// Ensure it's deleted
			deletedNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "test-network",
			}, deletedNetwork)).ToNot(Succeed())
		})

		It("should reconcile successfully after delete", func() {
			// Create and initialize network reconciler
			Expect(networkReconcile(ctx, *network)).To(Succeed())

			// Fetch the updated Network object from k8s
			fetchedNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Name:      network.Name,
				Namespace: network.Namespace,
			}, fetchedNetwork)).ToNot(Succeed())

			vniAvail, err := dpdkClient.GetVni(ctx, 111, uint8(dpdk.VniType_VNI_IPV4))
			Expect(err).NotTo(HaveOccurred())
			Expect(vniAvail.Spec.InUse).To(BeFalse())
		})
	})

})

var _ = Describe("Network Interface and LoadBalancer Controller", func() {
	var (
		loadBalancer     *metalnetv1alpha1.LoadBalancer
		networkInterface *metalnetv1alpha1.NetworkInterface
	)
	ctx := SetupContext()
	ns := SetupTest(ctx)

	// Creates the k8s network object and runs the reconcile loop before spec
	// OncePerOrdered decorator will run this only once per Ordered spec and not before every It spec
	BeforeEach(OncePerOrdered, func() {
		network = &metalnetv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-network",
				Namespace: ns.Name,
			},
			Spec: metalnetv1alpha1.NetworkSpec{
				ID:        123,
				PeeredIDs: []int32{2, 3},
				PeeredPrefixes: []metalnetv1alpha1.PeeredPrefix{
					{
						ID:       2,
						Prefixes: []metalnetv1alpha1.IPPrefix{}, // Add desired IPPrefixes here
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, network)).To(Succeed())

		Expect(networkReconcile(ctx, *network)).To(Succeed())

		// Deletes the k8s network object after spec is completed
		DeferCleanup(func(ctx SpecContext) {
			Expect(k8sClient.Delete(ctx, network)).To(Succeed())
			Expect(networkReconcile(ctx, *network)).To(Succeed())
		})
	})

	Context("NetworkInterface", Label("interface"), Ordered, func() {
		When("creating a NetworkInterface", func() {
			It("should create successfully", func() {
				// Define a new NetworkInterface object
				networkInterface = &metalnetv1alpha1.NetworkInterface{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-network-interface",
						Namespace: ns.Name,
					},
					Spec: metalnetv1alpha1.NetworkInterfaceSpec{
						NetworkRef: corev1.LocalObjectReference{
							Name: "test-network",
						},
						NodeName:   &testNode,
						IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol},
						IPs: []metalnetv1alpha1.IP{
							{
								Addr: netip.MustParseAddr("10.0.0.1"),
							},
						},
					},
				}

				// Create the NetworkInterface k8s object
				Expect(k8sClient.Create(ctx, networkInterface)).To(Succeed())

				// Ensure it's created
				createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Namespace: ns.Name,
					Name:      "test-network-interface",
				}, createdNetworkInterface)).To(Succeed())

				Expect(createdNetworkInterface.Spec.NetworkRef.Name).To(Equal("test-network"))
				Expect(createdNetworkInterface.Spec.IPs[0].Addr.String()).To(Equal("10.0.0.1"))

				// It should not yet be created in dpservice
				iface, err := dpdkClient.GetInterface(ctx, string(networkInterface.ObjectMeta.UID))
				Expect(err).To(HaveOccurred())
				Expect(iface.Status.Code).To(Equal(uint32(dpdkerrors.NOT_FOUND)))
			})

			It("should fail when already existing", func() {
				Expect(k8sClient.Create(ctx, networkInterface)).ToNot(Succeed())
			})

			It("should reconcile successfully", func() {
				Expect(ifaceReconcile(ctx, *networkInterface)).To(Succeed())
				// Fetch the updated Iface object from k8s
				fetchedIface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, fetchedIface)).To(Succeed())
				Expect(fetchedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				// Fetch the Iface object from dpservice
				iface, err := dpdkClient.GetInterface(ctx, string(networkInterface.ObjectMeta.UID))
				Expect(err).ToNot(HaveOccurred())
				Expect(iface.Spec.IPv4.String()).To(Equal("10.0.0.1"))
				Expect(iface.InterfaceMeta.ID).To(Equal(string(fetchedIface.UID)))

				// Fetch the VNI object from dpservice
				vniAvail, err := dpdkClient.GetVni(ctx, 123, uint8(dpdk.VniType_VNI_IPV4))
				Expect(err).NotTo(HaveOccurred())
				Expect(vniAvail.Spec.InUse).To(BeTrue())
			})
		})

		When("updating a NetworkInterface", func() {
			It("IP should update successfully", func() {
				// Update the k8s NetworkInterface object IP
				patchIface := networkInterface.DeepCopy()
				patchIface.Spec.IPs = []metalnetv1alpha1.IP{
					{
						Addr: netip.MustParseAddr("10.0.0.2"),
					},
				}
				// Apply patch
				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(networkInterface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *networkInterface)).To(Succeed())

				// Fetch updated k8s NetworkInterface object
				updatedIface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.IPs[0].Addr.String()).To(Equal("10.0.0.2"))
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))
			})

			It("VIP should add/update/delete successfully", func() {
				By("adding the VIP")
				// Add VIP to k8s interface object
				patchIface := networkInterface.DeepCopy()
				patchIface.Spec.VirtualIP = &metalnetv1alpha1.IP{Addr: netip.MustParseAddr("10.10.10.10")}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(networkInterface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *networkInterface)).To(Succeed())

				// Fetch updated k8s interface object
				updatedIface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.VirtualIP.Addr.String()).To(Equal("10.10.10.10"))
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				// Fetch the dpservice interface object
				iface, err := dpdkClient.GetInterface(ctx, string(networkInterface.ObjectMeta.UID))
				Expect(err).ToNot(HaveOccurred())
				// TODO: dpservice object is not updated during reconciliation, it will be implemented later
				// Expect(iface.Spec.VIP.Spec.IP.String()).To(Equal("10.10.10.10"))
				Expect(iface.Spec.VIP).To(BeNil())

				By("updating the VIP")
				// Update the k8s interface object
				patchIface = updatedIface.DeepCopy()
				patchIface.Spec.VirtualIP = &metalnetv1alpha1.IP{Addr: netip.MustParseAddr("10.10.10.20")}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(updatedIface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *updatedIface)).To(Succeed())

				// Fetch updated k8s object
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.VirtualIP.Addr.String()).To(Equal("10.10.10.20"))
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				By("deleting the VIP")
				// Delete VIP from k8s interface object
				patchIface = updatedIface.DeepCopy()
				patchIface.Spec.VirtualIP = nil

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(updatedIface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *updatedIface)).To(Succeed())

				// Fetch updated k8s interface object
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.VirtualIP).To(BeNil())
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				// Fetch the dpservice interface object
				iface, err = dpdkClient.GetInterface(ctx, string(networkInterface.ObjectMeta.UID))
				Expect(err).ToNot(HaveOccurred())
				Expect(iface.Spec.VIP).To(BeNil())
			})

			It("NAT should add/update/delete successfully", func() {
				By("adding the NAT")
				// Add NAT to k8s interface object
				patchIface := networkInterface.DeepCopy()
				patchIface.Spec.NAT = &metalnetv1alpha1.NATDetails{
					IP:      &metalnetv1alpha1.IP{Addr: netip.MustParseAddr("20.20.20.20")},
					Port:    1000,
					EndPort: 2000,
				}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(networkInterface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *networkInterface)).To(Succeed())

				// Fetch updated k8s interface object
				updatedIface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.NAT.IP.Addr.String()).To(Equal("20.20.20.20"))
				Expect(updatedIface.Spec.NAT.Port).To(Equal(int32(1000)))
				Expect(updatedIface.Spec.NAT.EndPort).To(Equal(int32(2000)))
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				By("updating the NAT")
				// Update NAT of k8s interface object
				patchIface = updatedIface.DeepCopy()
				patchIface.Spec.NAT = &metalnetv1alpha1.NATDetails{
					IP:      &metalnetv1alpha1.IP{Addr: netip.MustParseAddr("30.30.30.30")},
					Port:    3000,
					EndPort: 4000,
				}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(updatedIface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *updatedIface)).To(Succeed())

				// Fetch updated k8s interface object
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.NAT.IP.Addr.String()).To(Equal("30.30.30.30"))
				Expect(updatedIface.Spec.NAT.Port).To(Equal(int32(3000)))
				Expect(updatedIface.Spec.NAT.EndPort).To(Equal(int32(4000)))
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				By("deleting the NAT")
				// Delete nat from k8s interface object
				patchIface = updatedIface.DeepCopy()
				patchIface.Spec.NAT = nil

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(updatedIface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *updatedIface)).To(Succeed())

				// Fetch updated k8s interface object
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.NAT).To(BeNil())
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))
			})

			It("Prefixes should add/update/delete successfully", func() {
				By("adding the Prefix")
				patchIface := networkInterface.DeepCopy()
				patchIface.Spec.Prefixes = []metalnetv1alpha1.IPPrefix{
					metalnetv1alpha1.MustParseIPPrefix("10.0.0.0/24"),
				}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(networkInterface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *networkInterface)).To(Succeed())

				// Fetch updated k8s interface object
				updatedIface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.Prefixes).ToNot(BeEmpty())
				Expect(updatedIface.Spec.Prefixes[0].String()).To(Equal("10.0.0.0/24"))
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				By("updating the Prefix")
				patchIface = updatedIface.DeepCopy()
				patchIface.Spec.Prefixes = []metalnetv1alpha1.IPPrefix{
					metalnetv1alpha1.MustParseIPPrefix("10.1.1.0/24"),
				}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(updatedIface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *updatedIface)).To(Succeed())

				// Fetch updated k8s interface object
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.Prefixes).ToNot(BeEmpty())
				Expect(updatedIface.Spec.Prefixes[0].String()).To(Equal("10.1.1.0/24"))
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				By("deleting the Prefix")
				patchIface = updatedIface.DeepCopy()
				patchIface.Spec.Prefixes = []metalnetv1alpha1.IPPrefix{}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(updatedIface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *updatedIface)).To(Succeed())

				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.Prefixes).To(BeEmpty())
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))
			})

			It("LoadBalancerTargets should add/update/delete successfully", func() {
				By("adding the LoadBalancerTarget")
				patchIface := networkInterface.DeepCopy()
				patchIface.Spec.LoadBalancerTargets = []metalnetv1alpha1.IPPrefix{
					metalnetv1alpha1.MustParseIPPrefix("10.0.0.0/24"),
				}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(networkInterface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *networkInterface)).To(Succeed())

				// Fetch updated k8s interface object
				updatedIface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.LoadBalancerTargets).ToNot(BeEmpty())
				Expect(updatedIface.Spec.LoadBalancerTargets[0].String()).To(Equal("10.0.0.0/24"))
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				By("updating the LoadBalancerTarget")
				patchIface = updatedIface.DeepCopy()
				patchIface.Spec.LoadBalancerTargets = []metalnetv1alpha1.IPPrefix{
					metalnetv1alpha1.MustParseIPPrefix("10.1.1.0/24"),
				}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(updatedIface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *updatedIface)).To(Succeed())

				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.LoadBalancerTargets).ToNot(BeEmpty())
				Expect(updatedIface.Spec.LoadBalancerTargets[0].String()).To(Equal("10.1.1.0/24"))
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				By("deleting the LoadBalancerTarget")
				patchIface = updatedIface.DeepCopy()
				patchIface.Spec.LoadBalancerTargets = []metalnetv1alpha1.IPPrefix{}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(updatedIface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *updatedIface)).To(Succeed())

				// Fetch updated k8s interface object
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.LoadBalancerTargets).To(BeEmpty())
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))
			})

			It("FirewallRules should add/update/delete successfully", func() {
				By("adding the FirewallRule")
				var protocolType metalnetv1alpha1.ProtocolType = "TCP"
				var srcPort int32 = 80
				var dstPort int32 = -1
				fr1 := metalnetv1alpha1.FirewallRule{
					FirewallRuleID:    "fr1",
					Direction:         metalnetv1alpha1.FirewallRuleDirectionIngress,
					Action:            metalnetv1alpha1.FirewallRuleActionAccept,
					IpFamily:          "IPv4",
					SourcePrefix:      metalnetv1alpha1.MustParseNewIPPrefix("0.0.0.0/0"),
					DestinationPrefix: metalnetv1alpha1.MustParseNewIPPrefix("10.0.0.10/32"),
					ProtocolMatch: &metalnetv1alpha1.ProtocolMatch{
						ProtocolType: &protocolType,
						PortRange: &metalnetv1alpha1.PortMatch{
							SrcPort:    &srcPort,
							EndSrcPort: 80,
							DstPort:    &dstPort,
						},
					},
				}

				patchIface := networkInterface.DeepCopy()
				patchIface.Spec.FirewallRules = []metalnetv1alpha1.FirewallRule{fr1}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(networkInterface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *networkInterface)).To(Succeed())

				// Fetch updated k8s interface object
				updatedIface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.FirewallRules).ToNot(BeEmpty())
				Expect(updatedIface.Spec.FirewallRules[0].FirewallRuleID).To(Equal(types.UID("fr1")))
				Expect(updatedIface.Spec.FirewallRules[0].DestinationPrefix.String()).To(Equal("10.0.0.10/32"))
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				fw1, err := dpdkClient.GetFirewallRule(ctx, string(updatedIface.UID), string(fr1.FirewallRuleID))
				Expect(err).NotTo(HaveOccurred())
				Expect(fw1.Spec.FirewallAction).To(Equal("Accept"))
				Expect(fw1.Spec.TrafficDirection).To(Equal("Ingress"))
				Expect(fw1.Spec.SourcePrefix.String()).To(Equal("0.0.0.0/0"))

				By("updating the FirewallRule")
				dstPort = 443
				fr1.SourcePrefix = metalnetv1alpha1.MustParseNewIPPrefix("1.1.1.1/32")
				fr1.ProtocolMatch.PortRange.DstPort = &dstPort

				patchIface = updatedIface.DeepCopy()
				patchIface.Spec.FirewallRules = []metalnetv1alpha1.FirewallRule{fr1}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(updatedIface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *updatedIface)).To(Succeed())

				// Fetch updated k8s interface object
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.FirewallRules).ToNot(BeEmpty())
				Expect(updatedIface.Spec.FirewallRules[0].SourcePrefix.String()).To(Equal("1.1.1.1/32"))
				Expect(*updatedIface.Spec.FirewallRules[0].ProtocolMatch.PortRange.DstPort).To(Equal(int32(443)))
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				By("deleting the FirewallRule")
				patchIface = updatedIface.DeepCopy()
				patchIface.Spec.FirewallRules = []metalnetv1alpha1.FirewallRule{}

				Expect(k8sClient.Patch(ctx, patchIface, client.MergeFrom(updatedIface))).To(Succeed())

				Expect(ifaceReconcile(ctx, *updatedIface)).To(Succeed())

				// Fetch updated k8s interface object
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, updatedIface)).To(Succeed())

				Expect(updatedIface.Spec.FirewallRules).To(BeEmpty())
				Expect(updatedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				fw1, err = dpdkClient.GetFirewallRule(ctx, string(updatedIface.UID), string(fr1.FirewallRuleID))
				Expect(err).To(HaveOccurred())
				Expect(fw1.Status.Code).To(Equal(uint32(dpdkerrors.NOT_FOUND)))
			})
		})

		When("deleting a NetworkInterface", func() {
			It("should delete successfully", func() {
				// Delete the NetworkInterface object from k8s
				Expect(k8sClient.Delete(ctx, networkInterface)).To(Succeed())

				// Ensure it's deleted
				deletedIface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Namespace: ns.Name,
					Name:      networkInterface.Name,
				}, deletedIface)).ToNot(Succeed())

				// NetworkInterface should still be in dpservice
				_, err := dpdkClient.GetInterface(ctx, string(networkInterface.ObjectMeta.UID))
				Expect(err).ToNot(HaveOccurred())

				// Fetch the VNI object from dpservice
				vniAvail, err := dpdkClient.GetVni(ctx, 123, uint8(dpdk.VniType_VNI_IPV4))
				Expect(err).NotTo(HaveOccurred())
				Expect(vniAvail.Spec.InUse).To(BeTrue())
			})

			It("should reconcile successfully after delete", func() {
				// Create and initialize networkInterface reconciler
				Expect(ifaceReconcile(ctx, *networkInterface)).To(Succeed())

				// Try to fetch the deleted networkInterface object from k8s
				fetchedIface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, fetchedIface)).ToNot(Succeed())

				// Try to fetch the deleted networkInterface object from dpservice
				iface, err := dpdkClient.GetInterface(ctx, string(networkInterface.ObjectMeta.UID))
				Expect(err).To(HaveOccurred())
				Expect(iface.Status.Code).To(Equal(uint32(dpdkerrors.NOT_FOUND)))

				// Fetch the VNI object from dpservice
				vniAvail, err := dpdkClient.GetVni(ctx, 123, uint8(dpdk.VniType_VNI_IPV4))
				Expect(err).NotTo(HaveOccurred())
				Expect(vniAvail.Spec.InUse).To(BeFalse())
			})
		})
	})

	Context("Loadbalancer", Label("lb", "loadbalancer"), Ordered, func() {
		When("creating a Loadbalancer", func() {
			It("should create successfully", func() {
				// Defining and Creating Network is done in BeforeEach()

				// Define a new Loadbalancer object
				loadBalancer = &metalnetv1alpha1.LoadBalancer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-loadbalancer",
						Namespace: ns.Name,
					},
					Spec: metalnetv1alpha1.LoadBalancerSpec{
						NetworkRef: corev1.LocalObjectReference{Name: "test-network"},
						LBtype:     "Public",
						IPFamily:   corev1.IPv4Protocol,
						IP: metalnetv1alpha1.IP{
							Addr: netip.MustParseAddr("11.5.5.1"),
						},
						Ports: []metalnetv1alpha1.LBPort{
							{Protocol: "TCP", Port: 80},
							{Protocol: "UDP", Port: 80},
						},
						NodeName: &testNode,
					},
				}

				// Create the LoadBalancer object in k8s
				Expect(k8sClient.Create(ctx, loadBalancer)).To(Succeed())

				// Ensure it's created
				createdLB := &metalnetv1alpha1.LoadBalancer{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Namespace: ns.Name,
					Name:      "test-loadbalancer",
				}, createdLB)).To(Succeed())

				// requested LB and created LB should match
				Expect(createdLB).To(Equal(loadBalancer))

				// Finalizers should not yet be created
				Expect(createdLB.GetFinalizers()).To(BeNil())

				// LB should not yet be created in dpservice
				_, err := dpdkClient.GetLoadBalancer(ctx, string(loadBalancer.ObjectMeta.UID))
				Expect(err).To(HaveOccurred())
			})

			It("should fail when already existing", func() {
				Expect(k8sClient.Create(ctx, loadBalancer)).ToNot(Succeed())
			})

			It("should reconcile successfully", func() {
				// Create and initialize loadbalancer reconciler
				Expect(lbReconcile(ctx, *loadBalancer)).To(Succeed())

				// Fetch the updated LB object from k8s
				fetchedLB := &metalnetv1alpha1.LoadBalancer{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      loadBalancer.Name,
					Namespace: loadBalancer.Namespace,
				}, fetchedLB)).To(Succeed())

				Expect(fetchedLB.GetFinalizers()).ToNot(BeZero())
				Expect(fetchedLB.Status.State).To(Equal(metalnetv1alpha1.LoadBalancerStateReady))

				// Fetch the LB object from dpservice
				dpdkLB, err := dpdkClient.GetLoadBalancer(ctx, string(loadBalancer.ObjectMeta.UID))
				Expect(err).ToNot(HaveOccurred())

				// LB parameters in k8s and dpservice should match
				Expect(fetchedLB.Spec.IP.As4()).To(Equal(dpdkLB.Spec.LbVipIP.As4()))
				Expect(fetchedLB.Spec.Ports[0].Port).To(Equal(int32(dpdkLB.Spec.Lbports[0].Port)))

				// Fetch the VNI object from dpservice
				vniAvail, err := dpdkClient.GetVni(ctx, 123, uint8(dpdk.VniType_VNI_IPV4))
				Expect(err).NotTo(HaveOccurred())
				Expect(vniAvail.Spec.InUse).To(BeTrue())
				// Another reconcilition of network object is needed here. Because we dont have event watches in the test environment.
			})

			It("should update successfully", func() {
				// Update loadbalancer k8s object
				patchLB := loadBalancer.DeepCopy()
				patchLB.Spec.IP = metalnetv1alpha1.IP{
					Addr: netip.MustParseAddr("11.5.5.2"),
				}
				Expect(k8sClient.Patch(ctx, patchLB, client.MergeFrom(loadBalancer))).To(Succeed())

				Expect(lbReconcile(ctx, *loadBalancer)).To(Succeed())

				// Fetch updated k8s loadbalancer object
				updatedLB := &metalnetv1alpha1.LoadBalancer{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      loadBalancer.Name,
					Namespace: loadBalancer.Namespace,
				}, updatedLB)).To(Succeed())

				Expect(updatedLB.Spec.IP.Addr.String()).To(Equal("11.5.5.2"))
			})
		})

		When("deleting a Loadbalancer", Label("lb"), Ordered, func() {
			It("should delete successfully", func() {
				// Delete the Loadbalancer object from k8s
				Expect(k8sClient.Delete(ctx, loadBalancer)).To(Succeed())

				// Ensure it's deleted
				deletedLB := &metalnetv1alpha1.LoadBalancer{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Namespace: ns.Name,
					Name:      loadBalancer.Name,
				}, deletedLB)).ToNot(Succeed())

				// LB should still be in dpservice
				_, err := dpdkClient.GetLoadBalancer(ctx, string(loadBalancer.ObjectMeta.UID))
				Expect(err).ToNot(HaveOccurred())

				// Fetch the VNI object from dpservice
				vniAvail, err := dpdkClient.GetVni(ctx, 123, uint8(dpdk.VniType_VNI_IPV4))
				Expect(err).NotTo(HaveOccurred())
				Expect(vniAvail.Spec.InUse).To(BeTrue())
			})

			It("should reconcile successfully after delete", func() {
				// Create and initialize loadbalancer reconciler
				Expect(lbReconcile(ctx, *loadBalancer)).To(Succeed())

				// Fetch the deleted LB object from k8s
				fetchedLB := &metalnetv1alpha1.LoadBalancer{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      loadBalancer.Name,
					Namespace: loadBalancer.Namespace,
				}, fetchedLB)).ToNot(Succeed())

				// Fetch the deleted LB object from dpservice
				lb, err := dpdkClient.GetLoadBalancer(ctx, string(loadBalancer.ObjectMeta.UID))
				Expect(err).To(HaveOccurred())
				Expect(lb.Status.Code).To(Equal(uint32(dpdkerrors.NOT_FOUND)))

				// Fetch the VNI object from dpservice
				vniAvail, err := dpdkClient.GetVni(ctx, 123, uint8(dpdk.VniType_VNI_IPV4))
				Expect(err).NotTo(HaveOccurred())
				Expect(vniAvail.Spec.InUse).To(BeFalse())
			})
		})
	})
})

var _ = Describe("Negative cases", Label("negative"), func() {
	var wrongNetworkInterface *metalnetv1alpha1.NetworkInterface
	var wrongLoadBalancer *metalnetv1alpha1.LoadBalancer
	var wfr1 metalnetv1alpha1.FirewallRule

	ctx := SetupContext()
	ns := SetupTest(ctx)

	// Creates the k8s network object and runs the reconcile loop before spec
	BeforeEach(func() {
		network = &metalnetv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "negative-test-network",
				Namespace: ns.Name,
			},
			Spec: metalnetv1alpha1.NetworkSpec{
				ID:        200 + int32(GinkgoParallelProcess()),
				PeeredIDs: []int32{2, 3},
				PeeredPrefixes: []metalnetv1alpha1.PeeredPrefix{
					{
						ID:       2,
						Prefixes: []metalnetv1alpha1.IPPrefix{}, // Add desired IPPrefixes here
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, network)).To(Succeed())

		Expect(networkReconcile(ctx, *network)).To(Succeed())

		var protocolType metalnetv1alpha1.ProtocolType = "TCP"
		var srcPort int32 = 80
		var dstPort int32 = 443
		wfr1 = metalnetv1alpha1.FirewallRule{

			FirewallRuleID:    "wfr1",
			Direction:         "Egress",
			Action:            "ACCEPT",
			IpFamily:          "IPv4",
			SourcePrefix:      metalnetv1alpha1.MustParseNewIPPrefix("1.1.1.0/24"),
			DestinationPrefix: metalnetv1alpha1.MustParseNewIPPrefix("10.0.0.10/32"),
			ProtocolMatch: &metalnetv1alpha1.ProtocolMatch{
				ProtocolType: &protocolType,
				PortRange: &metalnetv1alpha1.PortMatch{
					SrcPort:    &srcPort,
					EndSrcPort: 80,
					DstPort:    &dstPort,
					EndDstPort: 443,
				},
			},
		}

		// Define a new NetworkInterface object
		wrongNetworkInterface = &metalnetv1alpha1.NetworkInterface{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wrong-network-interface",
				Namespace: ns.Name,
			},
			Spec: metalnetv1alpha1.NetworkInterfaceSpec{
				NetworkRef: corev1.LocalObjectReference{
					Name: "negative-test-network",
				},
				NodeName:   &testNode,
				IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol},
				IPs: []metalnetv1alpha1.IP{
					{
						Addr: netip.MustParseAddr("10.0.0.1"),
					},
				},
				Prefixes:            []metalnetv1alpha1.IPPrefix{metalnetv1alpha1.MustParseIPPrefix("5.5.5.5/32")},
				LoadBalancerTargets: []metalnetv1alpha1.IPPrefix{metalnetv1alpha1.MustParseIPPrefix("5.5.5.5/32")},
				//NAT:                 &nat,
				VirtualIP:     metalnetv1alpha1.MustParseNewIP("5.5.5.5"),
				FirewallRules: []metalnetv1alpha1.FirewallRule{},
			},
		}

		wrongLoadBalancer = &metalnetv1alpha1.LoadBalancer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wrong-test-loadbalancer",
				Namespace: ns.Name,
			},
			Spec: metalnetv1alpha1.LoadBalancerSpec{
				NetworkRef: corev1.LocalObjectReference{Name: "negative-test-network"},
				LBtype:     "Public",
				IPFamily:   corev1.IPv4Protocol,
				IP: metalnetv1alpha1.IP{
					Addr: netip.MustParseAddr("11.5.5.1"),
				},
				Ports: []metalnetv1alpha1.LBPort{
					{Protocol: "TCP", Port: 80},
					{Protocol: "UDP", Port: 80},
				},
				NodeName: &testNode,
			},
		}

		// Deletes the k8s network object after spec is completed
		DeferCleanup(func(ctx SpecContext) {
			Expect(k8sClient.Delete(ctx, network)).To(Succeed())
			Expect(networkReconcile(ctx, *network)).To(Succeed())
		})
	})

	When("creating a NetworkInterface with wrong data", func() {
		It("should fail", func() {
			By("unsupported direction string")
			wfr1.Direction = "XXX"
			wrongNetworkInterface.Spec.FirewallRules = []metalnetv1alpha1.FirewallRule{wfr1}

			// Create the NetworkInterface k8s object
			Expect(k8sClient.Create(ctx, wrongNetworkInterface)).To(Succeed())

			// Ensure it's created
			createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-network-interface",
			}, createdNetworkInterface)).To(Succeed())

			Expect(createdNetworkInterface.Spec.NetworkRef.Name).To(Equal("negative-test-network"))
			Expect(createdNetworkInterface.Spec.IPs[0].Addr.String()).To(Equal("10.0.0.1"))

			// Reconcile loop should fail, because of unsupported direction string
			// Error: traffic direction can be only: Ingress = 0/Egress = 1
			Expect(ifaceReconcile(ctx, *createdNetworkInterface)).ToNot(Succeed())

			// Interface should be now created in dpservice
			_, err := dpdkClient.GetInterface(ctx, string(wrongNetworkInterface.ObjectMeta.UID))
			Expect(err).ToNot(HaveOccurred())

			// dpservice FWRule object is not created during reconciliation, because of unsupported direction string
			_, err = dpdkClient.GetFirewallRule(ctx, "wfr1", string(wrongNetworkInterface.ObjectMeta.UID))
			Expect(err).To(HaveOccurred())

			// Delete the NetworkInterface object from k8s
			Expect(k8sClient.Delete(ctx, wrongNetworkInterface)).To(Succeed())
			// and reconcile
			Expect(ifaceReconcile(ctx, *wrongNetworkInterface)).To(Succeed())
		})
	})

	When("creating a NetworkInterface with wrong data", func() {
		It("should fail", func() {
			By("unsupported action string")
			wfr1.Action = "XXX"
			wrongNetworkInterface.Spec.FirewallRules = []metalnetv1alpha1.FirewallRule{wfr1}

			// Create the NetworkInterface k8s object
			Expect(k8sClient.Create(ctx, wrongNetworkInterface)).To(Succeed())

			// Ensure it's created
			createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-network-interface",
			}, createdNetworkInterface)).To(Succeed())

			Expect(createdNetworkInterface.Spec.NetworkRef.Name).To(Equal("negative-test-network"))
			Expect(createdNetworkInterface.Spec.IPs[0].Addr.String()).To(Equal("10.0.0.1"))

			// Reconcile loop should fail, because of unsupported action string
			// Error: action can be only: Allow = 0/Drop = 1
			Expect(ifaceReconcile(ctx, *createdNetworkInterface)).ToNot(Succeed())

			// Interface should be now created in dpservice
			_, err := dpdkClient.GetInterface(ctx, string(wrongNetworkInterface.ObjectMeta.UID))
			Expect(err).ToNot(HaveOccurred())

			// dpservice FWRule object is not created during reconciliation, because of unsupported direction string
			_, err = dpdkClient.GetFirewallRule(ctx, "wfr1", string(wrongNetworkInterface.ObjectMeta.UID))
			Expect(err).To(HaveOccurred())

			// Delete the NetworkInterface object from k8s
			Expect(k8sClient.Delete(ctx, wrongNetworkInterface)).To(Succeed())
			// and reconcile
			Expect(ifaceReconcile(ctx, *wrongNetworkInterface)).To(Succeed())

		})
	})

	When("creating a NetworkInterface with wrong data", func() {
		It("should fail", func() {
			By("fw rule src port out of range")
			wfr1.ProtocolMatch.PortRange.EndSrcPort = 75000
			wrongNetworkInterface.Spec.FirewallRules = []metalnetv1alpha1.FirewallRule{wfr1}

			// Try to create the NetworkInterface k8s object
			Expect(k8sClient.Create(ctx, wrongNetworkInterface)).ToNot(Succeed())

			// Ensure it's not created
			createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-network-interface",
			}, createdNetworkInterface)).ToNot(Succeed())
		})
	})

	When("creating a NetworkInterface with wrong data", func() {
		It("should fail", func() {
			By("fw rule src port is lower than -1")
			srcPort := int32(-10)
			wfr1.ProtocolMatch.PortRange.SrcPort = &srcPort
			wrongNetworkInterface.Spec.FirewallRules = []metalnetv1alpha1.FirewallRule{wfr1}

			// Try to create the NetworkInterface k8s object
			Expect(k8sClient.Create(ctx, wrongNetworkInterface)).ToNot(Succeed())

			// Ensure it's not created
			createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-network-interface",
			}, createdNetworkInterface)).ToNot(Succeed())
		})
	})

	When("creating a NetworkInterface with wrong data", func() {
		It("should fail", func() {
			By("fw rule DstPortEnd lower than DstPort ")
			// wfr1.ProtocolMatch.PortRange.DstPort is 443
			wfr1.ProtocolMatch.PortRange.EndDstPort = 80
			wrongNetworkInterface.Spec.FirewallRules = []metalnetv1alpha1.FirewallRule{wfr1}

			// Create the NetworkInterface k8s object
			Expect(k8sClient.Create(ctx, wrongNetworkInterface)).To(Succeed())

			// Ensure it's created
			createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-network-interface",
			}, createdNetworkInterface)).To(Succeed())

			Expect(createdNetworkInterface.Spec.NetworkRef.Name).To(Equal("negative-test-network"))
			Expect(createdNetworkInterface.Spec.IPs[0].Addr.String()).To(Equal("10.0.0.1"))

			// Reconcile loop should fail, because of end port is lower than start
			Expect(ifaceReconcile(ctx, *createdNetworkInterface)).ToNot(Succeed())

			// Interface should be now created in dpservice
			_, err := dpdkClient.GetInterface(ctx, string(wrongNetworkInterface.ObjectMeta.UID))
			Expect(err).ToNot(HaveOccurred())

			// dpservice FWRule object is not created during reconciliation, because of port issue
			_, err = dpdkClient.GetFirewallRule(ctx, "wfr1", string(wrongNetworkInterface.ObjectMeta.UID))
			Expect(err).To(HaveOccurred())

			// Delete the NetworkInterface object from k8s
			Expect(k8sClient.Delete(ctx, wrongNetworkInterface)).To(Succeed())
			// and reconcile
			Expect(ifaceReconcile(ctx, *wrongNetworkInterface)).To(Succeed())

		})
	})

	When("creating a NetworkInterface with wrong data", func() {
		It("should fail", func() {
			By("negative icmp type")
			var protocolType metalnetv1alpha1.ProtocolType = metalnetv1alpha1.FirewallRuleProtocolTypeICMP
			var icmpType = int32(-5)
			var icmpCode = int32(0)
			wfr1.ProtocolMatch = &metalnetv1alpha1.ProtocolMatch{
				ProtocolType: &protocolType,
				ICMP: &metalnetv1alpha1.ICMPMatch{
					IcmpType: &icmpType,
					IcmpCode: &icmpCode,
				},
			}

			wrongNetworkInterface.Spec.FirewallRules = []metalnetv1alpha1.FirewallRule{wfr1}

			// Try to create the NetworkInterface k8s object
			Expect(k8sClient.Create(ctx, wrongNetworkInterface)).ToNot(Succeed())

			// Ensure it's not created
			createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-network-interface",
			}, createdNetworkInterface)).ToNot(Succeed())
		})
	})

	When("creating a NetworkInterface with wrong data", func() {
		It("should fail", func() {
			By("EndPort lower than StartPort in NAT config")
			nat := metalnetv1alpha1.NATDetails{
				IP:      metalnetv1alpha1.MustParseNewIP("7.7.7.7"),
				Port:    100,
				EndPort: 10,
			}
			wrongNetworkInterface.Spec.NAT = &nat
			// Create the NetworkInterface k8s object
			Expect(k8sClient.Create(ctx, wrongNetworkInterface)).To(Succeed())

			// Ensure it's created
			createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-network-interface",
			}, createdNetworkInterface)).To(Succeed())

			Expect(createdNetworkInterface.Spec.NetworkRef.Name).To(Equal("negative-test-network"))
			Expect(createdNetworkInterface.Spec.IPs[0].Addr.String()).To(Equal("10.0.0.1"))

			// Reconcile loop should fail, because of wrong nat port
			Expect(ifaceReconcile(ctx, *createdNetworkInterface)).ToNot(Succeed())

			// Interface should be now created in dpservice
			_, err := dpdkClient.GetInterface(ctx, string(wrongNetworkInterface.ObjectMeta.UID))
			Expect(err).ToNot(HaveOccurred())

			// dpservice FWRule object is not created during reconciliation, because of wrong nat port
			_, err = dpdkClient.GetFirewallRule(ctx, "wfr1", string(wrongNetworkInterface.ObjectMeta.UID))
			Expect(err).To(HaveOccurred())

			// Delete the NetworkInterface object from k8s
			Expect(k8sClient.Delete(ctx, wrongNetworkInterface)).To(Succeed())
			// and reconcile
			Expect(ifaceReconcile(ctx, *wrongNetworkInterface)).To(Succeed())
		})
	})

	When("creating a NetworkInterface with wrong data", func() {
		It("should fail", func() {
			By("StartPort in NAT config is negative")
			nat := metalnetv1alpha1.NATDetails{
				IP:      metalnetv1alpha1.MustParseNewIP("7.7.7.7"),
				Port:    -1,
				EndPort: 80,
			}
			wrongNetworkInterface.Spec.NAT = &nat
			// Try to create the NetworkInterface k8s object
			Expect(k8sClient.Create(ctx, wrongNetworkInterface)).ToNot(Succeed())

			// Ensure it's not created
			createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-network-interface",
			}, createdNetworkInterface)).ToNot(Succeed())
		})
	})

	When("creating a NetworkInterface with wrong data", func() {
		It("should fail", func() {
			By("EndPort in NAT config is out of range")
			nat := metalnetv1alpha1.NATDetails{
				IP:      metalnetv1alpha1.MustParseNewIP("7.7.7.7"),
				Port:    80,
				EndPort: 65536,
			}
			wrongNetworkInterface.Spec.NAT = &nat
			// Try to create the NetworkInterface k8s object
			Expect(k8sClient.Create(ctx, wrongNetworkInterface)).ToNot(Succeed())

			// Ensure it's not created
			createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-network-interface",
			}, createdNetworkInterface)).ToNot(Succeed())
		})
	})

	When("creating a NetworkInterface with wrong data", func() {
		It("should fail", func() {
			By("empty IP slice")
			wrongNetworkInterface.Spec.IPs = []metalnetv1alpha1.IP{}
			// Try to create the NetworkInterface k8s object
			Expect(k8sClient.Create(ctx, wrongNetworkInterface)).ToNot(Succeed())

			// Ensure it's not created
			createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-network-interface",
			}, createdNetworkInterface)).ToNot(Succeed())
		})
	})

	When("creating a Loadbalancer with wrong data", Label("lb"), func() {
		It("should fail", func() {
			By("TCP port is negative")
			// Set port to be negative
			wrongLoadBalancer.Spec.Ports[0].Port = -1

			// Try to create the LoadBalancer object in k8s
			Expect(k8sClient.Create(ctx, wrongLoadBalancer)).ToNot(Succeed())

			// Ensure it's not created
			createdLB := &metalnetv1alpha1.LoadBalancer{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-test-loadbalancer",
			}, createdLB)).ToNot(Succeed())
		})
	})

	When("creating a Loadbalancer with wrong data", Label("lb"), func() {
		It("should fail", func() {
			By("UDP port out of range")
			// Set port to be put of range
			wrongLoadBalancer.Spec.Ports[1].Port = 75000

			// Try to create the LoadBalancer object in k8s
			Expect(k8sClient.Create(ctx, wrongLoadBalancer)).ToNot(Succeed())

			// Ensure it's not created
			createdLB := &metalnetv1alpha1.LoadBalancer{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-test-loadbalancer",
			}, createdLB)).ToNot(Succeed())
		})
	})

	When("creating a Loadbalancer with wrong data", Label("lb"), func() {
		It("should fail", func() {
			By("wrong protocol type")
			// Set protocol to wrong value
			wrongLoadBalancer.Spec.Ports[0].Protocol = "XXX"

			// Create the LoadBalancer object in k8s
			Expect(k8sClient.Create(ctx, wrongLoadBalancer)).To(Succeed())

			// Ensure it's created
			createdLB := &metalnetv1alpha1.LoadBalancer{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-test-loadbalancer",
			}, createdLB)).To(Succeed())

			Expect(lbReconcile(ctx, *wrongLoadBalancer)).ToNot(Succeed())

			Expect(k8sClient.Delete(ctx, wrongLoadBalancer)).To(Succeed())

			Expect(lbReconcile(ctx, *wrongLoadBalancer)).To(Succeed())
		})
	})

	When("creating a Loadbalancer with wrong data", Label("lb"), func() {
		It("should fail", func() {
			By("using IPv6 address")
			// set wrong IP (IPv6 is not supported yet)
			wrongLoadBalancer.Spec.IP = metalnetv1alpha1.IP{
				Addr: netip.MustParseAddr("ff80::1"),
			}

			// Create the LoadBalancer object in k8s
			Expect(k8sClient.Create(ctx, wrongLoadBalancer)).To(Succeed())

			// Ensure it's created
			createdLB := &metalnetv1alpha1.LoadBalancer{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-test-loadbalancer",
			}, createdLB)).To(Succeed())

			Expect(lbReconcile(ctx, *wrongLoadBalancer)).ToNot(Succeed())

			Expect(k8sClient.Delete(ctx, wrongLoadBalancer)).To(Succeed())

			Expect(lbReconcile(ctx, *wrongLoadBalancer)).To(Succeed())
		})
	})

	When("creating a Loadbalancer with wrong data", Label("lb"), func() {
		It("should fail", func() {
			By("no LB ports specified")
			// empty LB ports
			wrongLoadBalancer.Spec.Ports = []metalnetv1alpha1.LBPort{}

			// Try to create the LoadBalancer object in k8s
			Expect(k8sClient.Create(ctx, wrongLoadBalancer)).ToNot(Succeed())

			// Ensure it's not created
			createdLB := &metalnetv1alpha1.LoadBalancer{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-test-loadbalancer",
			}, createdLB)).ToNot(Succeed())
		})
	})

	When("creating a Loadbalancer with wrong data", func() {
		It("should fail", func() {
			By("wrong network reference")
			// change the network reference to non existent network
			wrongLoadBalancer.Spec.NetworkRef = corev1.LocalObjectReference{Name: "non-existent-test-network"}

			// Create the LoadBalancer object in k8s
			Expect(k8sClient.Create(ctx, wrongLoadBalancer)).To(Succeed())

			// Ensure it's created
			createdLB := &metalnetv1alpha1.LoadBalancer{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-test-loadbalancer",
			}, createdLB)).To(Succeed())

			// TODO: When creating LB object via yaml file, it crashes the metalnet
			// but it works in automated tests
			Expect(lbReconcile(ctx, *wrongLoadBalancer)).To(Succeed())

			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-test-loadbalancer",
			}, createdLB)).To(Succeed())
			Expect(createdLB.Status.State).To(Equal(metalnetv1alpha1.LoadBalancerStatePending))

			/* nonexistentNetwork := &metalnetv1alpha1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-existent-test-network",
					Namespace: ns.Name,
				},
				Spec: metalnetv1alpha1.NetworkSpec{
					ID:        456,
					PeeredIDs: []int32{10},
					PeeredPrefixes: []metalnetv1alpha1.PeeredPrefix{
						{
							ID:       10,
							Prefixes: []metalnetv1alpha1.IPPrefix{}, // Add desired IPPrefixes here
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, nonexistentNetwork)).To(Succeed())

			Expect(networkReconcile(ctx, *nonexistentNetwork)).To(Succeed())

			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-test-loadbalancer",
			}, createdLB)).To(Succeed())
			Expect(createdLB.Status.State).To(Equal(metalnetv1alpha1.LoadBalancerStateReady))
			*/
			Expect(k8sClient.Delete(ctx, wrongLoadBalancer)).To(Succeed())

			Expect(lbReconcile(ctx, *wrongLoadBalancer)).To(Succeed())
		})
	})

	// TODO: fix the conversion of PeeredIDs to uint32 and remove Pending
	When("creating a Network with wrong data", Pending, func() {
		It("should fail", func() {
			By("negative peered ID")
			// Define a new Network object
			wrongNetwork := &metalnetv1alpha1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wrong-test-network",
					Namespace: ns.Name,
				},
				Spec: metalnetv1alpha1.NetworkSpec{
					ID:        123,
					PeeredIDs: []int32{-5, 10},
					PeeredPrefixes: []metalnetv1alpha1.PeeredPrefix{
						{
							ID:       -2,
							Prefixes: []metalnetv1alpha1.IPPrefix{}, // Add desired IPPrefixes here
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, wrongNetwork)).To(Succeed())

			Expect(networkReconcile(ctx, *wrongNetwork)).ToNot(Succeed())

			// Ensure it's created
			createdNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "wrong-test-network",
			}, createdNetwork)).To(Succeed())

			Expect(k8sClient.Delete(ctx, wrongNetwork)).To(Succeed())

			Expect(networkReconcile(ctx, *wrongNetwork)).To(Succeed())
		})
	})
})

func networkReconcile(ctx context.Context, network metalnetv1alpha1.Network) error {
	// error location will always be in the spec that called the helper, and not the helper itself
	GinkgoHelper()

	reconciler := &NetworkReconciler{
		Client:            k8sClient,
		DPDK:              dpdkClient,
		RouteUtil:         metalbondRouteUtil,
		MetalnetCache:     metalnetCache,
		MetalnetMBClient:  metalnetMBClient,
		DefaultRouterAddr: &defaultRouterAddr,
		NodeName:          testNode,
	}

	// Loop the reconciler until Requeue is false or error occurs
	for {
		res, err := reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      network.Name,
				Namespace: network.Namespace,
			},
		})
		if err != nil {
			fmt.Println(err)
			return err
		}

		if res.Requeue == false {
			break
		}
	}
	return nil
}

func ifaceReconcile(ctx context.Context, networkInterface metalnetv1alpha1.NetworkInterface) error {
	// error location will always be in the spec that called the helper, and not the helper itself
	GinkgoHelper()

	// Create and initialize Network Interface reconciler
	reconciler := &NetworkInterfaceReconciler{
		Client:        k8sClient,
		EventRecorder: &record.FakeRecorder{},
		DPDK:          dpdkClient,
		RouteUtil:     metalbondRouteUtil,
		NodeName:      testNode,
		NetFnsManager: netFnsManager,
		PublicVNI:     100,
	}

	// Loop the reconciler until Requeue is false or error occurs
	for {
		res, err := reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      networkInterface.Name,
				Namespace: networkInterface.Namespace,
			},
		})
		if err != nil {
			fmt.Println(err)
			return err
		}

		if res.Requeue == false {
			break
		}
	}
	return nil
}

func lbReconcile(ctx context.Context, loadBalancer metalnetv1alpha1.LoadBalancer) error {
	// error location will always be in the spec that called the helper, and not the helper itself
	GinkgoHelper()

	reconciler := &LoadBalancerReconciler{
		Client:        k8sClient,
		EventRecorder: &record.FakeRecorder{},
		DPDK:          dpdkClient,
		RouteUtil:     metalbondRouteUtil,
		MetalnetCache: metalnetCache,
		NodeName:      testNode,
		PublicVNI:     100,
	}

	// Loop the reconciler until Requeue is false or error occurs
	for {
		res, err := reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      loadBalancer.Name,
				Namespace: loadBalancer.Namespace,
			},
		})
		if err != nil {
			fmt.Println(err)
			return err
		}

		if res.Requeue == false {
			break
		}
	}
	return nil
}
