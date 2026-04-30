/*
Copyright 2026.

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
	"flag"
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	meshv1alpha1 "github.com/iglin/envoy-mesh/control-plane/api/v1alpha1"
	"github.com/iglin/envoy-mesh/control-plane/internal/controller"
	"github.com/iglin/envoy-mesh/control-plane/internal/xds"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(meshv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var probeAddr string
	var xdsAddr string
	var enableLeaderElection bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "Address for the metrics endpoint (0 to disable).")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Address for health probes.")
	flag.StringVar(&xdsAddr, "xds-bind-address", ":18000", "Address for the xDS gRPC server.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election to ensure only one active controller manager.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	xdsSrv := xds.NewServer()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "545d2aa8.mesh.envoy.io",
	})
	if err != nil {
		setupLog.Error(err, "Failed to create manager")
		os.Exit(1)
	}

	if err := (&controller.EnvoyProxyReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		XDSServer: xdsSrv,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to set up EnvoyProxy controller")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	// Start the xDS gRPC server in the background. It shuts down when ctx is cancelled.
	go func() {
		if err := xdsSrv.Start(ctx, xdsAddr); err != nil {
			setupLog.Error(err, "xDS server error")
			os.Exit(1)
		}
	}()

	setupLog.Info("Starting manager", "xdsAddr", xdsAddr)
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}
