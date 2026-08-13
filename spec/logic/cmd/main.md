# Main (Operator Entrypoint)

## Overview

Entry point for the devpod-operator process. Bootstraps the controller-runtime manager by registering API types into the scheme, parsing CLI flags, configuring TLS and metrics serving, wiring the `DevPodReconciler`, registering health probes, and starting the manager with OS signal handling.

Does **not** perform reconciliation, resource creation, or any Kubernetes API operations — all runtime logic is delegated to the controller-runtime manager and the registered controllers.

## Boundaries

- Owns: the full process bootstrap sequence (scheme registration, flag parsing, logger initialization, TLS configuration, metrics server options, manager creation, controller wiring, probe registration, manager start).
- Owns: the package-level `scheme` variable and its population in `init()`.
- Owns: the `setupLog` logger used exclusively during bootstrap.
- Owns: TLS security policy — HTTP/2 is disabled by default to mitigate HTTP/2 Stream Cancellation and Rapid Reset CVEs.
- Owns: metrics server configuration (bind address, secure serving toggle, TLS certs, authn/authz filter).
- Owns: leader election configuration (ID, enable/disable toggle).
- Owns: fatal exit behavior — any setup failure terminates the process.
- Delegates: reconciliation logic to `controller.DevPodReconciler`.
- Delegates: runtime lifecycle (watch setup, cache sync, work queue, goroutine management) to the controller-runtime manager.
- Delegates: health/ready check implementation to `healthz.Ping` (provided by controller-runtime).
- Delegates: signal-based graceful shutdown to `ctrl.SetupSignalHandler()`.
- Must not: perform any reconciliation or resource CRUD.
- Must not: access Kubernetes resources directly (no `client.Get/List/Create/Update/Delete` calls).
- Must not: construct controllers beyond `DevPodReconciler` (additional controllers require explicit spec additions).

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `controller-runtime` (`ctrl`) | Manager framework | `ctrl.NewManager`, `ctrl.SetupSignalHandler`, `ctrl.GetConfigOrDie`, `ctrl.SetLogger`, `ctrl.Log.WithName` | Must not use low-level informer or workqueue APIs |
| `controller.DevPodReconciler` | Reconciliation unit | Construct via struct literal `{Client, Scheme}`, call `SetupWithManager(mgr)` | Must not call `Reconcile` directly |
| `devpodv1.AddToScheme` | API type registration | Call once in `init()` to register DevPod types | Must not call at runtime |
| `clientgoscheme.AddToScheme` | Core type registration | Call once in `init()` to register core Kubernetes types | Must not call at runtime |
| `zap` (controller-runtime log) | Logger factory | `zap.New(zap.UseFlagOptions(&opts))`, `opts.BindFlags(flag.CommandLine)` | Must not construct loggers outside bootstrap |
| `healthz.Ping` | Probe implementation | Pass as checker to `mgr.AddHealthzCheck` / `mgr.AddReadyzCheck` | — |
| `metricsserver.Options` | Metrics config struct | Configure and pass to manager options | Must not start the metrics server independently |
| `filters.WithAuthenticationAndAuthorization` | Metrics authn/authz filter | Assign to `FilterProvider` when secure metrics enabled | — |
| `flag` (stdlib) | CLI flag parsing | Define flags, call `flag.Parse()` | Must not parse flags more than once |
| `os` (stdlib) | Process exit | `os.Exit(1)` on fatal errors | Must not call `os.Exit(0)` explicitly (normal exit is via `mgr.Start` returning nil) |
| Kubernetes REST config | Cluster connection | Obtained via `ctrl.GetConfigOrDie()` | Must not construct rest.Config manually |

Construction constraints:
- `DevPodReconciler` is constructed via struct literal assigning `Client: mgr.GetClient()` and `Scheme: mgr.GetScheme()`. This is the standard controller-runtime pattern; no factory is required.
- The package-level `scheme` must be populated in `init()` before `main()` executes. `utilruntime.Must()` wraps registration calls to panic on failure.

## Behavior

### Scheme Registration (init)

1. Registers all client-go core types (Pod, Service, etc.) into the package-level `scheme` via `clientgoscheme.AddToScheme`.
2. Registers `DevPod` and `DevPodList` types into the package-level `scheme` via `devpodv1.AddToScheme`.
3. Both registrations are wrapped in `utilruntime.Must()` — a failure panics the process before `main()` runs.

### Flag Parsing

4. Defines and parses the following CLI flags (see Inputs section for full details): metrics bind address, health probe bind address, leader election toggle, secure metrics toggle, metrics cert path/name/key, enable-http2 toggle, and zap logging flags.

### Logger Initialization

5. Initializes zap logger with `Development: true` as the hardcoded default. Zap flags (bound via `opts.BindFlags`) allow runtime override of log level and encoder settings.
6. Sets the initialized logger as the global controller-runtime logger via `ctrl.SetLogger`.

### TLS Configuration

7. Defines a `disableHTTP2` TLS config modifier that restricts `NextProtos` to `["http/1.1"]`.
8. When `--enable-http2` is `false` (the default), appends `disableHTTP2` to the shared TLS options slice.

### Metrics Server Configuration

9. Constructs `metricsserver.Options` with: bind address, secure serving flag, and TLS options.
10. When `--metrics-secure` is `true` (the default), assigns `filters.WithAuthenticationAndAuthorization` as the `FilterProvider` to protect the metrics endpoint with Kubernetes authn/authz.
11. When `--metrics-cert-path` is non-empty, configures cert directory, cert file name, and key file name on the metrics server options.

### Manager Creation

12. Calls `ctrl.GetConfigOrDie()` to obtain the in-cluster or kubeconfig-based REST config. Panics if unavailable (fail-fast).
13. Creates the controller-runtime manager with: scheme, metrics server options, health probe bind address, leader election toggle, and leader election ID `"cec0a61b.devpod.com"`.
14. If manager creation fails, logs the error and exits with code 1.

