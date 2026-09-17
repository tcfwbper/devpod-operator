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
	"errors"
	"flag"
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"

	"github.com/tcfwbper/devpod-operator/internal/controller"
)

// =============================================================================
// Mock Manager
// =============================================================================

// mockManager implements ctrl.Manager for unit testing.
type mockManager struct {
	client       client.Client
	scheme       *runtime.Scheme
	healthChecks map[string]healthz.Checker
	readyChecks  map[string]healthz.Checker
	startCtx     context.Context
	startErr     error
	addHealthErr error
	addReadyErr  error
}

var _ ctrl.Manager = (*mockManager)(nil)

func newMockManager() *mockManager {
	return &mockManager{
		client:       fake.NewClientBuilder().Build(),
		scheme:       runtime.NewScheme(),
		healthChecks: make(map[string]healthz.Checker),
		readyChecks:  make(map[string]healthz.Checker),
	}
}

func (m *mockManager) GetClient() client.Client                                    { return m.client }
func (m *mockManager) GetScheme() *runtime.Scheme                                  { return m.scheme }
func (m *mockManager) GetConfig() *rest.Config                                     { return &rest.Config{} }
func (m *mockManager) GetHTTPClient() *http.Client                                 { return &http.Client{} }
func (m *mockManager) GetFieldIndexer() client.FieldIndexer                        { return nil }
func (m *mockManager) GetCache() cache.Cache                                       { return nil }
func (m *mockManager) GetEventRecorderFor(_ string) record.EventRecorder           { return nil }
func (m *mockManager) GetEventRecorder(_ string) events.EventRecorder              { return nil }
func (m *mockManager) GetRESTMapper() meta.RESTMapper                              { return nil }
func (m *mockManager) GetAPIReader() client.Reader                                 { return nil }
func (m *mockManager) Add(_ manager.Runnable) error                                { return nil }
func (m *mockManager) Elected() <-chan struct{}                                    { return make(chan struct{}) }
func (m *mockManager) AddMetricsServerExtraHandler(_ string, _ http.Handler) error { return nil }
func (m *mockManager) GetWebhookServer() webhook.Server                            { return nil }
func (m *mockManager) GetLogger() logr.Logger                                      { return logr.Discard() }
func (m *mockManager) GetControllerOptions() config.Controller                     { return config.Controller{} }
func (m *mockManager) GetConverterRegistry() conversion.Registry                   { return conversion.NewRegistry() }

func (m *mockManager) AddHealthzCheck(name string, check healthz.Checker) error {
	if m.addHealthErr != nil {
		return m.addHealthErr
	}
	m.healthChecks[name] = check
	return nil
}

func (m *mockManager) AddReadyzCheck(name string, check healthz.Checker) error {
	if m.addReadyErr != nil {
		return m.addReadyErr
	}
	m.readyChecks[name] = check
	return nil
}

func (m *mockManager) Start(ctx context.Context) error {
	m.startCtx = ctx
	return m.startErr
}

// =============================================================================
// Test Helpers
// =============================================================================

// defaultFlagConfig returns a flagConfig with production default values.
func defaultFlagConfig() *flagConfig {
	return &flagConfig{
		metricsAddr:          "0",
		probeAddr:            ":8081",
		enableLeaderElection: false,
		secureMetrics:        true,
		enableHTTP2:          false,
		metricsCertPath:      "",
		metricsCertName:      "tls.crt",
		metricsCertKey:       "tls.key",
		webhookCertPath:      "",
		webhookCertName:      "tls.crt",
		webhookCertKey:       "tls.key",
	}
}

// capturedOptions records the ctrl.Options passed to newManager.
type capturedOptions struct {
	opts ctrl.Options
}

// testRunDeps returns a runDeps configured for testing with a mock manager.
// The returned capturedOptions pointer captures the ctrl.Options passed to newManager.
func testRunDeps(mockMgr *mockManager) (runDeps, *capturedOptions) {
	captured := &capturedOptions{}
	return runDeps{
		newManager: func(_ *rest.Config, opts ctrl.Options) (ctrl.Manager, error) {
			captured.opts = opts
			return mockMgr, nil
		},
		getConfig:     func() *rest.Config { return &rest.Config{} },
		signalHandler: func() context.Context { return context.Background() },
		exit:          func(_ int) {},
		setupWithManager: func(_ *controller.DevPodReconciler, _ ctrl.Manager) error {
			return nil
		},
	}, captured
}

// =============================================================================
// init — Scheme Registration
// =============================================================================

