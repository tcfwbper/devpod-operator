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
| `TestReconcileStatefulSet_CreatesWhenNotFound` | `unit` | Creates a new StatefulSet when none exists. | Fake client with no existing StatefulSet; DevPod with fully populated Spec; Secret with `.Data` populated; scheme registered | `ctx`, `devpod`, `secret` | Returns non-nil StatefulSet matching `buildStatefulSet(devpod)` output; `devpod.com/spec-hash` annotation is set on StatefulSet metadata; `apps.devpod.com/secret-hash` annotation is set on pod template; owner reference points to DevPod |
| `TestReconcileStatefulSet_NoOpWhenHashMatches` | `unit` | No update when existing StatefulSet spec-hash matches desired. | Fake client with existing StatefulSet whose `devpod.com/spec-hash` matches the expected hash (computed with Secret hash already embedded in pod template) | `ctx`, `devpod`, `secret` | Returns the existing StatefulSet unchanged; no Update issued |
| `TestReconcileStatefulSet_UpdatesMutableFieldsOnHashMismatch` | `unit` | Updates only mutable fields when hash differs. | Fake client with existing StatefulSet whose `devpod.com/spec-hash` differs; existing has different Template but same Selector/ServiceName/VolumeClaimTemplates | `ctx`, `devpod`, `secret` | Returns updated StatefulSet with new Replicas, UpdateStrategy, Template from desired; spec-hash annotation updated; Selector, ServiceName, PodManagementPolicy, VolumeClaimTemplates remain unchanged from existing |
| `TestReconcileStatefulSet_DoesNotModifyImmutableFields` | `unit` | Selector, ServiceName, PodManagementPolicy, and VolumeClaimTemplates are never modified on existing StatefulSet. | Fake client with existing StatefulSet with hash mismatch; existing has Selector/ServiceName/VolumeClaimTemplates that differ from desired | `ctx`, `devpod`, `secret` | After update: Selector, ServiceName, PodManagementPolicy, VolumeClaimTemplates remain as they were on the existing object |
| `TestReconcileStatefulSet_SecretHashOnPodTemplate` | `unit` | Secret hash annotation is placed on the pod template, not on the StatefulSet metadata. | Fake client with no existing StatefulSet; Secret with `.Data = map[string][]byte{"key": []byte("val")}` | `ctx`, `devpod`, `secret` | Created StatefulSet's `Spec.Template.ObjectMeta.Annotations` contains `apps.devpod.com/secret-hash` matching `secretDataHash(secret.Data)`; StatefulSet's own ObjectMeta annotations do NOT contain `apps.devpod.com/secret-hash` |
| `TestReconcileStatefulSet_SecretHashInjectedBeforeSpecHash` | `unit` | The spec-hash captures the Secret hash because it is embedded in the Template before spec-hash computation. | Fake client with no existing StatefulSet; Secret with known `.Data` | `ctx`, `devpod`, `secret` | The `devpod.com/spec-hash` annotation equals `specHash(desired.Spec)` where `desired.Spec.Template.ObjectMeta.Annotations` already contains the Secret hash |

### Happy Path — Secret content change triggers update

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileStatefulSet_SecretChangeTriggersUpdate` | `unit` | StatefulSet is updated when Secret data changes but DevPod spec is unchanged. | Fake client with existing StatefulSet created with Secret data `{"pw": []byte("old")}` (spec-hash computed with that Secret hash); now called with Secret data `{"pw": []byte("new")}` | `ctx`, `devpod`, `secret` (with new data) | Update is issued; pod template annotation `apps.devpod.com/secret-hash` reflects new Secret data; spec-hash annotation is updated |
| `TestReconcileStatefulSet_LegacyStatefulSetWithoutSecretHash` | `unit` | Existing StatefulSet lacking the Secret-hash pod template annotation triggers a one-time update. | Fake client with existing StatefulSet that has a `devpod.com/spec-hash` computed without Secret hash in template (simulating pre-upgrade state) | `ctx`, `devpod`, `secret` | Update is issued; pod template now contains `apps.devpod.com/secret-hash`; spec-hash annotation is refreshed |

### Null / Empty Input

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileStatefulSet_NilAnnotations` | `unit` | Treats nil annotations as hash mismatch and proceeds with update. | Fake client with existing StatefulSet whose Annotations map is nil | `ctx`, `devpod`, `secret` | Update is issued; spec-hash annotation is set; pod template has Secret hash annotation |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileStatefulSet_GetError` | `unit` | Returns error when Get fails with a non-NotFound error. | Fake client configured to return an error on Get | `ctx`, `devpod`, `secret` | Returns nil StatefulSet and non-nil error |
| `TestReconcileStatefulSet_CreateError` | `unit` | Returns error when Create fails. | Fake client configured to return NotFound on Get and error on Create | `ctx`, `devpod`, `secret` | Returns nil StatefulSet and non-nil error |
| `TestReconcileStatefulSet_UpdateError` | `unit` | Returns error when Update fails. | Fake client with existing StatefulSet (hash mismatch), configured to return error on Update | `ctx`, `devpod`, `secret` | Returns nil StatefulSet and non-nil error |

### Mock / Dependency Interaction

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileStatefulSet_SetsOwnerReference` | `unit` | Sets DevPod as controller owner on creation. | Fake client with no existing StatefulSet; scheme registered with DevPod types | `ctx`, `devpod`, `secret` | Created StatefulSet has OwnerReferences with Controller=true pointing to the DevPod |
| `TestReconcileStatefulSet_CallsBuildStatefulSet` | `unit` | Uses buildStatefulSet output as the desired manifest. | Fake client with no existing StatefulSet | `ctx`, `devpod`, `secret` | Created StatefulSet matches the output of `buildStatefulSet(devpod)` (same Spec.Template containers, volumes) with the addition of the Secret hash annotation on pod template |
| `TestReconcileStatefulSet_CallsSecretDataHash` | `unit` | Calls secretDataHash with the Secret's .Data field. | Fake client with no existing StatefulSet; Secret with `.Data = map[string][]byte{"ubuntu-password": []byte("secret123")}` | `ctx`, `devpod`, `secret` | Created StatefulSet pod template annotation `apps.devpod.com/secret-hash` equals the output of `secretDataHash(secret.Data)` |
