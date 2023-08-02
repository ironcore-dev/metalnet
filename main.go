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

package main

import (
	"context"
	goflag "flag"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"time"

	flag "github.com/spf13/pflag"

	metalnetclient "github.com/onmetal/metalnet/client"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/onmetal/metalnet/dpdkmetalbond"
	"github.com/onmetal/metalnet/metalbond"
	"github.com/onmetal/metalnet/netfns"
	"github.com/onmetal/metalnet/sysfs"

	mb "github.com/onmetal/metalbond"
	dpdk "github.com/onmetal/net-dpservice-go/api"
	dpdkclient "github.com/onmetal/net-dpservice-go/client"
	dpdkproto "github.com/onmetal/net-dpservice-go/proto"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	networkingv1alpha1 "github.com/onmetal/metalnet/api/v1alpha1"
	"github.com/onmetal/metalnet/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme       = runtime.NewScheme()
	setupLog     = ctrl.Log.WithName("setup")
	hostName, _  = os.Hostname()
	buildVersion string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(networkingv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var nodeName string
	var dpserviceAddr string
	var metalbondPeers []string
	var metalbondDebug bool
	var routerAddress net.IP
	var publicVNI int
	var metalnetDir string
	var preferNetwork string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&nodeName, "node-name", hostName, "The node name to react to when reconciling network interfaces.")
	flag.StringVar(&dpserviceAddr, "dp-service-address", "127.0.0.1:1337", "The address of net-dpservice.")
	flag.StringSliceVar(&metalbondPeers, "metalbond-peer", nil, "The addresses of the metalbond peers.")
	flag.BoolVar(&metalbondDebug, "metalbond-debug", false, "Enable metalbond debug.")
	flag.IPVar(&routerAddress, "router-address", net.IP{}, "The address of the next router.")
	flag.IntVar(&publicVNI, "public-vni", 100, "Virtual network identifier used for public routing announcements.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&metalnetDir, "metalnet-dir", "/var/lib/metalnet", "Directory to store metalnet data at.")
	flag.StringVar(&preferNetwork, "prefer-network", "", "Prefer network routes (e.g. 2001:db8::1/52)")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(goflag.CommandLine)
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	if metalbondDebug {
		log.SetLevel(log.DebugLevel)
	}

	if routerAddress.Equal(net.IP{}) {
		setupLog.Error(fmt.Errorf("must specify --router-address"), "invalid flags")
		os.Exit(1)
	}

	sysFS, err := sysfs.NewDefaultFS()
	if err != nil {
		setupLog.Error(err, "error creating sysfs")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       fmt.Sprintf("%s.metalnet.onmetal.de", nodeName),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	claimStore, err := netfns.NewFileClaimStore(filepath.Join(metalnetDir, "netfns", "claims"))
	if err != nil {
		setupLog.Error(err, "unable to create claim store")
		os.Exit(1)
	}

	initAvailable, err := netfns.CollectVirtualFunctions(sysFS)
	if err != nil {
		setupLog.Error(err, "unable to collect virtual functions")
		os.Exit(1)
	}

	netFnsManager, err := netfns.NewManager(claimStore, initAvailable)
	if err != nil {
		setupLog.Error(err, "unable to create netfns manager")
		os.Exit(1)
	}

	var preferredNetwork *net.IPNet
	if len(preferNetwork) > 0 {
		_, preferredNetwork, err = net.ParseCIDR(preferNetwork)
		if err != nil {
			log.Fatalf("invalid prefer network address: %s - %v", preferNetwork, err)
		}
	}

	// setup net-dpservice client
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	conn, err := grpc.DialContext(ctx, dpserviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		setupLog.Error(err, "unable create dpdk client")
		os.Exit(1)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			setupLog.Error(err, "unable to close dpdk connection")
		}
	}()

	dpdkProtoClient := dpdkproto.NewDPDKonmetalClient(conn)
	dpdkClient := dpdkclient.NewClient(dpdkProtoClient)

	var mbClient dpdkmetalbond.MbInternalAccess
	config := mb.Config{
		KeepaliveInterval: 3,
	}

	mbClient, err = dpdkmetalbond.NewClient(&logger, dpdkClient, dpdkmetalbond.ClientOptions{
		IPv4Only:         true,
		PreferredNetwork: preferredNetwork,
	})
	if err != nil {
		setupLog.Error(err, "unable to initialize metalbond client")
		os.Exit(1)
	}

	mbInstance := mb.NewMetalBond(config, mbClient)
	metalbondClient := metalbond.NewClient(mbInstance)

	for _, metalbondPeer := range metalbondPeers {
		if err := mbInstance.AddPeer(metalbondPeer, ""); err != nil {
			setupLog.Error(err, "failed to add metalbond peer", "MetalbondPeer", metalbondPeer)
			os.Exit(1)
		}
	}

	dpdkUUID, err := dpdkProtoClient.CheckInitialized(context.Background(), &dpdkproto.CheckInitializedRequest{})
	if err != nil {
		_, err = dpdkProtoClient.Initialize(context.Background(), &dpdkproto.InitializeRequest{})
		if err != nil {
			setupLog.Error(err, "dp-service can not be initialized")
			os.Exit(1)
		}

		dpdkUUID, err = dpdkProtoClient.CheckInitialized(context.Background(), &dpdkproto.CheckInitializedRequest{})
		if err != nil {
			setupLog.Error(err, "dp-service down")
			os.Exit(1)
		}
	}

	protoVersion, err := dpdkClient.GetVersion(ctx, &dpdk.Version{
		TypeMeta: dpdk.TypeMeta{Kind: dpdk.VersionKind},
		VersionMeta: dpdk.VersionMeta{
			ClientName:    fmt.Sprintf("metalnet-%s", hostName),
			ClientVersion: buildVersion,
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to get proto version")
	}
	setupLog.Info("protobuf versions",
		"dpserviceProtocol", protoVersion.Spec.ServiceProtocol,
		"dpserviceVersion", protoVersion.Spec.ServiceVersion,
		"metalnetName", protoVersion.ClientName,
		"metalnetProtocol", protoVersion.ClientProtocol,
		"metalnetVersion", protoVersion.ClientVersion)

	if err := metalnetclient.SetupNetworkInterfaceNetworkRefNameFieldIndexer(context.TODO(), mgr.GetFieldIndexer()); err != nil {
		setupLog.Error(err, "unable to set up field indexer", "Field", metalnetclient.NetworkInterfaceNetworkRefNameField)
		os.Exit(1)
	}

	if err := metalnetclient.SetupLoadBalancerNetworkRefNameFieldIndexer(context.TODO(), mgr.GetFieldIndexer()); err != nil {
		setupLog.Error(err, "unable to set up field indexer", "Field", metalnetclient.LoadBalancerNetworkRefNameField)
		os.Exit(1)
	}

	if err = (&controllers.NetworkReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		DPDK:          dpdkClient,
		Metalbond:     metalbondClient,
		MBInternal:    mbClient,
		RouterAddress: netip.MustParseAddr(routerAddress.String()),
		NodeName:      nodeName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Network")
		os.Exit(1)
	}
	if err = (&controllers.NetworkInterfaceReconciler{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor("networkinterface"),
		Scheme:        mgr.GetScheme(),
		DPDK:          dpdkclient.NewClient(dpdkProtoClient),
		Metalbond:     metalbond.NewClient(mbInstance),
		NetFnsManager: netFnsManager,
		SysFS:         sysFS,
		NodeName:      nodeName,
		PublicVNI:     publicVNI,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NetworkInterface")
		os.Exit(1)
	}

	if err = (&controllers.LoadBalancerReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		DPDK:       dpdkclient.NewClient(dpdkProtoClient),
		Metalbond:  metalbond.NewClient(mbInstance),
		NodeName:   nodeName,
		PublicVNI:  publicVNI,
		MBInternal: mbClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LoadBalancer")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	var dpChecker healthz.Checker = func(_ *http.Request) error {
		uuid, err := dpdkProtoClient.CheckInitialized(context.Background(), &dpdkproto.CheckInitializedRequest{})
		if err != nil {
			return fmt.Errorf("dp-service down: %w", err)
		}
		if expectedUUID, actualUUID := dpdkUUID.GetUuid(), uuid.GetUuid(); expectedUUID != actualUUID {
			return fmt.Errorf("dp-service restart detected - %s | %s", expectedUUID, actualUUID)
		}
		return nil
	}
	if err := mgr.AddHealthzCheck("healthz", dpChecker); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
