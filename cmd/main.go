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
	"context"
	"crypto/tls"
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
	"github.com/tcfwbper/devpod-operator/internal/controller"
	// +kubebuilder:scaffold:imports
)

var (
	version  = "dev"
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(appsv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// flagConfig holds parsed CLI flag values.
type flagConfig struct {
	metricsAddr          string
	probeAddr            string
	enableLeaderElection bool
	secureMetrics        bool
	enableHTTP2          bool
	metricsCertPath      string
	metricsCertName      string
	metricsCertKey       string
	webhookCertPath      string
	webhookCertName      string
	webhookCertKey       string
}

// registerFlags registers CLI flags on the given FlagSet and returns a flagConfig
// holding the bound variables.
func registerFlags(fs *flag.FlagSet) *flagConfig {
	cfg := &flagConfig{}
	fs.StringVar(&cfg.metricsAddr, "metrics-bind-address", "0",
		"The address the metrics endpoint binds to. "+
			"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	fs.StringVar(&cfg.probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	fs.BoolVar(&cfg.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	fs.BoolVar(&cfg.secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	fs.StringVar(&cfg.webhookCertPath, "webhook-cert-path", "",
		"The directory that contains the webhook certificate.")
	fs.StringVar(&cfg.webhookCertName, "webhook-cert-name", "tls.crt",
		"The name of the webhook certificate file.")
	fs.StringVar(&cfg.webhookCertKey, "webhook-cert-key", "tls.key",
		"The name of the webhook key file.")
	fs.StringVar(&cfg.metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	fs.StringVar(&cfg.metricsCertName, "metrics-cert-name", "tls.crt",
		"The name of the metrics server certificate file.")
	fs.StringVar(&cfg.metricsCertKey, "metrics-cert-key", "tls.key",
		"The name of the metrics server key file.")
	fs.BoolVar(&cfg.enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	return cfg
}

// disableHTTP2 restricts the given TLS config to HTTP/1.1 only.
func disableHTTP2(c *tls.Config) {
	setupLog.Info("Disabling HTTP/2")
	c.NextProtos = []string{"http/1.1"}
}

// runDeps holds injectable dependencies for the bootstrap sequence.
type runDeps struct {
	newManager       func(config *rest.Config, opts ctrl.Options) (ctrl.Manager, error)
	getConfig        func() *rest.Config
	signalHandler    func() context.Context
	exit             func(code int)
	setupWithManager func(r *controller.DevPodReconciler, mgr ctrl.Manager) error
}

// run executes the bootstrap sequence: TLS config, manager creation,
// controller wiring, probe registration, and manager start.
//
//nolint:gocyclo
func run(cfg *flagConfig, deps runDeps) {
	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	var tlsOpts []func(*tls.Config)
	if !cfg.enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(cfg.webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", cfg.webhookCertPath, "webhook-cert-name", cfg.webhookCertName, "webhook-cert-key", cfg.webhookCertKey)

		webhookServerOptions.CertDir = cfg.webhookCertPath
		webhookServerOptions.CertName = cfg.webhookCertName
		webhookServerOptions.KeyName = cfg.webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   cfg.metricsAddr,
		SecureServing: cfg.secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if cfg.secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(cfg.metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", cfg.metricsCertPath, "metrics-cert-name", cfg.metricsCertName, "metrics-cert-key", cfg.metricsCertKey)

		metricsServerOptions.CertDir = cfg.metricsCertPath
		metricsServerOptions.CertName = cfg.metricsCertName
		metricsServerOptions.KeyName = cfg.metricsCertKey
	}

	mgr, err := deps.newManager(deps.getConfig(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: cfg.probeAddr,
		LeaderElection:         cfg.enableLeaderElection,
		LeaderElectionID:       "cec0a61b.devpod.com",
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
		setupLog.Error(err, "Failed to start manager")
		deps.exit(1)
		return
	}

	if err := deps.setupWithManager(&controller.DevPodReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}, mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "devpod")
		deps.exit(1)
		return
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		deps.exit(1)
		return
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		deps.exit(1)
		return
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(deps.signalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		deps.exit(1)
		return
	}
}

func main() {
	cfg := registerFlags(flag.CommandLine)

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("DevPod operator", "version", version)

	deps := runDeps{
		newManager: ctrl.NewManager,
		getConfig:  func() *rest.Config { return ctrl.GetConfigOrDie() },
		signalHandler: func() context.Context {
			return ctrl.SetupSignalHandler()
		},
		exit: os.Exit,
		setupWithManager: func(r *controller.DevPodReconciler, mgr ctrl.Manager) error {
			return r.SetupWithManager(mgr)
		},
	}

	run(cfg, deps)
}
