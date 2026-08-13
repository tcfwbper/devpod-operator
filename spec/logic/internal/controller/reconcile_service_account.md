# ServiceAccount Reconciliation

## Overview

Ensures an owned `ServiceAccount` exists for the DevPod with API server token auto-mounting disabled. The DevPod workload has no business communicating with the Kubernetes API, so the ServiceAccount is locked down at creation.

## Boundaries

- Owns: ServiceAccount existence and the `AutomountServiceAccountToken=false` setting.
- Delegates: owner reference assignment to `ctrl.SetControllerReference`.
- Must not: manage RBAC (Roles, RoleBindings, ClusterRoles) for this ServiceAccount.
- Must not: create any resource other than the ServiceAccount.

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `client.Client` (from receiver) | Kubernetes API access | `Get`, `Create`, `Update` on ServiceAccounts | Must not delete ServiceAccounts |
| `ctrl.SetControllerReference` | Owner reference setup | Call once on the desired ServiceAccount before creation | — |
| `helpers.standardLabels` | Label generation | Call to produce labels for the ServiceAccount | — |

Construction constraints:
- This is a method on `DevPodReconciler`. No separate construction is needed.

## Behavior

1. Constructs a desired `ServiceAccount` with: name = DevPod name, namespace = DevPod namespace, labels = `standardLabels(devpod)`, `AutomountServiceAccountToken = false`.
2. Sets the DevPod as the controller owner.
3. Attempts to Get the existing ServiceAccount by name.
4. If not found: Creates the desired ServiceAccount.
5. If found and `AutomountServiceAccountToken` is nil or true: Updates it to `false`.
6. If found and already `false`: no-op.

## Inputs

| Field | Type | Constraints | Required |
|---|---|---|---|
| ctx | context.Context | Carries logger and cancellation | Yes |
| devpod | *devpodv1.DevPod | Must have Name, Namespace set | Yes |

## Outputs

| Field | Type | Description |
|---|---|---|
| error | error | Non-nil if Get/Create/Update/SetControllerReference fails |

## Invariants

- Must always set `AutomountServiceAccountToken` to `false`. If an external actor sets it to `true`, the next reconcile corrects it.
- Must always set an owner reference pointing to the DevPod on creation.

## Edge Cases

- Condition: ServiceAccount exists but `AutomountServiceAccountToken` is nil (field was removed by external edit).
  Expected: Treats nil as "not false" and Updates to set it to `false`.

- Condition: ServiceAccount exists with correct configuration.
  Expected: No Update issued. Returns nil immediately.

## Related

- [DevPod Reconciler](./devpod_controller.md) — caller.
- [helpers](./helpers.md) — provides `standardLabels`.
