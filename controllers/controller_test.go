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
	networkingv1alpha1 "github.com/onmetal/metalnet/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Network Controller", func() {
	var loadBalancer *networkingv1alpha1.LoadBalancer
	ctx := SetupContext()
	ns := SetupTest(ctx)

	Context("When creating a Network", func() {
		It("Should create successfully", func() {
			// Define a new Network object
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

			// Create the Network object
			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			// Ensure it's created
			createdNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "test-network",
			}, createdNetwork)).Should(Succeed())

			Expect(createdNetwork.Spec.ID).To(Equal(int32(123)))

			vniAvail, err := dpdkClient.GetVni(ctx, 123, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(vniAvail.Spec.InUse).To(BeFalse())
		})

		It("Network should reconcile successfully", func() {
			// Create and initialize network reconciler
			reconciler := &NetworkReconciler{
				Client:        k8sClient,
				DPDK:          dpdkClient,
				Metalbond:     metalbondClient,
				MBInternal:    mbClient,
				RouterAddress: netip.MustParseAddr("::1"),
				NodeName:      testNode,
			}

			// Call the reconciler
			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      network.Name,
					Namespace: network.Namespace,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			// Fetch the updated Network object from the cluster
			fetchedNetwork := &metalnetv1alpha1.Network{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Name:      network.Name,
				Namespace: network.Namespace,
			}, fetchedNetwork)).Should(Succeed())
		})
	})

	Context("When creating a NetworkInterface", func() {
		It("Should create successfully", func() {
			// Define a new NetworkInterface object
			networkInterface := &metalnetv1alpha1.NetworkInterface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-network-interface",
					Namespace: ns.Name,
				},
				Spec: metalnetv1alpha1.NetworkInterfaceSpec{
					NetworkRef: corev1.LocalObjectReference{
						Name: "test-network",
					},
					IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol},
					IPs: []metalnetv1alpha1.IP{
						{
							Addr: netip.MustParseAddr("10.0.0.1"),
						},
					},
				},
			}

			// Create the NetworkInterface object
			Expect(k8sClient.Create(ctx, networkInterface)).Should(Succeed())

			// Ensure it's created
			createdNetworkInterface := &metalnetv1alpha1.NetworkInterface{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "test-network-interface",
			}, createdNetworkInterface)).Should(Succeed())

			Expect(createdNetworkInterface.Spec.NetworkRef.Name).To(Equal("test-network"))
			Expect(createdNetworkInterface.Spec.IPs[0].Addr.String()).To(Equal("10.0.0.1"))
			//Expect(createdNetworkInterface.Status.State).To(Equal(metalnetv1alpha1.NetworkInterfaceStateReady))
		})
	})

	Context("When creating a Loadbalancer", func() {
		It("Network of it should create successfully", func() {
			// Define a new Network object
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
			// Create the Network object
			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

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

			// Create the LoadBalancer object
			Expect(k8sClient.Create(ctx, loadBalancer)).Should(Succeed())

			// Ensure it's created
			createdLB := &metalnetv1alpha1.LoadBalancer{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: ns.Name,
				Name:      "test-loadbalancer",
			}, createdLB)).Should(Succeed())

			Expect(createdLB.Spec.NetworkRef.Name).To(Equal("test-network"))
			Expect(createdLB.Spec.IP.Addr.String()).To(Equal("11.5.5.1"))
		})

		It("LoadBalancer should reconcile successfully", func() {
			// Create and initialize loadbalancer reconciler
			reconciler := &LoadBalancerReconciler{
				Client:        k8sClient,
				EventRecorder: &record.FakeRecorder{},
				DPDK:          dpdkClient,
				MBInternal:    mbClient,
				Metalbond:     metalbondClient,
				NodeName:      testNode,
				PublicVNI:     100,
			}

			// Call the reconciler
			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      loadBalancer.Name,
					Namespace: loadBalancer.Namespace,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			// Call the reconciler again (Due to the finalizer. Maybe better way to do this ? )
			_, err = reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      loadBalancer.Name,
					Namespace: loadBalancer.Namespace,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			// Fetch the updated LB object from the cluster
			fetchedLB := &metalnetv1alpha1.LoadBalancer{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Name:      loadBalancer.Name,
				Namespace: loadBalancer.Namespace,
			}, fetchedLB)).Should(Succeed())

			Expect(fetchedLB.Status.State).To(Equal(metalnetv1alpha1.LoadBalancerStateReady))

			vniAvail, err := dpdkClient.GetVni(ctx, 123, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(vniAvail.Spec.InUse).To(BeTrue())

			// Another reconcilition of network object is needed here. Because we dont have event watches in the test environment.
		})
	})
})
