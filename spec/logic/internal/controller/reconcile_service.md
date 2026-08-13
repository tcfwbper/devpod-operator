# Service Reconciliation

## Overview

Ensures an owned NodePort `Service` exists for the DevPod, exposing every port listed in `spec.nodePorts`. Uses a spec-hash annotation to detect when the desired state has changed, avoiding spurious updates caused by API server field defaulting.

Does **not** decide which ports to expose — that is determined entirely by the DevPod CR's `spec.nodePorts` field.

## Boundaries

- Owns: Service existence, port mapping from `spec.nodePorts`, spec-hash change detection, and field-level updates (preserving API-server-assigned `ClusterIP`).
- Delegates: owner reference assignment to `ctrl.SetControllerReference`.
- Delegates: port naming to `helpers.portName`.
- Delegates: spec-hash computation to `helpers.specHash`.
- Must not: modify `spec.clusterIP` (assigned by the API server, must survive updates).
- Must not: create any resource other than the Service.
- Must not: decide the port list — it is taken verbatim from the CR.

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `client.Client` (from receiver) | Kubernetes API access | `Get`, `Create`, `Update` on Services | Must not delete Services |
| `ctrl.SetControllerReference` | Owner reference setup | Call once on the desired Service | — |
| `helpers.standardLabels` | Label generation | Call to produce labels | — |
| `helpers.selectorLabels` | Pod selector | Call to produce the Service's pod selector | — |
| `helpers.portName` | Port naming | Call for each port to get a DNS-1123 compliant name | — |
| `helpers.specHash` | Change detection | Call on `desired.Spec` to produce a hash string | — |
| `helpers.setAnnotation` | Annotation update | Call to write the spec-hash annotation on existing objects | — |

Construction constraints:
- This is a method on `DevPodReconciler`. No separate construction is needed.

## Behavior

1. Builds a `[]corev1.ServicePort` from `devpod.Spec.NodePorts`: for each mapping, `Name = portName(src)`, `Protocol = TCP`, `Port = src`, `TargetPort = src`, `NodePort = dest`.
2. Constructs the desired Service with: name = DevPod name, namespace = DevPod namespace, labels = `standardLabels`, type = `NodePort`, selector = `selectorLabels(devpod)`, ports from step 1, `ExternalTrafficPolicy = Cluster`, `SessionAffinity = None`.
3. Computes `specHash(desired.Spec)` and stores it as the `devpod.com/spec-hash` annotation on the desired Service.
4. Sets the DevPod as the controller owner.
5. Attempts to Get the existing Service by name.
6. If not found: Creates the desired Service and returns it.
7. If found and `existing.Annotations["devpod.com/spec-hash"]` matches the new hash: returns the existing Service (no-op).
8. If hashes differ: copies mutable fields from desired to existing (`Type`, `Selector`, `Ports`, `ExternalTrafficPolicy`, `SessionAffinity`), updates the spec-hash annotation, and issues an Update. Does **not** copy `ClusterIP`.
9. Returns the updated Service.

## Inputs

| Field | Type | Constraints | Required |
|---|---|---|---|
| ctx | context.Context | Carries logger and cancellation | Yes |
| devpod | *devpodv1.DevPod | Must have Name, Namespace, Spec.NodePorts populated | Yes |

## Outputs

| Field | Type | Description |
|---|---|---|
| *corev1.Service | pointer | The reconciled Service (created, unchanged, or updated) |
| error | error | Non-nil if Get/Create/Update/hash computation/SetControllerReference fails; includes NodePort conflicts rejected by the API server |

## Invariants

- Must always produce a Service of type `NodePort`.
- Must never overwrite `spec.clusterIP` during updates.
- Must compare spec-hashes before issuing an Update — if hashes match, no write is performed.
- The `devpod.com/spec-hash` annotation must always reflect the hash of the last-written spec.
- Must always set an owner reference pointing to the DevPod.
- Port names must be DNS-1123 compliant (lowercase, alphanumeric + hyphens).

## Edge Cases

- Condition: Requested NodePort (`dest`) is already allocated by another Service in the cluster.
  Expected: The API server rejects the Create or Update. This unit returns the API error. The caller reports `Degraded/ReconcileError`.

- Condition: Service exists but annotations map is nil.
  Expected: Hash comparison treats nil annotation as "not matching" and proceeds with the update.

- Condition: `spec.nodePorts` changes (e.g., a port is added or removed).
  Expected: Hash changes, Service is updated with the new port set.

- Condition: No fields changed but API server defaulted some field (e.g., `ipFamilyPolicy`).
  Expected: The operator's spec-hash is still the same (computed from what it wrote, not what the API server returned), so no update is triggered.

## Related

- [DevPod Reconciler](./devpod_controller.md) — caller; extracts `sshNodePort` from the returned Service.
- [helpers](./helpers.md) — provides `standardLabels`, `selectorLabels`, `portName`, `specHash`, `setAnnotation`.
