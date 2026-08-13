# Test Specification: `reconcile_service_account_test.go`

## Source File Under Test

`internal/controller/reconcile_service_account.go`

## Test File

`internal/controller/reconcile_service_account_test.go`

---

## `reconcileServiceAccount`

### Happy Path — reconcileServiceAccount

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileServiceAccount_CreatesWhenNotFound` | `unit` | Creates a new ServiceAccount when none exists. | Fake client with no existing ServiceAccount; DevPod `Name="dev1"`, `Namespace="ns"` | `ctx`, `devpod` | Returns nil error; ServiceAccount created with `Name="dev1"`, `Namespace="ns"`, `AutomountServiceAccountToken=false`, labels match `standardLabels(devpod)`, owner reference points to DevPod |
| `TestReconcileServiceAccount_NoOpWhenCorrect` | `unit` | No update when existing ServiceAccount already has AutomountServiceAccountToken=false. | Fake client with existing ServiceAccount where `AutomountServiceAccountToken = false` | `ctx`, `devpod` | Returns nil error; no Update is issued |
| `TestReconcileServiceAccount_FixesNilAutomount` | `unit` | Updates when AutomountServiceAccountToken is nil. | Fake client with existing ServiceAccount where `AutomountServiceAccountToken = nil` | `ctx`, `devpod` | Returns nil error; Update issued setting `AutomountServiceAccountToken = false` |
| `TestReconcileServiceAccount_FixesTrueAutomount` | `unit` | Updates when AutomountServiceAccountToken is true. | Fake client with existing ServiceAccount where `AutomountServiceAccountToken = true` | `ctx`, `devpod` | Returns nil error; Update issued setting `AutomountServiceAccountToken = false` |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileServiceAccount_GetError` | `unit` | Returns error when Get fails with a non-NotFound error. | Fake client configured to return an error on Get | `ctx`, `devpod` | Returns non-nil error |
| `TestReconcileServiceAccount_CreateError` | `unit` | Returns error when Create fails. | Fake client configured to return NotFound on Get and error on Create | `ctx`, `devpod` | Returns non-nil error |
| `TestReconcileServiceAccount_UpdateError` | `unit` | Returns error when Update fails. | Fake client with existing ServiceAccount (nil automount), configured to return error on Update | `ctx`, `devpod` | Returns non-nil error |

### Mock / Dependency Interaction

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileServiceAccount_SetsOwnerReference` | `unit` | Sets DevPod as controller owner on creation. | Fake client with no existing ServiceAccount; scheme registered with DevPod types | `ctx`, `devpod` | Created ServiceAccount has OwnerReferences with Controller=true pointing to the DevPod |
