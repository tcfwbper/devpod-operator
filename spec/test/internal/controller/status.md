# Test Specification: `status_test.go`

## Source File Under Test

`internal/controller/status.go`

## Test File

`internal/controller/status_test.go`

---

## `conditionSet`

### Happy Path — Construction

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestConditionSet_MutualExclusion` | `unit` | Only one of Ready/Progressing/Degraded can be True at a time. | | Construct a conditionSet with ready=True, progressing=False, degraded=False | Fields reflect mutual exclusion: exactly one True |

---

## `setConditions`

### Happy Path — setConditions

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestSetConditions_SetsAllThreeTypes` | `unit` | All three condition types are set on the DevPod status. | A DevPod with empty Conditions; a conditionSet with ready=True, progressing=False, degraded=False, reason="ReasonDevPodReady", message="msg" | | DevPod.Status.Conditions contains 3 entries with types "Ready" (True), "Progressing" (False), "Degraded" (False); all have ObservedGeneration matching DevPod.Generation |
| `TestSetConditions_OverwritesPreviousConditions` | `unit` | Existing conditions are updated in place. | A DevPod with pre-existing "Ready"=True condition; a conditionSet with ready=False, degraded=True | | "Ready" condition is now False; "Degraded" is True |

---

## `updateStatus`

### Happy Path — updateStatus

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestUpdateStatus_SetsObservedGeneration` | `unit` | Sets Status.ObservedGeneration to metadata.Generation. | Fake client; DevPod with `Generation = 5` | `ctx`, `devpod` | After call: `devpod.Status.ObservedGeneration == 5`; Status().Update() was called |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestUpdateStatus_ReturnsError` | `unit` | Returns error when status update fails. | Fake client configured to return error on Status().Update() | `ctx`, `devpod` | Returns non-nil error |

---

## `ready`

### Happy Path — ready

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReady_SetsCorrectConditions` | `unit` | Sets Ready=True, Progressing=False, Degraded=False with correct reason and message. | Fake client; DevPod with `Status.SSHNodePort = 30022`, `Generation = 3` | `ctx`, `devpod` | Conditions: Ready=True (reason=ReasonDevPodReady, message contains "30022"), Progressing=False, Degraded=False; ObservedGeneration=3 on all conditions |
| `TestReady_ReturnsNoRequeue` | `unit` | Returns zero Result (no requeue). | Fake client; valid DevPod | `ctx`, `devpod` | Returns `ctrl.Result{}` with zero RequeueAfter and nil error |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReady_StatusUpdateError` | `unit` | Returns error when status update fails. | Fake client configured to fail on Status().Update() | `ctx`, `devpod` | Returns non-nil error |

---

## `progressing`

### Happy Path — progressing

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestProgressing_SetsCorrectConditions` | `unit` | Sets Ready=False, Progressing=True, Degraded=False. | Fake client; DevPod with `Generation = 2` | `ctx`, `devpod`, `reason="WaitingForStatefulSet"`, `message="rolling out"` | Conditions: Ready=False, Progressing=True (reason=WaitingForStatefulSet), Degraded=False |
| `TestProgressing_RequeuesAfterInterval` | `unit` | Returns Result with RequeueAfter=10s. | Fake client; valid DevPod | `ctx`, `devpod`, reason, message | Returns `ctrl.Result{RequeueAfter: 10*time.Second}` and nil error |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestProgressing_StatusUpdateError` | `unit` | Returns error when status update fails. | Fake client configured to fail on Status().Update() | `ctx`, `devpod`, reason, message | Returns non-nil error |

---

## `pending`

### Happy Path — pending

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestPending_SetsAllConditionsFalse` | `unit` | Sets Ready=False, Progressing=False, Degraded=False. | Fake client; DevPod with `Generation = 1` | `ctx`, `devpod`, `reason="PasswordNotSet"`, `message="waiting"` | All three conditions are False with correct reason/message |
| `TestPending_ReturnsNoRequeue` | `unit` | Returns zero Result (no requeue). | Fake client; valid DevPod | `ctx`, `devpod`, reason, message | Returns `ctrl.Result{}` with zero RequeueAfter and nil error |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestPending_StatusUpdateError` | `unit` | Returns error when status update fails. | Fake client configured to fail on Status().Update() | `ctx`, `devpod`, reason, message | Returns non-nil error |

---

## `degraded`

### Happy Path — degraded

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestDegraded_WithCause_SetsConditionsAndReturnsError` | `unit` | Sets Ready=False, Progressing=False, Degraded=True and returns the cause. | Fake client; DevPod; `cause = errors.New("api timeout")` | `ctx`, `devpod`, `reason="ReconcileError"`, `message="failed"`, `cause` | Conditions: Degraded=True; returns `ctrl.Result{}` and `cause` error |
| `TestDegraded_NilCause_RequeuesAfterInterval` | `unit` | When cause is nil, returns RequeueAfter=10s with no error. | Fake client; DevPod | `ctx`, `devpod`, `reason="StorageClassNotFound"`, `message="sc missing"`, `cause=nil` | Conditions: Degraded=True; returns `ctrl.Result{RequeueAfter: 10*time.Second}` and nil error |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestDegraded_StatusUpdateError` | `unit` | Returns status update error regardless of cause. | Fake client configured to fail on Status().Update(); `cause = errors.New("original")` | `ctx`, `devpod`, reason, message, cause | Returns the status update error (not the cause) |
