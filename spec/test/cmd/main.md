# Test Specification: `main_test.go`

## Source File Under Test

`cmd/main.go`

## Test File

`cmd/main_test.go`

---

## `init`

### Happy Path — init

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestInit_RegistersCoreTypes` | `unit` | Scheme contains core Kubernetes types after init. | Import the `cmd` package (triggers `init()`) | Package-level `scheme` variable | `scheme.AllKnownTypes()` includes core types (e.g., `corev1.Pod`, `corev1.Service`) |
| `TestInit_RegistersDevPodTypes` | `unit` | Scheme contains DevPod CRD types after init. | Import the `cmd` package (triggers `init()`) | Package-level `scheme` variable | `scheme.AllKnownTypes()` includes `devpodv1.DevPod` and `devpodv1.DevPodList` |

---

## `disableHTTP2`

### Happy Path — disableHTTP2

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestDisableHTTP2_SetsNextProtos` | `unit` | Restricts TLS NextProtos to HTTP/1.1 only. | | `&tls.Config{}` | `cfg.NextProtos` equals `[]string{"http/1.1"}` |
| `TestDisableHTTP2_OverwritesExistingNextProtos` | `unit` | Overwrites any pre-existing NextProtos with HTTP/1.1 only. | | `&tls.Config{NextProtos: []string{"h2", "http/1.1"}}` | `cfg.NextProtos` equals `[]string{"http/1.1"}` |

---

## `main` (Bootstrap Sequence)

### Happy Path — TLS Configuration

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestMain_HTTP2DisabledByDefault` | `unit` | TLS options include disableHTTP2 when enable-http2 flag is false. | Extract bootstrap into a testable `run()` function or equivalent seam; stub `ctrl.NewManager` to capture the `metricsserver.Options` passed; stub `ctrl.GetConfigOrDie` to return a fake REST config; set `--enable-http2=false` (default) | Flag set with defaults | The TLS options slice passed to `metricsserver.Options` contains the `disableHTTP2` modifier; applying the modifier to a `*tls.Config` results in `NextProtos == []string{"http/1.1"}` |
| `TestMain_HTTP2EnabledExplicitly` | `unit` | TLS options do NOT include disableHTTP2 when flag is true. | Same seam as above; set `--enable-http2=true` | `--enable-http2=true` | The TLS options slice passed to `metricsserver.Options` is empty (no disableHTTP2 modifier appended) |

### Happy Path — Metrics Configuration

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestMain_MetricsSecureEnabled` | `unit` | Assigns authn/authz FilterProvider when metrics-secure is true. | Testable bootstrap seam; stub manager creation to capture options; set `--metrics-secure=true` (default) | Flag set with defaults | `metricsserver.Options.FilterProvider` is `filters.WithAuthenticationAndAuthorization` |
| `TestMain_MetricsSecureDisabled` | `unit` | Does not assign FilterProvider when metrics-secure is false. | Testable bootstrap seam; stub manager creation; set `--metrics-secure=false` | `--metrics-secure=false` | `metricsserver.Options.FilterProvider` is nil |
| `TestMain_MetricsCertPathConfigured` | `unit` | Configures cert directory, cert name, and key name when cert-path is set. | Testable bootstrap seam; stub manager creation; set `--metrics-cert-path=/certs`, `--metrics-cert-name=server.crt`, `--metrics-cert-key=server.key` | `--metrics-cert-path=/certs`, `--metrics-cert-name=server.crt`, `--metrics-cert-key=server.key` | `metricsserver.Options.CertDir == "/certs"`, `CertName == "server.crt"`, `KeyName == "server.key"` |
| `TestMain_MetricsCertPathEmpty` | `unit` | Does not configure cert directory when cert-path is empty. | Testable bootstrap seam; stub manager creation; set `--metrics-cert-path=""` (default) | Flag set with defaults | `metricsserver.Options.CertDir` is empty; `CertName` and `KeyName` are not explicitly set |
| `TestMain_MetricsBindAddress` | `unit` | Passes metrics bind address flag value to metrics server options. | Testable bootstrap seam; stub manager creation; set `--metrics-bind-address=:8443` | `--metrics-bind-address=:8443` | `metricsserver.Options.BindAddress == ":8443"` |

