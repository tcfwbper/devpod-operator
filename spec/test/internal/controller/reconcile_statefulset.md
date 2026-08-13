# Test Specification: `reconcile_statefulset_test.go`

## Source File Under Test

`internal/controller/reconcile_statefulset.go`

## Test File

`internal/controller/reconcile_statefulset_test.go`

---

## `reconcileStatefulSet`

### Happy Path — reconcileStatefulSet

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileStatefulSet_CreatesWhenNotFound` | `unit` | Creates a new StatefulSet when none exists. | Fake client with no existing StatefulSet; DevPod with fully populated Spec; scheme registered | `ctx`, `devpod` | Returns non-nil StatefulSet matching `buildStatefulSet(devpod)` output; `devpod.com/spec-hash` annotation is set; owner reference points to DevPod |
| `TestReconcileStatefulSet_NoOpWhenHashMatches` | `unit` | No update when existing StatefulSet spec-hash matches desired. | Fake client with existing StatefulSet whose `devpod.com/spec-hash` matches the expected hash | `ctx`, `devpod` | Returns the existing StatefulSet unchanged; no Update issued |
| `TestReconcileStatefulSet_UpdatesMutableFieldsOnHashMismatch` | `unit` | Updates only mutable fields when hash differs. | Fake client with existing StatefulSet whose `devpod.com/spec-hash` differs; existing has different Template but same Selector/ServiceName/VolumeClaimTemplates | `ctx`, `devpod` | Returns updated StatefulSet with new Replicas, UpdateStrategy, Template from desired; spec-hash annotation updated; Selector, ServiceName, PodManagementPolicy, VolumeClaimTemplates remain unchanged from existing |
| `TestReconcileStatefulSet_DoesNotModifyImmutableFields` | `unit` | Selector, ServiceName, PodManagementPolicy, and VolumeClaimTemplates are never modified on existing StatefulSet. | Fake client with existing StatefulSet with hash mismatch; existing has Selector/ServiceName/VolumeClaimTemplates that differ from desired | `ctx`, `devpod` | After update: Selector, ServiceName, PodManagementPolicy, VolumeClaimTemplates remain as they were on the existing object |

### Null / Empty Input

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileStatefulSet_NilAnnotations` | `unit` | Treats nil annotations as hash mismatch and proceeds with update. | Fake client with existing StatefulSet whose Annotations map is nil | `ctx`, `devpod` | Update is issued; spec-hash annotation is set |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileStatefulSet_GetError` | `unit` | Returns error when Get fails with a non-NotFound error. | Fake client configured to return an error on Get | `ctx`, `devpod` | Returns nil StatefulSet and non-nil error |
| `TestReconcileStatefulSet_CreateError` | `unit` | Returns error when Create fails. | Fake client configured to return NotFound on Get and error on Create | `ctx`, `devpod` | Returns nil StatefulSet and non-nil error |
| `TestReconcileStatefulSet_UpdateError` | `unit` | Returns error when Update fails. | Fake client with existing StatefulSet (hash mismatch), configured to return error on Update | `ctx`, `devpod` | Returns nil StatefulSet and non-nil error |

### Mock / Dependency Interaction

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileStatefulSet_SetsOwnerReference` | `unit` | Sets DevPod as controller owner on creation. | Fake client with no existing StatefulSet; scheme registered with DevPod types | `ctx`, `devpod` | Created StatefulSet has OwnerReferences with Controller=true pointing to the DevPod |
| `TestReconcileStatefulSet_CallsBuildStatefulSet` | `unit` | Uses buildStatefulSet output as the desired manifest. | Fake client with no existing StatefulSet | `ctx`, `devpod` | Created StatefulSet matches the output of `buildStatefulSet(devpod)` (same Spec.Template, containers, volumes) |
