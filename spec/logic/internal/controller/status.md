# Status Management

## Overview

Owns the condition state-machine and requeue-decision logic for DevPod status reporting. Provides four terminal-state helpers (`ready`, `progressing`, `pending`, `degraded`) that each set all three conditions atomically, update the status subresource, and return the appropriate `ctrl.Result`.

Does **not** decide which state to enter — that decision belongs to the main reconciler. This unit only translates a chosen state into consistent conditions and the correct requeue behavior.

## Boundaries

- Owns: setting all three condition types (`Ready`, `Progressing`, `Degraded`) atomically via `setConditions`.
- Owns: the `conditionSet` struct that prevents contradictory condition states.
- Owns: `updateStatus` which writes `status.observedGeneration` and issues a status subresource update.
- Owns: requeue decisions tied to each lifecycle phase:
  - `ready`: no requeue.
  - `progressing`: requeue after `requeueInterval`.
  - `pending`: no requeue (a watch event will trigger reconcile).
  - `degraded` with cause: return error for exponential backoff.
  - `degraded` without cause: requeue after `requeueInterval`.
- Must not: decide which condition to set (caller decides by calling the appropriate helper).
- Must not: modify any fields on the DevPod other than `Status.Conditions` and `Status.ObservedGeneration`.

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `client.Client` (from receiver) | Status subresource update | `r.Status().Update(ctx, devpod)` | Must not use `r.Update()` (that writes the full object, not just status) |
| `apimeta.SetStatusCondition` | Condition insertion | Call for each of the three condition types | — |
| `devpodv1` constants | Condition types and reasons | Reference `ConditionReady`, `ConditionProgressing`, `ConditionDegraded`, and all `Reason*` constants | — |

Construction constraints:
- These are methods on `DevPodReconciler`. No separate construction is needed.
- `requeueInterval` is a package-level constant = 10 seconds.

## Behavior

### conditionSet (struct)

1. Groups the status of all three conditions (`ready`, `progressing`, `degraded`) plus a shared `reason` and `message`. This ensures conditions are never set independently, preventing contradictory states.

### setConditions

2. For each of the three condition types (`Ready`, `Progressing`, `Degraded`), calls `apimeta.SetStatusCondition` with the corresponding status from the `conditionSet`, the shared reason/message, and the DevPod's current `Generation` as `ObservedGeneration`.

### updateStatus

3. Sets `devpod.Status.ObservedGeneration = devpod.Generation`.
4. Issues `r.Status().Update(ctx, devpod)` — writes the status subresource.

### ready

5. Sets conditions: Ready=True, Progressing=False, Degraded=False.
6. Message: `"DevPod is reachable on node port <sshNodePort>"`.
7. Reason: `ReasonDevPodReady`.
8. Calls `updateStatus`.
9. Returns `ctrl.Result{}, <updateStatus error>`.

### progressing

10. Sets conditions: Ready=False, Progressing=True, Degraded=False.
11. Message and reason: provided by caller.
12. Calls `updateStatus`. If error, returns it.
13. Returns `ctrl.Result{RequeueAfter: requeueInterval}, nil`.

### pending

14. Sets conditions: Ready=False, Progressing=False, Degraded=False.
15. Message and reason: provided by caller.
16. Calls `updateStatus`.
17. Returns `ctrl.Result{}, <updateStatus error>`. No requeue — a watch event on the Secret triggers the next reconcile.

### degraded

18. Sets conditions: Ready=False, Progressing=False, Degraded=True.
19. Message and reason: provided by caller.
20. Calls `updateStatus`. If error, returns it.
21. If `cause` (error parameter) is non-nil: returns `ctrl.Result{}, cause` — controller-runtime will retry with exponential backoff.
22. If `cause` is nil: returns `ctrl.Result{RequeueAfter: requeueInterval}, nil` — periodic poll for spec/Secret correction.

## Inputs

### ready

| Field | Type | Constraints | Required |
|---|---|---|---|
| ctx | context.Context | — | Yes |
| devpod | *devpodv1.DevPod | Must have Status.SSHNodePort set | Yes |

### progressing

| Field | Type | Constraints | Required |
|---|---|---|---|
| ctx | context.Context | — | Yes |
| devpod | *devpodv1.DevPod | — | Yes |
| reason | string | One of the devpodv1.Reason* constants | Yes |
| message | string | Human-readable detail | Yes |

### pending

| Field | Type | Constraints | Required |
|---|---|---|---|
| ctx | context.Context | — | Yes |
| devpod | *devpodv1.DevPod | — | Yes |
| reason | string | One of the devpodv1.Reason* constants | Yes |
| message | string | Human-readable detail | Yes |

### degraded

| Field | Type | Constraints | Required |
|---|---|---|---|
| ctx | context.Context | — | Yes |
| devpod | *devpodv1.DevPod | — | Yes |
| reason | string | One of the devpodv1.Reason* constants | Yes |
| message | string | Human-readable detail | Yes |
| cause | error | The underlying error, or nil if the problem is in the spec/Secret | No (nullable) |

## Outputs

All helpers return `(ctrl.Result, error)`.

| Helper | Result when successful | Error behavior |
|---|---|---|
| ready | `{}, nil` | Returns updateStatus error |
| progressing | `{RequeueAfter: 10s}, nil` | Returns updateStatus error |
| pending | `{}, nil` | Returns updateStatus error |
| degraded (cause!=nil) | `{}, cause` | Returns updateStatus error if status write fails; otherwise returns cause |
| degraded (cause==nil) | `{RequeueAfter: 10s}, nil` | Returns updateStatus error |

## Invariants

- At most one of `Ready`, `Progressing`, `Degraded` must be True at any given time. When the DevPod is pending human input, all three are False. The `conditionSet` struct enforces mutual exclusion by requiring all three to be specified together.
- `ObservedGeneration` on every condition must equal the DevPod's `.metadata.generation` at the time of the status write.
- `Status.ObservedGeneration` (top-level) must also equal `.metadata.generation`.
- `requeueInterval` is 10 seconds (hardcoded constant).
- Must never return both `RequeueAfter > 0` and a non-nil error simultaneously.
- Must use `r.Status().Update()` (status subresource), not `r.Update()` (full object).

## Edge Cases

- Condition: `updateStatus` fails (e.g., conflict due to stale resourceVersion).
  Expected: All helpers return the error. Controller-runtime retries the reconcile, which re-derives the correct state.

- Condition: `degraded` called with nil cause.
  Expected: Returns `RequeueAfter: 10s` without an error. This avoids exponential backoff for problems that the user must fix (e.g., missing StorageClass, invalid password).

- Condition: `degraded` called with non-nil cause.
  Expected: Returns the cause as an error. Controller-runtime applies exponential backoff.

- Condition: `devpod.Status.SSHNodePort` is 0 when `ready` is called.
  Expected: Message says "DevPod is reachable on node port 0". This should not happen in practice because the Service reconciliation precedes the ready call.

## Related

- [DevPod Reconciler](./devpod_controller.md) — sole caller of all status helpers.
- [DevPod CRD Schema](../../api/v1/devpod_types.md) — defines condition type and reason constants.