### Happy Path — Manager Options

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestMain_LeaderElectionID` | `unit` | Manager is created with hardcoded leader election ID. | Testable bootstrap seam; stub `ctrl.NewManager` to capture options; provide fake REST config | Flag set with defaults | Manager options contain `LeaderElectionID == "cec0a61b.devpod.com"` |
| `TestMain_LeaderElectionEnabled` | `unit` | Manager is created with leader election enabled when flag is true. | Testable bootstrap seam; set `--leader-elect=true` | `--leader-elect=true` | Manager options contain `LeaderElection == true` |
| `TestMain_LeaderElectionDisabledByDefault` | `unit` | Manager is created with leader election disabled by default. | Testable bootstrap seam; use default flags | Flag set with defaults | Manager options contain `LeaderElection == false` |
| `TestMain_HealthProbeBindAddress` | `unit` | Manager is created with the health probe bind address from the flag. | Testable bootstrap seam; set `--health-probe-bind-address=:9090` | `--health-probe-bind-address=:9090` | Manager options contain `HealthProbeBindAddress == ":9090"` |
| `TestMain_UsesPackageScheme` | `unit` | Manager is created with the package-level scheme. | Testable bootstrap seam; stub `ctrl.NewManager` to capture options | Flag set with defaults | Manager options contain `Scheme` equal to the package-level `scheme` variable |

### Mock / Dependency Interaction

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestMain_ConstructsReconcilerWithManagerClient` | `unit` | DevPodReconciler is constructed with the manager's client. | Testable bootstrap seam; stub `ctrl.NewManager` to return a fake manager with a known client; stub `SetupWithManager` to capture the reconciler | Flag set with defaults | `DevPodReconciler.Client` is the value returned by `mgr.GetClient()` |
| `TestMain_ConstructsReconcilerWithManagerScheme` | `unit` | DevPodReconciler is constructed with the manager's scheme. | Testable bootstrap seam; stub `ctrl.NewManager` to return a fake manager with a known scheme; stub `SetupWithManager` to capture the reconciler | Flag set with defaults | `DevPodReconciler.Scheme` is the value returned by `mgr.GetScheme()` |
| `TestMain_CallsSetupWithManager` | `unit` | Calls SetupWithManager on the constructed DevPodReconciler. | Testable bootstrap seam; mock `DevPodReconciler.SetupWithManager` to record the call | Flag set with defaults | `SetupWithManager` is called exactly once with the created manager |
| `TestMain_RegistersHealthzProbe` | `unit` | Registers healthz probe with name "healthz" and healthz.Ping checker. | Testable bootstrap seam; mock `mgr.AddHealthzCheck` to record the call | Flag set with defaults | `mgr.AddHealthzCheck` called with name `"healthz"` and checker `healthz.Ping` |
| `TestMain_RegistersReadyzProbe` | `unit` | Registers readyz probe with name "readyz" and healthz.Ping checker. | Testable bootstrap seam; mock `mgr.AddReadyzCheck` to record the call | Flag set with defaults | `mgr.AddReadyzCheck` called with name `"readyz"` and checker `healthz.Ping` |
| `TestMain_StartsManagerWithSignalContext` | `unit` | Calls mgr.Start with the signal handler context. | Testable bootstrap seam; mock `mgr.Start` to record the call; stub `ctrl.SetupSignalHandler` to return a known context | Flag set with defaults | `mgr.Start` is called exactly once with the context from `ctrl.SetupSignalHandler()` |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestMain_ManagerCreationFailure_Exits` | `unit` | Exits with code 1 when manager creation fails. | Testable bootstrap seam; stub `ctrl.NewManager` to return an error; replace `os.Exit` with a capturing hook (e.g., `var osExit = os.Exit` overridden in test) | Flag set with defaults; `ctrl.NewManager` returns `(nil, errors.New("port conflict"))` | Captured exit code is `1` |
| `TestMain_ControllerSetupFailure_Exits` | `unit` | Exits with code 1 when SetupWithManager fails. | Testable bootstrap seam; stub `ctrl.NewManager` to succeed; stub `SetupWithManager` to return an error; replace `os.Exit` with a capturing hook | Flag set with defaults; `SetupWithManager` returns `errors.New("duplicate controller")` | Captured exit code is `1` |
| `TestMain_HealthzRegistrationFailure_Exits` | `unit` | Exits with code 1 when healthz probe registration fails. | Testable bootstrap seam; mock `mgr.AddHealthzCheck` to return an error; replace `os.Exit` with a capturing hook | Flag set with defaults; `mgr.AddHealthzCheck` returns error | Captured exit code is `1` |
| `TestMain_ReadyzRegistrationFailure_Exits` | `unit` | Exits with code 1 when readyz probe registration fails. | Testable bootstrap seam; mock `mgr.AddReadyzCheck` to return an error (after healthz succeeds); replace `os.Exit` with a capturing hook | Flag set with defaults; `mgr.AddReadyzCheck` returns error | Captured exit code is `1` |
| `TestMain_ManagerStartFailure_Exits` | `unit` | Exits with code 1 when mgr.Start returns an error. | Testable bootstrap seam; stub all setup steps to succeed; mock `mgr.Start` to return an error; replace `os.Exit` with a capturing hook | Flag set with defaults; `mgr.Start` returns `errors.New("leader election lost")` | Captured exit code is `1` |
| `TestMain_ManagerStartSuccess_NoExitCall` | `unit` | Does not call os.Exit when mgr.Start returns nil. | Testable bootstrap seam; stub all setup steps to succeed; mock `mgr.Start` to return nil; replace `os.Exit` with a capturing hook | Flag set with defaults; `mgr.Start` returns `nil` | `os.Exit` is never called; function returns normally |

---

## Flag Defaults

### Happy Path — Flag Defaults

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestFlagDefaults_MetricsBindAddress` | `unit` | Default metrics bind address is "0" (disabled). | Parse flags with no arguments using a dedicated `flag.FlagSet` (to avoid global state pollution) or inspect the registered flag default | No CLI arguments | `--metrics-bind-address` default value is `"0"` |
| `TestFlagDefaults_HealthProbeBindAddress` | `unit` | Default health probe bind address is ":8081". | Parse flags with no arguments | No CLI arguments | `--health-probe-bind-address` default value is `":8081"` |
| `TestFlagDefaults_LeaderElect` | `unit` | Default leader election is disabled. | Parse flags with no arguments | No CLI arguments | `--leader-elect` default value is `false` |
| `TestFlagDefaults_MetricsSecure` | `unit` | Default metrics-secure is enabled. | Parse flags with no arguments | No CLI arguments | `--metrics-secure` default value is `true` |
| `TestFlagDefaults_MetricsCertPath` | `unit` | Default metrics cert path is empty. | Parse flags with no arguments | No CLI arguments | `--metrics-cert-path` default value is `""` |
| `TestFlagDefaults_MetricsCertName` | `unit` | Default metrics cert name is "tls.crt". | Parse flags with no arguments | No CLI arguments | `--metrics-cert-name` default value is `"tls.crt"` |
| `TestFlagDefaults_MetricsCertKey` | `unit` | Default metrics cert key is "tls.key". | Parse flags with no arguments | No CLI arguments | `--metrics-cert-key` default value is `"tls.key"` |
| `TestFlagDefaults_EnableHTTP2` | `unit` | Default enable-http2 is disabled. | Parse flags with no arguments | No CLI arguments | `--enable-http2` default value is `false` |
