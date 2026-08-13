# StatefulSet Reconciliation

## Overview

Ensures an owned `StatefulSet` exists for the DevPod and keeps its mutable fields in sync with the desired state. Uses a spec-hash annotation for change detection to avoid update hot-loops caused by API server field defaulting.

Does **not** construct the StatefulSet manifest — that is delegated to the workload builder. Only handles the CRUD lifecycle and field-update constraints imposed by the Kubernetes StatefulSet API.

## Boundaries

- Owns: StatefulSet Get/Create/Update lifecycle, spec-hash change detection, and enforcement of the mutable-only update rule.
- Delegates: StatefulSet manifest construction to `build_statefulset`.
- Delegates: owner reference assignment to `ctrl.SetControllerReference`.
- Delegates: spec-hash computation to `helpers.specHash`.
- Must not: modify `Selector`, `ServiceName`, `PodManagementPolicy`, or `VolumeClaimTemplates` on an existing StatefulSet (these are immutable in the Kubernetes API).
- Must not: create any resource other than the StatefulSet.

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `client.Client` (from receiver) | Kubernetes API access | `Get`, `Create`, `Update` on StatefulSets | Must not delete StatefulSets |
| `ctrl.SetControllerReference` | Owner reference setup | Call once on the desired StatefulSet | — |
| `build_statefulset.buildStatefulSet` | Manifest rendering | Call to obtain the desired StatefulSet | Must not modify the returned object's immutable fields |
| `helpers.specHash` | Change detection | Call on `desired.Spec` to produce a hash | — |
| `helpers.setAnnotation` | Annotation update | Call to write the spec-hash annotation | — |

Construction constraints:
- This is a method on `DevPodReconciler`. No separate construction is needed.

## Behavior

1. Calls `buildStatefulSet(devpod)` to obtain the desired StatefulSet manifest.
2. Computes `specHash(desired.Spec)` and stores it as the `devpod.com/spec-hash` annotation.
3. Sets the DevPod as the controller owner.
4. Attempts to Get the existing StatefulSet by name.
5. If not found: Creates the desired StatefulSet and returns it.
6. If found and the spec-hash annotation matches: returns the existing StatefulSet (no-op).
7. If hashes differ: copies only the mutable fields from desired to existing:
   - `Spec.Replicas`
   - `Spec.UpdateStrategy`
   - `Spec.Template`
8. Updates the spec-hash annotation on the existing StatefulSet and issues an Update.
9. Returns the updated StatefulSet.

## Inputs

| Field | Type | Constraints | Required |
|---|---|---|---|
| ctx | context.Context | Carries logger and cancellation | Yes |
| devpod | *devpodv1.DevPod | Must have Name, Namespace, and a fully-populated Spec | Yes |

## Outputs

| Field | Type | Description |
|---|---|---|
| *appsv1.StatefulSet | pointer | The reconciled StatefulSet (created, unchanged, or updated) |
| error | error | Non-nil if Get/Create/Update/hash computation/SetControllerReference fails |

## Invariants

- Must never modify `Selector`, `ServiceName`, `PodManagementPolicy`, or `VolumeClaimTemplates` on an existing StatefulSet. The CRD marks the fields that feed these as immutable, preventing drift at the API level.
- Must compare spec-hashes before issuing an Update — if hashes match, no write is performed.
- The `devpod.com/spec-hash` annotation must always reflect the hash of the last-written spec.
- Must always set an owner reference pointing to the DevPod on creation.

## Edge Cases

- Condition: StatefulSet exists but annotations map is nil.
  Expected: Hash comparison treats nil annotation as "not matching" and proceeds with the update.

- Condition: Only `spec.image` changed on the DevPod (affects Template but not VolumeClaimTemplates).
  Expected: Hash changes; only `Template` is overwritten in the Update.

- Condition: The API server's admission controller modifies the StatefulSet (e.g., mutating webhook adds a sidecar).
  Expected: The operator's spec-hash is computed from what *it* wrote, not the live object. The next reconcile detects a mismatch and re-asserts the desired Template. External mutations are overwritten.

## Related

- [DevPod Reconciler](./devpod_controller.md) — caller; reads `ReadyReplicas` and `ObservedGeneration` from the returned StatefulSet.
- [build_statefulset](./build_statefulset.md) — produces the desired manifest consumed by this unit.
- [helpers](./helpers.md) — provides `specHash`, `setAnnotation`.
