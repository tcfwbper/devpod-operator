# Test Specification: `devpod_controller_test.go`

## Source File Under Test

`internal/controller/devpod_controller.go`

## Test File

`internal/controller/devpod_controller_test.go`

---

## `Reconcile`

### Happy Path — Reconcile

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcile_FullHappyPath` | `unit` | All sub-reconcilers succeed and StatefulSet is ready; reports Ready. | Fake client with DevPod (finalizer present), Secret with valid 8+ char password, StorageClass exists, ServiceAccount/Service/StatefulSet sub-reconcilers succeed; StatefulSet has ReadyReplicas=1 and ObservedGeneration==Generation | `ctx`, `ctrl.Request{NamespacedName}` | Returns `ctrl.Result{}`, nil; DevPod status has Ready=True |
| `TestReconcile_AddsFinalizer` | `unit` | Adds finalizer on first reconcile and returns immediately. | Fake client with DevPod (no finalizer, no DeletionTimestamp) | `ctx`, `ctrl.Request{NamespacedName}` | DevPod is updated with finalizer `"apps.devpod.com/finalizer"`; returns without creating any sub-resources |
| `TestReconcile_ProgressingWhenStatefulSetNotReady` | `unit` | Reports Progressing when StatefulSet has 0 ready replicas. | Fake client with DevPod (finalizer present), valid password, StorageClass exists, all sub-reconcilers succeed; StatefulSet has ReadyReplicas=0 | `ctx`, `ctrl.Request{NamespacedName}` | Returns `ctrl.Result{RequeueAfter: 10s}`, nil; status has Progressing=True with reason WaitingForStatefulSet |
| `TestReconcile_ProgressingWhenObservedGenerationLags` | `unit` | Reports Progressing when StatefulSet ObservedGeneration < Generation. | Fake client with DevPod; StatefulSet with ObservedGeneration=1, Generation=2, ReadyReplicas=1 | `ctx`, `ctrl.Request{NamespacedName}` | Returns `ctrl.Result{RequeueAfter: 10s}`, nil; status has Progressing=True |

### Happy Path — status fields

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcile_WritesPasswordSecret` | `unit` | Writes status.passwordSecret from reconcileSecret result. | Fake client with DevPod, Secret named "dev1" | `ctx`, `ctrl.Request` | After reconcile: `devpod.Status.PasswordSecret == "dev1"` |
| `TestReconcile_WritesSSHNodePort` | `unit` | Writes status.sshNodePort from reconcileService result. | Fake client with DevPod; Service has port 22 with NodePort=30022 | `ctx`, `ctrl.Request` | After reconcile: `devpod.Status.SSHNodePort == 30022` |
| `TestReconcile_WritesReadyReplicas` | `unit` | Writes status.readyReplicas from reconcileStatefulSet result. | Fake client with DevPod; StatefulSet has Status.ReadyReplicas=1 | `ctx`, `ctrl.Request` | After reconcile: `devpod.Status.ReadyReplicas == 1` |

