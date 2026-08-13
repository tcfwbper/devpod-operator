# Secret Reconciliation

## Overview

Ensures an owned `Secret` exists for the DevPod, containing the `ubuntu-password` key. The Secret is created with an empty password on purpose — it acts as both a placeholder for the human to fill in and the reconciliation gate that blocks workload deployment until a valid password is provided.

Also owns password validation logic that determines whether a supplied password can be safely applied by the container's `chpasswd` utility.

## Boundaries

- Owns: creating the Secret if absent, ensuring the `ubuntu-password` key exists.
- Owns: password validation rules (forbidden characters, minimum length).
- Delegates: owner reference assignment to `ctrl.SetControllerReference`.
- Must not: overwrite an existing password value (the Secret holds user-supplied data).
- Must not: create any resource other than the Secret.
- Must not: update status conditions (caller handles that based on the returned Secret/error).

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `client.Client` (from receiver) | Kubernetes API access | `Get`, `Create`, `Update` on Secrets | Must not delete Secrets |
| `ctrl.SetControllerReference` | Owner reference setup | Call once on the desired Secret before creation | — |
| `helpers.standardLabels` | Label generation | Call to produce labels for the Secret | — |

Construction constraints:
- This is a method on `DevPodReconciler`. No separate construction is needed.

## Behavior

1. Constructs a desired `Secret` with: name = DevPod name, namespace = DevPod namespace, type = Opaque, data = `{ubuntu-password: ""}`, labels = `standardLabels(devpod)`.
2. Sets the DevPod as the controller owner of the desired Secret.
3. Attempts to Get the existing Secret by name.
4. If not found: Creates the desired Secret and returns it.
5. If found: checks whether the `ubuntu-password` key is present in `.Data`.
6. If the key is missing (e.g., someone removed it): adds the key with an empty value and Updates the Secret.
7. Returns the existing Secret as-is (never overwrites the password value).

### Password Validation (`validatePassword`)

8. Rejects passwords containing `":"` — chpasswd parses `user:password` format.
9. Rejects passwords containing `"\n"` or `"\r"` — chpasswd reads line by line.
10. Rejects passwords shorter than 8 characters (`minPasswordLength`).
11. Returns nil if the password passes all checks.

## Inputs

### reconcileSecret

| Field | Type | Constraints | Required |
|---|---|---|---|
| ctx | context.Context | Carries logger and cancellation | Yes |
| devpod | *devpodv1.DevPod | Must have Name, Namespace set | Yes |

### validatePassword

| Field | Type | Constraints | Required |
|---|---|---|---|
| password | string | The raw password value from the Secret | Yes |

## Outputs

### reconcileSecret

| Field | Type | Description |
|---|---|---|
| *corev1.Secret | pointer | The reconciled Secret (either newly created or fetched) |
| error | error | Non-nil if Get/Create/Update/SetControllerReference fails |

### validatePassword

| Field | Type | Description |
|---|---|---|
| error | error | Non-nil with a human-readable message if the password is invalid; nil if valid |

## Invariants

- Must never write a non-empty value to the `ubuntu-password` key. The only values this unit writes are the empty byte slice `[]byte{}`.
- Must always set an owner reference pointing to the DevPod on creation, ensuring the Secret is garbage-collected with the DevPod.
- Must always include the `ubuntu-password` key in the Secret's data map, even if the value is empty.
- `validatePassword` is a pure function with no side effects.
- `minPasswordLength` is 8 (hardcoded constant).

## Edge Cases

- Condition: Secret exists but `.Data` is nil (manually wiped).
  Expected: Initializes `.Data` to `map[string][]byte{}`, sets the key to empty, and Updates.

- Condition: Secret exists with the key but password is empty string.
  Expected: Returns the Secret unchanged. Caller (main reconciler) interprets empty as "not yet set".

- Condition: Secret exists with a populated password.
  Expected: Returns the Secret unchanged. The password is never overwritten.

- Condition: Password is exactly 8 characters, no forbidden chars.
  Expected: `validatePassword` returns nil (valid).

- Condition: Password is 7 characters.
  Expected: `validatePassword` returns error "password must be at least 8 characters long".

- Condition: Password contains a colon anywhere.
  Expected: `validatePassword` returns error `password must not contain ":"`.

- Condition: Password contains a newline or carriage return.
  Expected: `validatePassword` returns error "password must not contain a line break".

## Related

- [DevPod Reconciler](./devpod_controller.md) — caller; uses the returned Secret to gate further reconciliation.
- [helpers](./helpers.md) — provides `standardLabels`.
