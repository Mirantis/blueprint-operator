package main

import (
	"flag"
	"os"

	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	apiextenv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	// BlueprintInfo is a gauge metric to observe the Blueprint Operator version.
	BlueprintInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "blueprint_info",
		Help: "Blueprint Operator Information",
	}, []string{"version"})
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextenv1.AddToScheme(scheme))

	utilruntime.Must(boundlessv1alpha1.AddToScheme(scheme))
	utilruntime.Must(certmanager.AddToScheme(scheme))

	utilruntime.Must(helmv2.AddToScheme(scheme))
	utilruntime.Must(sourcev1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
	// Register custom metrics
	metrics.Registry.MustRegister(BlueprintInfo, controllers.AddOnHistVec, controllers.InstallationHistVec, controllers.ManifestHistVec)

}

func main() {
	var webhook bool
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&webhook, "webhook", false, "Run as webhook controller")
	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if webhook {
		enableLeaderElection = false
	}

	// TODO: Set to correct version
	BlueprintInfo.WithLabelValues("1.0.0-rc1").Set(1)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "a3fc41e4.mirantis.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if webhook {
		// @todo: Create a separate implementation for webhook server
		//   JIRA: https://mirantis.jira.com/browse/BOP-534
		//   This is a temporary solution to run the operator as a webhook server
		//   to save time on the initial implementation as we currently don't have CI for creating/managing other images.
		//   The final implementation should be a separate image for the webhook server.
		setupLog.Info("Running as webhook controller")
		if err = (&boundlessv1alpha1.Blueprint{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Blueprint")
			os.Exit(1)
		}
	} else {
		setupLog.Info("Running as operator controller")
		if err = (&controllers.AddonReconciler{
			Client:      mgr.GetClient(),
			Scheme:      mgr.GetScheme(),
			Recorder:    mgr.GetEventRecorderFor("addon controller"),
			SetupLogger: setupLog,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Addon")
			os.Exit(1)
		}
		if err = (&controllers.BlueprintReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Blueprint")
			os.Exit(1)
		}
		if err = (&controllers.ManifestReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("manifest controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Manifest")
			os.Exit(1)
		}
		if err = (&controllers.InstallationReconciler{
			Client:      mgr.GetClient(),
			Scheme:      mgr.GetScheme(),
			SetupLogger: setupLog,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Installation")
			os.Exit(1)
		}
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
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
