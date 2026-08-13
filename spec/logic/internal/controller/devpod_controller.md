# DevPod Reconciler

## Overview

Orchestrates the reconciliation of a `DevPod` custom resource toward its desired state. It sequences precondition checks, delegates sub-resource reconciliation to dedicated units, reads back observed state, and reports the overall lifecycle phase through status conditions.

Does **not** construct Kubernetes resource manifests directly — that is delegated to the sub-reconcile and builder units. Does **not** own the condition state-machine logic — that is delegated to the status unit.

## Boundaries

- Owns: the top-level `Reconcile()` control flow (fetch CR, check deletion, ensure finalizer, gate on preconditions, sequence sub-reconcilers, derive readiness).
- Owns: finalizer registration and the `finalize()` cleanup path (PVC deletion based on ReclaimPolicy).
- Owns: `SetupWithManager()` — wiring watches for the DevPod and all owned resource types.
- Owns: the `DevPodReconciler` struct definition (embeds `client.Client` and `*runtime.Scheme`).
- Delegates: Secret existence and password validation to `reconcile_secret`.
- Delegates: ServiceAccount reconciliation to `reconcile_service_account`.
- Delegates: Service reconciliation to `reconcile_service`.
- Delegates: StatefulSet CRUD to `reconcile_statefulset`.
- Delegates: status condition writing and requeue-decision to `status`.
- Must not: build Kubernetes resource manifests inline (must call sub-reconcilers or builder).
- Must not: write status conditions directly (must call status helpers).
- Must not: validate the password itself (delegates to `reconcile_secret`'s validation).

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `client.Client` (embedded) | Kubernetes API access | `Get`, `List`, `Update`, `Delete`, `Status().Update()` on DevPod and PVCs in finalize | Must not `Create` resources directly — that is done by sub-reconcilers |
| `*runtime.Scheme` | Owner reference setup | Passed through to sub-reconcilers | Must not use for type introspection at runtime |
| `reconcile_secret` | Secret sub-reconciler | Call `reconcileSecret(ctx, devpod)` and `validatePassword(password)` | Must not read or write Secret data outside these calls |
| `reconcile_service_account` | ServiceAccount sub-reconciler | Call `reconcileServiceAccount(ctx, devpod)` | — |
| `reconcile_service` | Service sub-reconciler | Call `reconcileService(ctx, devpod)` | — |
| `reconcile_statefulset` | StatefulSet sub-reconciler | Call `reconcileStatefulSet(ctx, devpod)` | — |
| `status` | Condition and requeue management | Call `ready()`, `progressing()`, `pending()`, `degraded()` | Must not call `apimeta.SetStatusCondition` directly |
| `helpers` | Shared utilities | Call `sshNodePort()`, `selectorLabels()` | — |
| `controller-runtime` | Framework | `ctrl.Request`, `ctrl.Result`, `ctrl.NewControllerManagedBy`, `controllerutil.ContainsFinalizer/AddFinalizer/RemoveFinalizer`, `log.FromContext` | Must not use low-level informer or workqueue APIs directly |
| `storagev1.StorageClass` | Precondition check | `r.Get()` to verify existence | Must not create or modify StorageClasses |

Construction constraints:
- `DevPodReconciler` is constructed in `cmd/main.go` by assigning `Client` and `Scheme` from the manager. No factory is required; struct literal is the standard pattern for controller-runtime reconcilers.

## Behavior

### Reconcile Flow

1. Fetches the `DevPod` CR by namespaced name. If not found (already deleted), returns success with no requeue.
2. If the CR has a non-zero `DeletionTimestamp`, branches to the finalize path.
3. If the finalizer `apps.devpod.com/finalizer` is not present, adds it and returns (the resulting update event triggers a fresh reconcile).
4. Calls `reconcileSecret`. On error, reports `Degraded/ReconcileError` via the status helper. Writes `status.passwordSecret` from the returned Secret name.
5. Reads the password from the Secret's `ubuntu-password` key. If empty, reports `Pending/PasswordNotSet` (no requeue — the Secret watch will wake reconcile). If invalid (per `validatePassword`), reports `Degraded/InvalidPassword` (periodic requeue, no returned error).
6. Verifies the requested `spec.persistence.storageClass` exists by fetching the `StorageClass`. If not found, reports `Degraded/StorageClassNotFound`. Other errors report `Degraded/ReconcileError`.
7. Calls `reconcileServiceAccount`. On error, reports `Degraded/ReconcileError`.
8. Calls `reconcileService`. On error, reports `Degraded/ReconcileError`. Writes `status.sshNodePort` from the returned Service.
9. Calls `reconcileStatefulSet`. On error, reports `Degraded/ReconcileError`. Writes `status.readyReplicas` from the returned StatefulSet.
10. Checks rollout: if `statefulSet.Status.ObservedGeneration < statefulSet.Generation` or `ReadyReplicas < 1`, reports `Progressing/WaitingForStatefulSet`.
11. Otherwise, reports `Ready/DevPodReady`.

### Finalize Flow

12. If the finalizer is absent, returns immediately (nothing to clean up).
13. If `spec.persistence.reclaimPolicy` is `Delete`, lists all PVCs in the DevPod's namespace matching `selectorLabels(devpod)` and deletes each one (skipping those already terminating). NotFound errors are ignored.
14. Removes the finalizer from the DevPod and issues an Update. The garbage collector handles Secret, ServiceAccount, Service, and StatefulSet via owner references.

### Manager Setup

15. `SetupWithManager` registers the controller to watch: `DevPod` (primary), and owned `Secret`, `ServiceAccount`, `Service`, `StatefulSet` (via `Owns()`). Controller is named `"devpod"`.

## Inputs

| Field | Type | Constraints | Required |
|---|---|---|---|
| ctx | context.Context | Carries logger and cancellation | Yes |
| req | ctrl.Request | NamespacedName of the DevPod | Yes |

## Outputs

| Field | Type | Description |
|---|---|---|
| ctrl.Result | struct | RequeueAfter set to `requeueInterval` (10s) when progressing or degraded-without-cause; zero otherwise |
| error | error | Non-nil only for transient errors that should trigger exponential backoff retry |

Side effects:
- Adds/removes finalizer on the DevPod.
- Deletes PVCs during finalization (when ReclaimPolicy=Delete).
- Updates DevPod status subresource (via status helpers).

## Invariants

- Must add the finalizer **before** creating any owned resource that the finalizer is responsible for cleaning up.
- Must never create owned resources while `DeletionTimestamp` is non-zero.
- At most one of Ready, Progressing, Degraded must be True at any given time. When the DevPod is pending human input (e.g., password not yet set), all three are False. The `conditionSet` struct enforces mutual exclusion.
- Must not return both a non-zero `RequeueAfter` and a non-nil error from the same reconcile invocation.
- Must not delete PVCs when `ReclaimPolicy` is `Retain`.
- The set of watched types in `SetupWithManager` must exactly match the set of resource types this controller creates via sub-reconcilers: Secret, ServiceAccount, Service, StatefulSet.
- Must update `status.observedGeneration` to `metadata.generation` on every status write (delegated to status helpers).

## Edge Cases

- Condition: DevPod CR not found (deleted between enqueue and reconcile).
  Expected: Returns `ctrl.Result{}, nil` — no error, no requeue.

- Condition: Finalizer not yet present on first reconcile.
  Expected: Adds finalizer and returns immediately. No sub-resources are created in this pass.

- Condition: Password is empty string (Secret exists but key is blank).
  Expected: Reports Pending/PasswordNotSet. Does not create StatefulSet. Does not requeue (watch on Secret triggers next reconcile).

- Condition: Password contains ":" or newline, or is shorter than 8 characters.
  Expected: Reports Degraded/InvalidPassword. No workload deployed. Requeues periodically.

- Condition: StorageClass does not exist in cluster.
  Expected: Reports Degraded/StorageClassNotFound. No workload deployed. Requeues periodically.

- Condition: StatefulSet exists but has 0 ready replicas.
  Expected: Reports Progressing/WaitingForStatefulSet. Requeues after `requeueInterval`.

- Condition: PVC deletion during finalize returns NotFound.
  Expected: Ignored (PVC already gone). Finalizer removal proceeds.

- Condition: Multiple reconciles racing on the same DevPod.
  Expected: Controller-runtime serializes reconciles for the same key. No special handling needed.

## Related

- [reconcile_secret](./reconcile_secret.md) — owns Secret lifecycle and password validation.
- [reconcile_service_account](./reconcile_service_account.md) — owns ServiceAccount lifecycle.
- [reconcile_service](./reconcile_service.md) — owns NodePort Service lifecycle.
- [reconcile_statefulset](./reconcile_statefulset.md) — owns StatefulSet CRUD.
- [build_statefulset](./build_statefulset.md) — renders the desired StatefulSet manifest.
- [status](./status.md) — owns condition state-machine and requeue decisions.
- [helpers](./helpers.md) — shared labeling, hashing, and port utilities.
- [DevPod CRD Schema](../../api/v1/devpod_types.md) — defines all types consumed by this controller.