func TestInit_RegistersCoreTypes(t *testing.T) {
	// The package-level scheme variable is populated by init() which runs
	// when this test package is loaded.
	knownTypes := scheme.AllKnownTypes()

	// Core Kubernetes types should be present via clientgoscheme.AddToScheme.
	podGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	serviceGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}

	_, hasPod := knownTypes[podGVK]
	_, hasService := knownTypes[serviceGVK]

	assert.True(t, hasPod, "scheme should contain corev1.Pod")
	assert.True(t, hasService, "scheme should contain corev1.Service")
}

func TestInit_RegistersDevPodTypes(t *testing.T) {
	knownTypes := scheme.AllKnownTypes()

	// DevPod CRD types should be present via appsv1.AddToScheme.
	devPodGVK := schema.GroupVersionKind{Group: "apps.devpod.com", Version: "v1", Kind: "DevPod"}
	devPodListGVK := schema.GroupVersionKind{Group: "apps.devpod.com", Version: "v1", Kind: "DevPodList"}

	_, hasDevPod := knownTypes[devPodGVK]
	_, hasDevPodList := knownTypes[devPodListGVK]

	assert.True(t, hasDevPod, "scheme should contain devpodv1.DevPod")
	assert.True(t, hasDevPodList, "scheme should contain devpodv1.DevPodList")
}

// =============================================================================
// disableHTTP2
// =============================================================================

func TestDisableHTTP2_SetsNextProtos(t *testing.T) {
	cfg := &tls.Config{}
	disableHTTP2(cfg)
	assert.Equal(t, []string{protoHTTP11}, cfg.NextProtos)
}

func TestDisableHTTP2_OverwritesExistingNextProtos(t *testing.T) {
	cfg := &tls.Config{NextProtos: []string{"h2", protoHTTP11}}
	disableHTTP2(cfg)
	assert.Equal(t, []string{protoHTTP11}, cfg.NextProtos)
}

// =============================================================================
// main (Bootstrap Sequence) — TLS Configuration
// =============================================================================

func TestMain_HTTP2DisabledByDefault(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	cfg := defaultFlagConfig()
	// enableHTTP2 defaults to false
	run(cfg, deps)

	// TLSOpts should contain disableHTTP2 function
	assert.NotEmpty(t, captured.opts.Metrics.TLSOpts, "TLSOpts should not be empty when HTTP/2 is disabled")

	// Verify the effect: applying TLSOpts restricts to http/1.1
	tlsCfg := &tls.Config{}
	for _, fn := range captured.opts.Metrics.TLSOpts {
		fn(tlsCfg)
	}
	assert.Equal(t, []string{protoHTTP11}, tlsCfg.NextProtos)
}

func TestMain_HTTP2EnabledExplicitly(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	cfg := defaultFlagConfig()
	cfg.enableHTTP2 = true

	run(cfg, deps)

	assert.Empty(t, captured.opts.Metrics.TLSOpts, "TLSOpts should be empty when HTTP/2 is enabled")
}

// =============================================================================
// main (Bootstrap Sequence) — Metrics Configuration
// =============================================================================

func TestMain_MetricsSecureEnabled(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	cfg := defaultFlagConfig()
	cfg.secureMetrics = true

	run(cfg, deps)

	assert.True(t, captured.opts.Metrics.SecureServing)
	assert.NotNil(t, captured.opts.Metrics.FilterProvider, "FilterProvider should be set when metrics are secure")
}

func TestMain_MetricsSecureDisabled(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	cfg := defaultFlagConfig()
	cfg.secureMetrics = false

	run(cfg, deps)

	assert.False(t, captured.opts.Metrics.SecureServing)
	assert.Nil(t, captured.opts.Metrics.FilterProvider, "FilterProvider should not be set when metrics are insecure")
}

func TestMain_MetricsCertPathConfigured(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	cfg := defaultFlagConfig()
	cfg.metricsCertPath = "/etc/certs"
	cfg.metricsCertName = "server.crt"
	cfg.metricsCertKey = "server.key"

	run(cfg, deps)

	assert.Equal(t, "/etc/certs", captured.opts.Metrics.CertDir)
	assert.Equal(t, "server.crt", captured.opts.Metrics.CertName)
	assert.Equal(t, "server.key", captured.opts.Metrics.KeyName)
}

func TestMain_MetricsCertPathEmpty(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	cfg := defaultFlagConfig()
	cfg.metricsCertPath = ""

	run(cfg, deps)

	assert.Empty(t, captured.opts.Metrics.CertDir, "CertDir should be empty when cert path is not provided")
}

