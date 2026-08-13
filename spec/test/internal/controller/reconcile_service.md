# Test Specification: `reconcile_service_test.go`

## Source File Under Test

`internal/controller/reconcile_service.go`

## Test File

`internal/controller/reconcile_service_test.go`

---

## `reconcileService`

### Happy Path — reconcileService

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileService_CreatesWhenNotFound` | `unit` | Creates a new NodePort Service when none exists. | Fake client with no existing Service; DevPod `Name="dev1"`, `Namespace="ns"`, `Spec.NodePorts=[{Src:22, Dest:30022}, {Src:8080, Dest:30080}]` | `ctx`, `devpod` | Returns non-nil Service with `Name="dev1"`, Type=NodePort, Selector=selectorLabels, Ports contain `{Name:"ssh", Port:22, TargetPort:22, NodePort:30022, Protocol:TCP}` and `{Name:"port-8080", Port:8080, TargetPort:8080, NodePort:30080, Protocol:TCP}`, ExternalTrafficPolicy=Cluster, SessionAffinity=None; owner reference points to DevPod; `devpod.com/spec-hash` annotation is set |
| `TestReconcileService_NoOpWhenHashMatches` | `unit` | No update when existing Service spec-hash matches desired. | Fake client with existing Service whose `devpod.com/spec-hash` annotation matches the computed hash for the same DevPod spec | `ctx`, `devpod` | Returns the existing Service; no Update issued |
| `TestReconcileService_UpdatesWhenHashDiffers` | `unit` | Updates mutable fields when spec-hash differs. | Fake client with existing Service with outdated `devpod.com/spec-hash`, ClusterIP="10.0.0.1" | `ctx`, `devpod` with changed NodePorts | Returns updated Service with new ports, new spec-hash annotation; `ClusterIP` is preserved as "10.0.0.1" |
| `TestReconcileService_PreservesClusterIP` | `unit` | Never overwrites ClusterIP during update. | Fake client with existing Service, `ClusterIP="10.96.0.5"`, outdated hash | `ctx`, `devpod` | Updated Service still has `ClusterIP="10.96.0.5"` |

### Null / Empty Input

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileService_NilAnnotations` | `unit` | Treats nil annotations as hash mismatch and proceeds with update. | Fake client with existing Service whose Annotations map is nil | `ctx`, `devpod` | Update is issued with spec-hash annotation set |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileService_GetError` | `unit` | Returns error when Get fails with a non-NotFound error. | Fake client configured to return an error on Get | `ctx`, `devpod` | Returns nil Service and non-nil error |
| `TestReconcileService_CreateError` | `unit` | Returns error when Create fails. | Fake client configured to return NotFound on Get and error on Create | `ctx`, `devpod` | Returns nil Service and non-nil error |
| `TestReconcileService_UpdateError` | `unit` | Returns error when Update fails. | Fake client with existing Service (hash mismatch), configured to return error on Update | `ctx`, `devpod` | Returns nil Service and non-nil error |
| `TestReconcileService_HashComputationError` | `unit` | Returns error when specHash fails. | Setup that causes `specHash` to fail (e.g., inject unmarshalable field — if the test seam exists) | `ctx`, `devpod` | Returns nil Service and non-nil error wrapping the hash error |

### Mock / Dependency Interaction

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileService_SetsOwnerReference` | `unit` | Sets DevPod as controller owner on creation. | Fake client with no existing Service; scheme registered with DevPod types | `ctx`, `devpod` | Created Service has OwnerReferences with Controller=true pointing to the DevPod |
| `TestReconcileService_LabelsAndSelector` | `unit` | Service uses standardLabels for metadata and selectorLabels for pod selector. | Fake client with no existing Service | `ctx`, `devpod` | Created Service `.Labels` equals `standardLabels(devpod)`, `.Spec.Selector` equals `selectorLabels(devpod)` |
