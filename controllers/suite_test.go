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
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	mb "github.com/onmetal/metalbond"
	networkingv1alpha1 "github.com/onmetal/metalnet/api/v1alpha1"
	"github.com/onmetal/metalnet/dpdkmetalbond"
	"github.com/onmetal/metalnet/metalbond"
	"github.com/onmetal/metalnet/netfns"
	dpdkclient "github.com/onmetal/net-dpservice-go/client"
	dpdkproto "github.com/onmetal/net-dpservice-go/proto"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg             *rest.Config
	k8sClient       client.Client
	testEnv         *envtest.Environment
	network         *networkingv1alpha1.Network
	ctxCancel       context.CancelFunc
	ctxGrpc         context.Context
	dpserviceAddr   string = "127.0.0.1:1337"
	testNode        string = "testNode"
	metalnetDir     string = "/var/lib/metalnet"
	netFnsManager   *netfns.Manager
	conn            *grpc.ClientConn
	dpdkProtoClient dpdkproto.DPDKonmetalClient
	dpdkClient      dpdkclient.Client
	metalbondClient metalbond.Client
	mbClient        dpdkmetalbond.MbInternalAccess
)

// This assumes running metalbond server and dp-service on the same localhost of this test suite
// metalbond server --listen [::1]:4711 --http [::1]:4712 --keepalive 3
// /test/dp_service.py --no-init

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = networkingv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Setup TAP File claim store
	claimStore, err := netfns.NewFileClaimStore(filepath.Join(metalnetDir, "netfns", "claims"), true)
	Expect(err).NotTo(HaveOccurred())

	initAvailable, err := netfns.CollectTAPFunctions([]string{"net_tap4", "net_tap5"})
	Expect(err).NotTo(HaveOccurred())

	netFnsManager, err = netfns.NewManager(claimStore, initAvailable)
	Expect(err).NotTo(HaveOccurred())

	// setup net-dpservice client
	ctxGrpc, ctxCancel = context.WithTimeout(context.Background(), 100*time.Millisecond)

	conn, err = grpc.DialContext(ctxGrpc, dpserviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	Expect(err).NotTo(HaveOccurred())

	dpdkProtoClient = dpdkproto.NewDPDKonmetalClient(conn)
	dpdkClient = dpdkclient.NewClient(dpdkProtoClient)

	_, err = dpdkClient.Initialize(context.TODO())
	Expect(err).NotTo(HaveOccurred())

	//setup metalbond client
	config := mb.Config{
		KeepaliveInterval: 3,
	}

	opts := zap.Options{
		Development: true,
	}
	logger := zap.New(zap.UseFlagOptions(&opts))
	mbClient, err = dpdkmetalbond.NewClient(&logger, dpdkClient, dpdkmetalbond.ClientOptions{
		IPv4Only:         true,
		PreferredNetwork: nil,
	})
	Expect(err).NotTo(HaveOccurred())

	mbInstance := mb.NewMetalBond(config, mbClient)
	metalbondClient = metalbond.NewClient(mbInstance)

	err = mbInstance.AddPeer("[::1]:4711", "")
	Expect(err).NotTo(HaveOccurred())
})

// SetupTest returns a namespace which will be created before each ginkgo Container Node and deleted at the end of their Closures
// so that each test case can run in an independent way
func SetupTest(ctx context.Context) *corev1.Namespace {
	var (
		ns = &corev1.Namespace{}
	)

	BeforeEach(func() {
		*ns = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "testns-",
				Labels:       map[string]string{"test": "ns"},
			},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed(), "failed to create test namespace")

	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, ns)).To(Succeed(), "failed to delete test namespace")
	})

	return ns
}

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
	errConn := conn.Close()
	Expect(errConn).NotTo(HaveOccurred())
	ctxCancel()
})