func TestMain_MetricsBindAddress(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	cfg := defaultFlagConfig()
	cfg.metricsAddr = ":8443"

	run(cfg, deps)

	assert.Equal(t, ":8443", captured.opts.Metrics.BindAddress)
}

// =============================================================================
// main (Bootstrap Sequence) — Manager Options
// =============================================================================

func TestMain_LeaderElectionID(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	run(defaultFlagConfig(), deps)

	assert.Equal(t, "cec0a61b.devpod.com", captured.opts.LeaderElectionID)
}

func TestMain_LeaderElectionEnabled(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	cfg := defaultFlagConfig()
	cfg.enableLeaderElection = true

	run(cfg, deps)

	assert.True(t, captured.opts.LeaderElection)
}

func TestMain_LeaderElectionDisabledByDefault(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	cfg := defaultFlagConfig()
	// enableLeaderElection defaults to false

	run(cfg, deps)

	assert.False(t, captured.opts.LeaderElection)
}

func TestMain_HealthProbeBindAddress(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	cfg := defaultFlagConfig()
	cfg.probeAddr = ":9090"

	run(cfg, deps)

	assert.Equal(t, ":9090", captured.opts.HealthProbeBindAddress)
}

func TestMain_UsesPackageScheme(t *testing.T) {
	mockMgr := newMockManager()
	deps, captured := testRunDeps(mockMgr)

	run(defaultFlagConfig(), deps)

	assert.Same(t, scheme, captured.opts.Scheme)
}

// =============================================================================
// main (Bootstrap Sequence) — Mock / Dependency Interaction
// =============================================================================

func TestMain_ConstructsReconcilerWithManagerClient(t *testing.T) {
	mockMgr := newMockManager()
	expectedClient := mockMgr.client

	deps, _ := testRunDeps(mockMgr)
	var receivedReconciler *controller.DevPodReconciler
	deps.setupWithManager = func(r *controller.DevPodReconciler, _ ctrl.Manager) error {
		receivedReconciler = r
		return nil
	}

	run(defaultFlagConfig(), deps)

	assert.NotNil(t, receivedReconciler)
	assert.Same(t, expectedClient, receivedReconciler.Client)
}

func TestMain_ConstructsReconcilerWithManagerScheme(t *testing.T) {
	mockMgr := newMockManager()
	expectedScheme := mockMgr.scheme

	deps, _ := testRunDeps(mockMgr)
	var receivedReconciler *controller.DevPodReconciler
	deps.setupWithManager = func(r *controller.DevPodReconciler, _ ctrl.Manager) error {
		receivedReconciler = r
		return nil
	}

	run(defaultFlagConfig(), deps)

	assert.NotNil(t, receivedReconciler)
	assert.Same(t, expectedScheme, receivedReconciler.Scheme)
}

func TestMain_CallsSetupWithManager(t *testing.T) {
	mockMgr := newMockManager()
	deps, _ := testRunDeps(mockMgr)

	setupCalled := false
	deps.setupWithManager = func(_ *controller.DevPodReconciler, _ ctrl.Manager) error {
		setupCalled = true
		return nil
	}

	run(defaultFlagConfig(), deps)

	assert.True(t, setupCalled, "setupWithManager should be called")
}

func TestMain_RegistersHealthzProbe(t *testing.T) {
	mockMgr := newMockManager()
	deps, _ := testRunDeps(mockMgr)

	run(defaultFlagConfig(), deps)

	_, hasHealthz := mockMgr.healthChecks["healthz"]
	assert.True(t, hasHealthz, "healthz check should be registered")
}

func TestMain_RegistersReadyzProbe(t *testing.T) {
	mockMgr := newMockManager()
	deps, _ := testRunDeps(mockMgr)

	run(defaultFlagConfig(), deps)

	_, hasReadyz := mockMgr.readyChecks["readyz"]
	assert.True(t, hasReadyz, "readyz check should be registered")
}

func TestMain_StartsManagerWithSignalContext(t *testing.T) {
	mockMgr := newMockManager()
	deps, _ := testRunDeps(mockMgr)

	expectedCtx := context.WithValue(context.Background(), struct{ key string }{"test"}, "signal")
	deps.signalHandler = func() context.Context { return expectedCtx }

	run(defaultFlagConfig(), deps)

	assert.Equal(t, expectedCtx, mockMgr.startCtx, "manager should be started with signal context")
}

// =============================================================================
// main (Bootstrap Sequence) — Error Propagation
// =============================================================================

