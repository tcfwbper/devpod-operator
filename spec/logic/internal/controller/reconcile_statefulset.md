# StatefulSet Reconciliation

## Overview

Ensures an owned `StatefulSet` exists for the DevPod and keeps its mutable fields in sync with the desired state. Uses a spec-hash annotation for change detection to avoid update hot-loops caused by API server field defaulting. Injects a Secret content hash into the pod template annotations to trigger rolling updates when Secret data changes (since Kubernetes does not restart pods when a referenced Secret's content is updated).

Does **not** construct the StatefulSet manifest — that is delegated to the workload builder. Only handles the CRUD lifecycle, Secret-hash injection, and field-update constraints imposed by the Kubernetes StatefulSet API.

## Boundaries

- Owns: StatefulSet Get/Create/Update lifecycle, spec-hash change detection, Secret-hash annotation injection into pod template, and enforcement of the mutable-only update rule.
- Delegates: StatefulSet manifest construction to `build_statefulset`.
- Delegates: owner reference assignment to `ctrl.SetControllerReference`.
- Delegates: spec-hash computation to `helpers.specHash`.
- Delegates: Secret content hash computation to `helpers.secretDataHash`.
- Must not: modify `Selector`, `ServiceName`, `PodManagementPolicy`, or `VolumeClaimTemplates` on an existing StatefulSet (these are immutable in the Kubernetes API).
- Must not: create any resource other than the StatefulSet.
- Must not: read the Secret from the Kubernetes API (it receives the Secret as a parameter from the caller).

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `client.Client` (from receiver) | Kubernetes API access | `Get`, `Create`, `Update` on StatefulSets | Must not delete StatefulSets; must not Get Secrets |
| `ctrl.SetControllerReference` | Owner reference setup | Call once on the desired StatefulSet | — |
| `build_statefulset.buildStatefulSet` | Manifest rendering | Call to obtain the desired StatefulSet | Must not modify the returned object's immutable fields |
| `helpers.specHash` | Change detection | Call on `desired.Spec` to produce a hash | — |
| `helpers.secretDataHash` | Secret content hash | Call on `secret.Data` to produce a hash | — |
| `helpers.setAnnotation` | Annotation update | Call to write annotations | — |

Construction constraints:
- This is a method on `DevPodReconciler`. No separate construction is needed.

## Behavior

1. Calls `buildStatefulSet(devpod)` to obtain the desired StatefulSet manifest.
2. Computes `secretDataHash(secret.Data)` and writes the result as the `apps.devpod.com/secret-hash` annotation on `desired.Spec.Template.ObjectMeta.Annotations` (the pod template, not the StatefulSet metadata). This ensures that when Secret content changes, the pod template changes, causing Kubernetes to roll the pods.
3. Computes `specHash(desired.Spec)` — this now captures the Secret hash because it is embedded in the Template annotations — and stores the result as the `devpod.com/spec-hash` annotation on the StatefulSet's own ObjectMeta.
4. Sets the DevPod as the controller owner.
5. Attempts to Get the existing StatefulSet by name.
6. If not found: Creates the desired StatefulSet and returns it.
7. If found and the spec-hash annotation matches: returns the existing StatefulSet (no-op).
8. If hashes differ: copies only the mutable fields from desired to existing:
   - `Spec.Replicas`
   - `Spec.UpdateStrategy`
   - `Spec.Template` (which now carries the updated Secret hash annotation)
9. Updates the spec-hash annotation on the existing StatefulSet and issues an Update.
10. Returns the updated StatefulSet.

## Inputs

| Field | Type | Constraints | Required |
|---|---|---|---|
| ctx | context.Context | Carries logger and cancellation | Yes |
| devpod | *devpodv1.DevPod | Must have Name, Namespace, and a fully-populated Spec | Yes |
| secret | *corev1.Secret | The DevPod's owned Secret (already fetched by caller); `.Data` is used for hash computation | Yes |

## Outputs

| Field | Type | Description |
|---|---|---|
| *appsv1.StatefulSet | pointer | The reconciled StatefulSet (created, unchanged, or updated) |
| error | error | Non-nil if Get/Create/Update/hash computation/SetControllerReference fails |

## Invariants

- Must never modify `Selector`, `ServiceName`, `PodManagementPolicy`, or `VolumeClaimTemplates` on an existing StatefulSet. The CRD marks the fields that feed these as immutable, preventing drift at the API level.
- Must compare spec-hashes before issuing an Update — if hashes match, no write is performed.
- The `devpod.com/spec-hash` annotation must always reflect the hash of the last-written spec.
- The `apps.devpod.com/secret-hash` annotation must always be present on the pod template and reflect the hash of the Secret's `.Data` at the time of reconciliation.
- The Secret hash must be injected into the pod template **before** computing the spec-hash, so that Secret content changes are captured by the spec-hash comparison.
- Must always set an owner reference pointing to the DevPod on creation.

## Edge Cases

- Condition: StatefulSet exists but annotations map is nil.
  Expected: Hash comparison treats nil annotation as "not matching" and proceeds with the update.

- Condition: Only `spec.image` changed on the DevPod (affects Template but not VolumeClaimTemplates).
  Expected: Hash changes; only `Template` is overwritten in the Update.

- Condition: The API server's admission controller modifies the StatefulSet (e.g., mutating webhook adds a sidecar).
  Expected: The operator's spec-hash is computed from what *it* wrote, not the live object. The next reconcile detects a mismatch and re-asserts the desired Template. External mutations are overwritten.

- Condition: Secret data changes but DevPod spec is unchanged.
  Expected: `secretDataHash` produces a different value → pod template annotation changes → `specHash` changes → StatefulSet is updated → Kubernetes rolls the pod.

- Condition: Operator is upgraded and existing StatefulSet lacks the `apps.devpod.com/secret-hash` pod template annotation.
  Expected: The new annotation is injected, causing the spec-hash to differ from the stored value. A one-time rolling update is triggered. This is acceptable and ensures all pods are under consistent management.

## Related

- [DevPod Reconciler](./devpod_controller.md) — caller; reads `ReadyReplicas` and `ObservedGeneration` from the returned StatefulSet.
- [build_statefulset](./build_statefulset.md) — produces the desired manifest consumed by this unit.
- [helpers](./helpers.md) — provides `specHash`, `secretDataHash`, `setAnnotation`.
