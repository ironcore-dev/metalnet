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

	. "github.com/onmetal/onmetal-api/utils/testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metalnetv1alpha1 "github.com/onmetal/metalnet/api/v1alpha1"
	dpdk "github.com/onmetal/net-dpservice-go/proto"
	corev1 "k8s.io/api/core/v1"
)

// TODO:	'It' specs should be independent, now they are correct because inner spec run in order (to change it use 'ginkgo --randomize-all')
//			can be fixed by creating single 'It' or by 'Ordered containers'

var _ = Describe("Network Controller", Label("network"), Ordered, func() {
	ctx := SetupContext()
	ns := SetupTest(ctx)

	Context("When creating a Network", Ordered, func() {
		It("should create successfully", func() {
			fmt.Println("##### network create")
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

			// Ensure it's created
			createdNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "test-network",
			}, createdNetwork)).To(Succeed())

			// Equal() uses reflect.DeepEqual to compare (it compares whole structs recursively)
			Expect(createdNetwork).To(Equal(network))
			//Expect(createdNetwork.Spec.ID).To(Equal(network.Spec.ID))

			vniAvail, err := dpdkClient.GetVni(ctx, 123, uint8(dpdk.VniType_VNI_IPV4))
			Expect(err).NotTo(HaveOccurred())
			Expect(vniAvail.Spec.InUse).To(BeFalse())
		})

		It("should reconcile successfully", func() {
			fmt.Println("##### network reconcile")
			// Create and initialize network reconciler
			networkReconcile(ctx, *network)

			// Fetch the updated Network object from k8s
			fetchedNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Name:      network.Name,
				Namespace: network.Namespace,
			}, fetchedNetwork)).To(Succeed())

			Expect(fetchedNetwork.Spec.ID).To(Equal(int32(123)))
		})

		It("should fail when already existing", func() {
			fmt.Println("##### network create again")
			Expect(k8sClient.Create(ctx, network)).ToNot(Succeed())
		})

		It("should update successfully", func() {
			fmt.Println("##### network update")

			patchNetwork := network.DeepCopy()
			patchNetwork.Spec.PeeredIDs = []int32{4, 5}
			Expect(k8sClient.Patch(ctx, patchNetwork, client.MergeFrom(network))).To(Succeed())

			networkReconcile(ctx, *network)

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
			fmt.Println("##### network delete")
			// Delete the Network object
			Expect(k8sClient.Delete(ctx, network)).To(Succeed())

			// Ensure it's deleted
			deletedNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "test-network",
			}, deletedNetwork)).ToNot(Succeed())

			vniAvail, err := dpdkClient.GetVni(ctx, 123, uint8(dpdk.VniType_VNI_IPV4))
			Expect(err).NotTo(HaveOccurred())
			Expect(vniAvail.Spec.InUse).To(BeFalse())
		})

		It("should reconcile successfully after delete", func() {
			fmt.Println("##### network reconcile delete")
			// Create and initialize network reconciler
			networkReconcile(ctx, *network)

			// Fetch the updated Network object from k8s
			fetchedNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Name:      network.Name,
				Namespace: network.Namespace,
			}, fetchedNetwork)).ToNot(Succeed())
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
		fmt.Println("##### beforeeach network create", ns.Name)
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

		networkReconcile(ctx, *network)

		// Deletes the k8s network object after spec is completed
		DeferCleanup(func(ctx SpecContext) {
			fmt.Println("##### defer network delete")
			Expect(k8sClient.Delete(ctx, network)).To(Succeed())
			networkReconcile(ctx, *network)
		})
	})

	Context("NetworkInterface", Label("interface"), Ordered, func() {
		When("creating a NetworkInterface", Ordered, func() {
			It("should create successfully", func() {
				fmt.Println("##### int create", ns.Name)
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

				// Create the NetworkInterface object
				Expect(k8sClient.Create(ctx, networkInterface)).To(Succeed())

				// Ensure it's created
				createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Namespace: ns.Name,
					Name:      "test-network-interface",
				}, createdNetworkInterface)).To(Succeed())

				Expect(createdNetworkInterface.Spec.NetworkRef.Name).To(Equal("test-network"))
				Expect(createdNetworkInterface.Spec.IPs[0].Addr.String()).To(Equal("10.0.0.1"))
			})

			It("should reconcile successfully", func() {
				fmt.Println("##### int reconcile")

				// Create and initialize Network Interface reconciler
				reconciler := &NetworkInterfaceReconciler{
					Client:        k8sClient,
					EventRecorder: &record.FakeRecorder{},
					DPDK:          dpdkClient,
					Metalbond:     metalbondClient,
					NodeName:      testNode,
					NetFnsManager: netFnsManager,
					PublicVNI:     100,
				}

				// Loop the reconciler until Requeue is false
				for {
					res, err := reconciler.Reconcile(ctx, ctrl.Request{
						NamespacedName: types.NamespacedName{
							Name:      networkInterface.Name,
							Namespace: networkInterface.Namespace,
						},
					})
					Expect(err).ToNot(HaveOccurred())

					if res.Requeue == false {
						break
					}
				}

				// Fetch the updated Iface object from k8s
				fetchedIface := &metalnetv1alpha1.NetworkInterface{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      networkInterface.Name,
					Namespace: networkInterface.Namespace,
				}, fetchedIface)).To(Succeed())
				Expect(fetchedIface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))

				// Fetch the Iface object from dpservice
				_, err := dpdkClient.GetInterface(ctx, string(networkInterface.ObjectMeta.UID))
				Expect(err).ToNot(HaveOccurred())

				// Fetch the VNI object from dpservice
				vniAvail, err := dpdkClient.GetVni(ctx, 123, uint8(dpdk.VniType_VNI_IPV4))
				Expect(err).NotTo(HaveOccurred())
				Expect(vniAvail.Spec.InUse).To(BeTrue())

				Expect(k8sClient.Delete(ctx, fetchedIface)).To(Succeed())
				// Loop the reconciler until Requeue is false
				for {
					res, err := reconciler.Reconcile(ctx, ctrl.Request{
						NamespacedName: types.NamespacedName{
							Name:      networkInterface.Name,
							Namespace: networkInterface.Namespace,
						},
					})
					Expect(err).ToNot(HaveOccurred())

					if res.Requeue == false {
						break
					}
				}
			})
		})
	})

	Context("Loadbalancer", Label("lb"), Ordered, func() {
		When("creating a Loadbalancer", func() {
			It("should create successfully", func() {
				fmt.Println("##### lb create", ns.Name)
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
				fmt.Println("##### lb create again", ns.Name)
				Expect(k8sClient.Create(ctx, loadBalancer)).ToNot(Succeed())
			})

			It("should reconcile successfully", func() {
				fmt.Println("##### lb reconcile", ns.Name)
				// Create and initialize loadbalancer reconciler
				lbReconcile(ctx, *loadBalancer)

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
				fmt.Println("##### lb update", ns.Name)
				patchLB := loadBalancer.DeepCopy()
				patchLB.Spec.IP = metalnetv1alpha1.IP{
					Addr: netip.MustParseAddr("11.5.5.2"),
				}
				Expect(k8sClient.Patch(ctx, patchLB, client.MergeFrom(loadBalancer))).To(Succeed())

				lbReconcile(ctx, *loadBalancer)

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
				fmt.Println("##### lb delete", ns.Name)
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
				fmt.Println("##### lb rec delete", ns.Name)
				// Create and initialize loadbalancer reconciler
				lbReconcile(ctx, *loadBalancer)

				// Fetch the deleted LB object from k8s
				fetchedLB := &metalnetv1alpha1.LoadBalancer{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      loadBalancer.Name,
					Namespace: loadBalancer.Namespace,
				}, fetchedLB)).ToNot(Succeed())

				// Fetch the deleted LB object from dpservice
				_, err := dpdkClient.GetLoadBalancer(ctx, string(loadBalancer.ObjectMeta.UID))
				Expect(err).To(HaveOccurred())

				// Fetch the VNI object from dpservice
				vniAvail, err := dpdkClient.GetVni(ctx, 123, uint8(dpdk.VniType_VNI_IPV4))
				Expect(err).NotTo(HaveOccurred())
				Expect(vniAvail.Spec.InUse).To(BeFalse())
			})
		})
	})
})