func TestMain_ManagerCreationFailure_Exits(t *testing.T) {
	exitCalled := false
	exitCode := 0

	deps := runDeps{
		newManager: func(_ *rest.Config, _ ctrl.Options) (ctrl.Manager, error) {
			return nil, errors.New("manager creation failed")
		},
		getConfig:     func() *rest.Config { return &rest.Config{} },
		signalHandler: func() context.Context { return context.Background() },
		exit: func(code int) {
			exitCalled = true
			exitCode = code
		},
		setupWithManager: func(_ *controller.DevPodReconciler, _ ctrl.Manager) error {
			return nil
		},
	}

	run(defaultFlagConfig(), deps)

	assert.True(t, exitCalled, "exit should be called on manager creation failure")
	assert.Equal(t, 1, exitCode)
}

func TestMain_ControllerSetupFailure_Exits(t *testing.T) {
	mockMgr := newMockManager()
	exitCalled := false
	exitCode := 0

	deps, _ := testRunDeps(mockMgr)
	deps.exit = func(code int) {
		exitCalled = true
		exitCode = code
	}
	deps.setupWithManager = func(_ *controller.DevPodReconciler, _ ctrl.Manager) error {
		return errors.New("controller setup failed")
	}

	run(defaultFlagConfig(), deps)

	assert.True(t, exitCalled, "exit should be called on controller setup failure")
	assert.Equal(t, 1, exitCode)
}

func TestMain_HealthzRegistrationFailure_Exits(t *testing.T) {
	mockMgr := newMockManager()
	mockMgr.addHealthErr = errors.New("healthz registration failed")

	exitCalled := false
	exitCode := 0

	deps, _ := testRunDeps(mockMgr)
	deps.exit = func(code int) {
		exitCalled = true
		exitCode = code
	}

	run(defaultFlagConfig(), deps)

	assert.True(t, exitCalled, "exit should be called on healthz registration failure")
	assert.Equal(t, 1, exitCode)
}

func TestMain_ReadyzRegistrationFailure_Exits(t *testing.T) {
	mockMgr := newMockManager()
	mockMgr.addReadyErr = errors.New("readyz registration failed")

	exitCalled := false
	exitCode := 0

	deps, _ := testRunDeps(mockMgr)
	deps.exit = func(code int) {
		exitCalled = true
		exitCode = code
	}

	run(defaultFlagConfig(), deps)

	assert.True(t, exitCalled, "exit should be called on readyz registration failure")
	assert.Equal(t, 1, exitCode)
}

func TestMain_ManagerStartFailure_Exits(t *testing.T) {
	mockMgr := newMockManager()
	mockMgr.startErr = errors.New("manager start failed")

	exitCalled := false
	exitCode := 0

	deps, _ := testRunDeps(mockMgr)
	deps.exit = func(code int) {
		exitCalled = true
		exitCode = code
	}

	run(defaultFlagConfig(), deps)

	assert.True(t, exitCalled, "exit should be called on manager start failure")
	assert.Equal(t, 1, exitCode)
}

func TestMain_ManagerStartSuccess_NoExitCall(t *testing.T) {
	mockMgr := newMockManager()
	// startErr is nil by default — success

	exitCalled := false

	deps, _ := testRunDeps(mockMgr)
	deps.exit = func(_ int) {
		exitCalled = true
	}

	run(defaultFlagConfig(), deps)

	assert.False(t, exitCalled, "exit should not be called on successful start")
}

// =============================================================================
// Flag Defaults
// =============================================================================

func TestFlagDefaults_MetricsBindAddress(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg := registerFlags(fs)

	assert.Equal(t, "0", cfg.metricsAddr)
}

func TestFlagDefaults_HealthProbeBindAddress(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg := registerFlags(fs)

	assert.Equal(t, ":8081", cfg.probeAddr)
}

func TestFlagDefaults_LeaderElect(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg := registerFlags(fs)

	assert.False(t, cfg.enableLeaderElection)
}

func TestFlagDefaults_MetricsSecure(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg := registerFlags(fs)

	assert.True(t, cfg.secureMetrics)
}

func TestFlagDefaults_MetricsCertPath(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg := registerFlags(fs)

	assert.Empty(t, cfg.metricsCertPath)
}

func TestFlagDefaults_MetricsCertName(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg := registerFlags(fs)

	assert.Equal(t, "tls.crt", cfg.metricsCertName)
}

func TestFlagDefaults_MetricsCertKey(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg := registerFlags(fs)

	assert.Equal(t, "tls.key", cfg.metricsCertKey)
}

func TestFlagDefaults_EnableHTTP2(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg := registerFlags(fs)

	assert.False(t, cfg.enableHTTP2)
}
