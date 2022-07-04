/*
Copyright 2021.

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
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	cpev1 "github.com/IBM/cpe-operator/api/v1"
	"github.com/IBM/cpe-operator/controllers"

	helmclient "github.com/mittwald/go-helm-client"
	"k8s.io/client-go/kubernetes"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const BUILD_MAX_QSIZE = 300

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(cpev1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func newDiscoveryAndDynamicClient(config *rest.Config) (*discovery.DiscoveryClient, dynamic.Interface) {
	dc, _ := discovery.NewDiscoveryClientForConfig(config)
	dyn, _ := dynamic.NewForConfig(config)
	return dc, dyn
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	config := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "6fcf615e.cogadvisor.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	dc, dyn := newDiscoveryAndDynamicClient(config)
	clientset, err := kubernetes.NewForConfig(config)

	err = controllers.InitSearchSpace()
	if err != nil {
		setupLog.Info(fmt.Sprintf("Search Space Init Error: %v", err))
	}
	setupLog.Info(fmt.Sprintf("Search Space: %v", controllers.SearchSpace))

	tunedHandler := &controllers.TunedHandler{
		Clientset: clientset,
		Log:       ctrl.Log.WithName("controllers").WithName("TunedHandler"),
		DYN:       dyn,
	}

	jobTrackers := make(map[string]*controllers.JobTracker)
	cos := controllers.COSObject{}
	cos.InitValue()

	quit := make(chan struct{})
	defer close(quit)

	jobTrackManager := &controllers.JobTrackManager{
		Client:       mgr.GetClient(),
		Clientset:    clientset,
		JobTrackers:  jobTrackers,
		Cos:          cos,
		GlobalQuit:   quit,
		Log:          ctrl.Log.WithName("controllers").WithName("JobTracker"),
		DC:           dc,
		DYN:          dyn,
		TunedHandler: tunedHandler,
	}

	go jobTrackManager.Run()

	if err = (&controllers.BenchmarkReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("Benchmark"),
		Scheme:       mgr.GetScheme(),
		DC:           dc,
		DYN:          dyn,
		JTM:          jobTrackManager,
		TunedHandler: tunedHandler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Benchmark")
		os.Exit(1)
	}

	helmOpt := &helmclient.RestConfClientOptions{
		Options: &helmclient.Options{
			RepositoryCache:  "/tmp/.helmcache",
			RepositoryConfig: "/tmp/.helmrepo",
			Debug:            true,
			Linting:          true,
		},
		RestConfig: config,
	}

	helmClient, err := helmclient.NewClientFromRestConf(helmOpt)

	if err != nil {
		panic(err)
	}

	_ = helmClient

	if err = (&controllers.BenchmarkOperatorReconciler{
		Clientset:  clientset,
		Client:     mgr.GetClient(),
		Log:        ctrl.Log.WithName("controllers").WithName("BenchmarkOperator"),
		Scheme:     mgr.GetScheme(),
		DC:         dc,
		DYN:        dyn,
		HelmClient: helmClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BenchmarkOperator")
		os.Exit(1)
	}

	buildQueue := make(chan *unstructured.Unstructured, BUILD_MAX_QSIZE)
	defer close(buildQueue)

	buildWatcher := &controllers.BuildWatcher{
		Client:     mgr.GetClient(),
		Log:        ctrl.Log.WithName("trackers").WithName("BuildWatcher"),
		Scheme:     mgr.GetScheme(),
		DC:         dc,
		DYN:        dyn,
		BuildQueue: buildQueue,
		Quit:       quit,
	}

	err = buildWatcher.InitInformer()
	if err == nil {
		go buildWatcher.Run()
	}

	//+kubebuilder:scaffold:builder

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