func networkReconcile(ctx context.Context, network metalnetv1alpha1.Network) {
	reconciler := &NetworkReconciler{
		Client:        k8sClient,
		DPDK:          dpdkClient,
		Metalbond:     metalbondClient,
		MBInternal:    mbClient,
		RouterAddress: netip.MustParseAddr("::1"),
		NodeName:      testNode,
	}

	// Loop the reconciler until Requeue is false
	for {
		res, err := reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      network.Name,
				Namespace: network.Namespace,
			},
		})
		Expect(err).ToNot(HaveOccurred())
		if res.Requeue == false {
			break
		}
	}
}

func lbReconcile(ctx context.Context, loadBalancer metalnetv1alpha1.LoadBalancer) {
	reconciler := &LoadBalancerReconciler{
		Client:        k8sClient,
		EventRecorder: &record.FakeRecorder{},
		DPDK:          dpdkClient,
		MBInternal:    mbClient,
		Metalbond:     metalbondClient,
		NodeName:      testNode,
		PublicVNI:     100,
	}

	// Loop the reconciler until Requeue is false
	for {
		res, err := reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      loadBalancer.Name,
				Namespace: loadBalancer.Namespace,
			},
		})
		Expect(err).ToNot(HaveOccurred())

		if res.Requeue == false {
			break
		}
	}
}
