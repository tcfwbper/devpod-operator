# Controller Helpers

## Overview

A collection of pure utility functions shared across the controller package. Provides label generation, annotation helpers, spec-hash computation, port naming, and SSH node-port extraction. None of these functions perform I/O or make reconciliation decisions.

## Boundaries

- Owns: `standardLabels`, `selectorLabels`, `setAnnotation`, `specHash`, `secretDataHash`, `portName`, `sshNodePort`.
- Must not: interact with the Kubernetes API.
- Must not: make any reconciliation or status decisions.
- Must not: import controller-runtime packages (only uses `k8s.io/apimachinery` types and `sigs.k8s.io/controller-runtime/pkg/client` for the `client.Object` interface in `setAnnotation`).

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `devpodv1.DevPod` | Input data | Read `.Name` | Must not modify the DevPod |
| `client.Object` interface | Annotation target | Call `GetAnnotations()` / `SetAnnotations()` | Must not call any other methods |
| `encoding/json` | Serialization for hashing | `json.Marshal` | — |
| `crypto/sha256` | Hash computation | `sha256.Sum256` | — |
| `sort` | Deterministic key ordering for `secretDataHash` | `sort.Strings` | — |

Construction constraints:
- All functions are package-level (not methods on any struct). No construction needed.

## Behavior

### specHash

1. JSON-marshals the provided value (typically a resource `.Spec`).
2. Computes SHA-256 of the marshaled bytes.
3. Returns the first 32 hex characters of the hash.
4. Returns an error if JSON marshaling fails.

### secretDataHash

5. Accepts a `map[string][]byte` (the `.Data` field of a Kubernetes Secret).
6. Sorts the map keys lexicographically.
7. Concatenates each key-value pair as raw bytes in sorted order: for each key, appends `[]byte(key)` followed by the value bytes.
8. Computes SHA-256 of the concatenated bytes.
9. Returns the first 32 hex characters of the hash.
10. If the input map is nil or empty, returns a deterministic hash of the empty byte slice.

### standardLabels

11. Returns a label map containing:
    - `app.kubernetes.io/name` = `"devpod"`
    - `app.kubernetes.io/instance` = `devpod.Name`
    - `app.kubernetes.io/managed-by` = `"devpod-operator"`

### selectorLabels

12. Returns a label map containing:
    - `app.kubernetes.io/name` = `"devpod"`
    - `app.kubernetes.io/instance` = `devpod.Name`
13. This is a subset of `standardLabels` — it excludes `managed-by` because selectors must be stable and `managed-by` is informational.

### setAnnotation

14. Gets the object's existing annotations map.
15. If nil, initializes a new map.
16. Sets the given key-value pair.
17. Calls `SetAnnotations` on the object.

### portName

18. If the port is 22, returns `"ssh"`.
19. Otherwise, returns `"port-<n>"` where `<n>` is the port number.

### sshNodePort

20. Iterates the Service's `spec.ports`.
21. Returns the `NodePort` of the first entry whose `Port` equals 22.
22. Returns 0 if no such entry exists.

## Inputs

### specHash

| Field | Type | Constraints | Required |
|---|---|---|---|
| spec | any | Must be JSON-marshalable | Yes |

### secretDataHash

| Field | Type | Constraints | Required |
|---|---|---|---|
| data | map[string][]byte | The Secret's `.Data` field; may be nil | Yes |

### standardLabels / selectorLabels

| Field | Type | Constraints | Required |
|---|---|---|---|
| devpod | *devpodv1.DevPod | Must have `.Name` set | Yes |

### setAnnotation

| Field | Type | Constraints | Required |
|---|---|---|---|
| object | client.Object | Any Kubernetes object | Yes |
| key | string | Annotation key | Yes |
| value | string | Annotation value | Yes |

### portName

| Field | Type | Constraints | Required |
|---|---|---|---|
| port | int32 | A container port number | Yes |

### sshNodePort

| Field | Type | Constraints | Required |
|---|---|---|---|
| service | *corev1.Service | Must have `Spec.Ports` populated | Yes |

## Outputs

### specHash

| Field | Type | Description |
|---|---|---|
| string | — | First 32 hex chars of the SHA-256 hash |
| error | — | Non-nil if JSON marshaling fails |

### secretDataHash

| Field | Type | Description |
|---|---|---|
| string | — | First 32 hex chars of the SHA-256 hash of the Secret data |

### standardLabels / selectorLabels

| Field | Type | Description |
|---|---|---|
| map[string]string | — | Label map |

### setAnnotation

No return value (mutates the object in place).

### portName

| Field | Type | Description |
|---|---|---|
| string | — | DNS-1123 compliant port name |

### sshNodePort

| Field | Type | Description |
|---|---|---|
| int32 | — | The node port for SSH, or 0 if not found |

## Invariants

- `selectorLabels` must always be a strict subset of `standardLabels`.
- `selectorLabels` must never include `app.kubernetes.io/managed-by` (that label is informational and must not participate in selectors).
- `specHash` must be deterministic: same input always produces the same output.
- `specHash` truncates to 32 hex characters (128 bits of the 256-bit hash).
- `secretDataHash` must be deterministic: same data map (regardless of Go map iteration order) always produces the same output.
- `secretDataHash` truncates to 32 hex characters (same length as `specHash`).
- `secretDataHash` does not return an error — it always succeeds (no marshaling involved).
- `portName` must always produce a DNS-1123 label (lowercase alphanumeric and hyphens, starting with alpha).
- All functions in this unit are pure (no side effects beyond `setAnnotation`'s in-place mutation of the passed object).
- `setAnnotation` must be nil-safe on the annotations map.

## Edge Cases

- Condition: `specHash` is given a struct with unexported fields.
  Expected: `json.Marshal` skips unexported fields (standard Go behavior). The hash is still deterministic for the exported portion.

- Condition: `secretDataHash` is called with a nil map.
  Expected: Returns the SHA-256 hash of the empty byte slice (deterministic, not a panic).

- Condition: `secretDataHash` is called with a map containing multiple keys.
  Expected: Keys are sorted lexicographically before hashing. The result is independent of Go map iteration order.

- Condition: `setAnnotation` is called on an object whose annotation map is nil.
  Expected: Initializes the map before inserting. Does not panic.

- Condition: `sshNodePort` is called on a Service with no port 22 entry.
  Expected: Returns 0.

- Condition: `portName(0)` is called (invalid port).
  Expected: Returns `"port-0"`. Validation of valid port ranges is the CRD's responsibility, not this function's.

- Condition: `standardLabels` is called and caller modifies the returned map.
  Expected: Each call returns a new map. Mutation does not affect future calls.

## Related

- [DevPod Reconciler](./devpod_controller.md) — uses `sshNodePort`, `selectorLabels`.
- [reconcile_secret](./reconcile_secret.md) — uses `standardLabels`.
- [reconcile_service_account](./reconcile_service_account.md) — uses `standardLabels`.
- [reconcile_service](./reconcile_service.md) — uses `standardLabels`, `selectorLabels`, `portName`, `specHash`, `setAnnotation`.
- [reconcile_statefulset](./reconcile_statefulset.md) — uses `specHash`, `secretDataHash`, `setAnnotation`.
- [build_statefulset](./build_statefulset.md) — uses `standardLabels`, `selectorLabels`, `portName`.