### Controller Wiring

15. Constructs `DevPodReconciler` with `Client` and `Scheme` obtained from the manager.
16. Calls `SetupWithManager(mgr)` to register the controller's watches and event handlers.
17. If controller setup fails, logs the error (with controller name "devpod") and exits with code 1.

### Probe Registration

18. Registers a liveness probe at path `"healthz"` using `healthz.Ping`.
19. Registers a readiness probe at path `"readyz"` using `healthz.Ping`.
20. If either registration fails, logs the error and exits with code 1.

### Manager Start

21. Logs `"Starting manager"`.
22. Calls `mgr.Start(ctrl.SetupSignalHandler())` — blocks until SIGTERM/SIGINT is received or a runtime error occurs.
23. If the manager returns an error, logs it and exits with code 1.
24. If the manager returns nil (graceful shutdown), the process exits normally (code 0).

## Inputs

### CLI Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--metrics-bind-address` | string | `"0"` (disabled) | Address the metrics endpoint binds to. Use `:8443` for HTTPS or `:8080` for HTTP. |
| `--health-probe-bind-address` | string | `":8081"` | Address the health/ready probe endpoint binds to. |
| `--leader-elect` | bool | `false` | Enable leader election for HA. Ensures only one active controller manager. |
| `--metrics-secure` | bool | `true` | Serve metrics via HTTPS. Set `false` for HTTP. |
| `--metrics-cert-path` | string | `""` (empty) | Directory containing the metrics server TLS certificate. Empty = auto-generated self-signed cert. |
| `--metrics-cert-name` | string | `"tls.crt"` | File name of the metrics server certificate within `--metrics-cert-path`. |
| `--metrics-cert-key` | string | `"tls.key"` | File name of the metrics server private key within `--metrics-cert-path`. |
| `--enable-http2` | bool | `false` | Enable HTTP/2 for the metrics server. Disabled by default for CVE mitigation. |
| zap flags | various | (see zap docs) | Bound via `zap.Options.BindFlags`. Includes `--zap-devel`, `--zap-log-level`, `--zap-encoder`, etc. |

### Implicit Inputs

| Input | Source | Description |
|---|---|---|
| Kubernetes REST config | In-cluster service account or `~/.kube/config` | Obtained via `ctrl.GetConfigOrDie()` |
| OS signals | SIGTERM, SIGINT | Trigger graceful shutdown via `ctrl.SetupSignalHandler()` |

## Outputs

| Output | Type | Description |
|---|---|---|
| Running operator process | side effect | Blocks on `mgr.Start()` until signal or error |
| Exit code 0 | process exit | Graceful shutdown (signal received, manager stopped cleanly) |
| Exit code 1 | process exit | Any setup error or manager runtime error |
| Log messages | stderr (console format) | Structured logs via zap; includes setup progress and error details |

## Invariants

- Must register all API types into the scheme (client-go core + devpodv1) **before** creating the manager.
- Must call `flag.Parse()` exactly once, after all flags are defined.
- Must initialize the global logger before any log-emitting operation.
- HTTP/2 must be disabled by default. Enabling requires explicit `--enable-http2=true`.
- Leader election ID must be `"cec0a61b.devpod.com"` (hardcoded).
- When `--metrics-secure` is true, the metrics endpoint must be protected with authn/authz via `FilterProvider`.
- Must wire all controllers before calling `mgr.Start()`.
- Must register both healthz and readyz probes before calling `mgr.Start()`.
- Must not return from `main()` on error without calling `os.Exit(1)`.
- Must not call `os.Exit` on success — normal termination is via `mgr.Start()` returning nil.
- `setupLog` must only be used during bootstrap (before `mgr.Start()`). Runtime logging is handled by controller-runtime's per-reconcile loggers.
- `utilruntime.Must()` in `init()` ensures scheme registration failures are unrecoverable panics — the process cannot start with an incomplete scheme.

## Edge Cases

- Condition: No kubeconfig available (neither in-cluster nor `~/.kube/config`).
  Expected: `ctrl.GetConfigOrDie()` panics. Process terminates immediately with a stack trace. This is intentional fail-fast behavior.

- Condition: Manager creation fails (e.g., invalid REST config, port conflict on probe address).
  Expected: Logs error via `setupLog.Error` with message "Failed to start manager". Exits with code 1.

- Condition: `DevPodReconciler.SetupWithManager` fails (e.g., duplicate controller name registration).
  Expected: Logs error with controller name "devpod". Exits with code 1.

- Condition: Health or ready probe registration fails.
  Expected: Logs error. Exits with code 1.

- Condition: Manager returns error at runtime (e.g., leader election lost, cache sync failure).
  Expected: Logs error with message "Failed to run manager". Exits with code 1.

- Condition: `--metrics-bind-address` is `"0"` (default).
  Expected: Metrics endpoint is not started. No port is bound for metrics.

- Condition: `--metrics-cert-path` is empty while `--metrics-secure` is true.
  Expected: Controller-runtime auto-generates a self-signed certificate for the metrics server.

- Condition: `--enable-http2` is true.
  Expected: The `disableHTTP2` TLS modifier is NOT appended. HTTP/2 is available on the metrics server.

## Related

- [DevPod Reconciler](../internal/controller/devpod_controller.md) — the sole controller wired by this entry point; its construction constraints are fulfilled here.
- [DevPod CRD Schema](../api/v1/devpod_types.md) — types registered into the scheme by `devpodv1.AddToScheme`.
- [GroupVersion Info](../api/v1/groupversion_info.md) — provides the `SchemeBuilder` used by `AddToScheme`.