### State Transitions

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcile_PendingWhenPasswordEmpty` | `unit` | Reports Pending when password is empty string. | Fake client with DevPod (finalizer present), Secret exists with `ubuntu-password=""` | `ctx`, `ctrl.Request` | Returns `ctrl.Result{}`, nil (no requeue); status has all conditions False with reason PasswordNotSet |
| `TestReconcile_DegradedWhenPasswordInvalid` | `unit` | Reports Degraded when password fails validation. | Fake client with DevPod, Secret exists with `ubuntu-password="short"` (7 chars) | `ctx`, `ctrl.Request` | Returns `ctrl.Result{RequeueAfter: 10s}`, nil; status has Degraded=True with reason InvalidPassword |
| `TestReconcile_DegradedWhenStorageClassNotFound` | `unit` | Reports Degraded when StorageClass does not exist. | Fake client with DevPod, valid password, but no StorageClass matching `spec.persistence.storageClass` | `ctx`, `ctrl.Request` | Returns `ctrl.Result{RequeueAfter: 10s}`, nil; status has Degraded=True with reason StorageClassNotFound |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcile_NotFound` | `unit` | Returns success when DevPod CR not found. | Fake client with no DevPod for the requested NamespacedName | `ctx`, `ctrl.Request` | Returns `ctrl.Result{}`, nil |
| `TestReconcile_SecretReconcileError` | `unit` | Reports Degraded/ReconcileError and returns error when reconcileSecret fails. | Fake client with DevPod; Secret Get returns transient error | `ctx`, `ctrl.Request` | Status has Degraded=True with reason ReconcileError; returns non-nil error |
| `TestReconcile_ServiceAccountReconcileError` | `unit` | Reports Degraded/ReconcileError when reconcileServiceAccount fails. | Fake client with DevPod, valid password, StorageClass exists; ServiceAccount Create returns error | `ctx`, `ctrl.Request` | Status has Degraded=True with reason ReconcileError; returns non-nil error |
| `TestReconcile_ServiceReconcileError` | `unit` | Reports Degraded/ReconcileError when reconcileService fails. | Fake client with DevPod, valid password, StorageClass exists, ServiceAccount OK; Service Create returns error | `ctx`, `ctrl.Request` | Status has Degraded=True with reason ReconcileError; returns non-nil error |
| `TestReconcile_StatefulSetReconcileError` | `unit` | Reports Degraded/ReconcileError when reconcileStatefulSet fails. | Fake client with DevPod, valid password, StorageClass exists, ServiceAccount/Service OK; StatefulSet Create returns error | `ctx`, `ctrl.Request` | Status has Degraded=True with reason ReconcileError; returns non-nil error |
| `TestReconcile_StorageClassGetError` | `unit` | Reports Degraded/ReconcileError on transient StorageClass Get error (not NotFound). | Fake client with DevPod, valid password; StorageClass Get returns internal server error | `ctx`, `ctrl.Request` | Status has Degraded=True with reason ReconcileError; returns non-nil error |

---

## `Finalize`

### Happy Path — Finalize

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestFinalize_DeletesPVCsWhenReclaimDelete` | `unit` | Deletes all matching PVCs when ReclaimPolicy is Delete. | Fake client with DevPod (DeletionTimestamp set, finalizer present, `Spec.Persistence.ReclaimPolicy="Delete"`); two PVCs with matching selectorLabels in same namespace | `ctx`, `ctrl.Request` | Both PVCs are deleted; finalizer is removed from DevPod |
| `TestFinalize_RetainsPVCsWhenReclaimRetain` | `unit` | Does not delete PVCs when ReclaimPolicy is Retain. | Fake client with DevPod (DeletionTimestamp set, finalizer present, `Spec.Persistence.ReclaimPolicy="Retain"`); PVCs exist | `ctx`, `ctrl.Request` | PVCs are NOT deleted; finalizer is removed from DevPod |
| `TestFinalize_RemovesFinalizer` | `unit` | Removes the finalizer after cleanup. | Fake client with DevPod (DeletionTimestamp set, finalizer present) | `ctx`, `ctrl.Request` | DevPod no longer has `"apps.devpod.com/finalizer"` in its finalizers list |
| `TestFinalize_NoFinalizerPresent` | `unit` | Returns immediately when finalizer is absent on a deleting resource. | Fake client with DevPod (DeletionTimestamp set, no finalizer) | `ctx`, `ctrl.Request` | Returns `ctrl.Result{}`, nil immediately; no PVC deletion attempted |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestFinalize_PVCDeleteNotFoundIgnored` | `unit` | Ignores NotFound errors during PVC deletion. | Fake client with DevPod (deleting, ReclaimPolicy=Delete); PVC List returns items but Delete returns NotFound | `ctx`, `ctrl.Request` | No error returned; finalizer still removed |
| `TestFinalize_PVCListError` | `unit` | Returns error when PVC list fails. | Fake client with DevPod (deleting, ReclaimPolicy=Delete); List returns error | `ctx`, `ctrl.Request` | Returns non-nil error; finalizer NOT removed |

---

## `SetupWithManager`

### Happy Path — SetupWithManager

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestSetupWithManager_Registers` | `unit` | Registers the controller with watches on DevPod and all owned types. | A test Manager (e.g., from `ctrl.NewManager` with fake config or envtest) | `mgr` | Returns nil error; controller is named `"devpod"` |
