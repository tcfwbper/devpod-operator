# DevPod CRD Schema

## Overview

Defines the Custom Resource Definition schema for the `DevPod` kind — all type definitions, constants, validation rules, and default values that together form the API contract between the DevPod operator and its consumers (kubectl, controller, admission webhooks).

Does **not** perform reconciliation, resource creation, or any side effects. The controller is the sole consumer of these types at runtime.

## Boundaries

- Owns: the complete data model contract of the `DevPod` custom resource (spec, status, sub-structs, enum types, condition/reason constants, and the `DockerEnabled()` convenience method).
- Owns: kubebuilder marker-driven validation rules (min/max length, patterns, CEL immutability, list semantics, default values).
- Owns: scheme type registration via `init()` (registers `DevPod` and `DevPodList` with the package-level `SchemeBuilder`).
- Delegates: CRD YAML generation to `controller-gen` (code gen product, out of scope).
- Delegates: deep copy generation to `controller-gen` (`zz_generated.deepcopy.go`).
- Delegates: reconciliation logic, status updates, and resource ownership to the controller.
- Must not: contain any business logic beyond nil-safe accessors.
- Must not: import controller-runtime or any package outside `k8s.io/apimachinery`.

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `k8s.io/apimachinery/pkg/apis/meta/v1` | Standard Kubernetes metadata types | Use `metav1.ObjectMeta`, `metav1.TypeMeta`, `metav1.ListMeta`, `metav1.Condition` | Must not call any runtime API |
| `k8s.io/apimachinery/pkg/api/resource` | Quantity type for storage sizes | Use `resource.Quantity` as field type | Must not parse or validate quantities at runtime |
| `k8s.io/apimachinery/pkg/runtime` | Scheme registration | Use `runtime.Scheme` in `init()` to register types via `SchemeBuilder` | Must not create or manage Scheme instances |
| `SchemeBuilder` (package-level, from `groupversion_info.go`) | Type registration mechanism | Call `SchemeBuilder.Register()` in `init()` | Must not redefine or re-initialize `SchemeBuilder` |

Construction constraints:
- `DevPod` and `DevPodList` instances are created by the API server or by `client-go` deserialization. Direct struct literal construction is the standard pattern in Kubernetes API types.
- `resource.Quantity` fields use pointer type (`*resource.Quantity`) to allow omitempty semantics and CRD defaulting to take effect.

## Behavior

### Constants

1. Defines three condition type constants: `ConditionReady`, `ConditionProgressing`, `ConditionDegraded`.
2. Defines six reason constants: `ReasonPasswordNotSet`, `ReasonInvalidPassword`, `ReasonStorageClassNotFound`, `ReasonReconcileError`, `ReasonWaitingForStatefulSet`, `ReasonDevPodReady`.
3. Defines `PVCReclaimPolicy` as a string enum with two values: `PVCReclaimRetain` ("Retain") and `PVCReclaimDelete` ("Delete").

### Spec Structs

4. `AuthSpec` holds a single field `Username` (string, required) identifying the Linux account inside the DevPod.
5. `PersistenceSpec` holds `StorageClass` (string, required), `Size` (*resource.Quantity, optional, default "50Gi"), and `ReclaimPolicy` (PVCReclaimPolicy, optional, default "Retain").
6. `DockerPersistenceSpec` holds `Size` (*resource.Quantity, optional, default "50Gi").
7. `DockerSpec` holds `Enabled` (*bool, optional, default true), `Image` (string, optional, default "docker.io/docker:dind"), and `Persistence` (DockerPersistenceSpec, optional).
8. `NodePortMapping` holds `Src` (int32, required, range 1-65535) and `Dest` (int32, required, range 30000-32767).
9. `PackagesSpec` holds `Apt` ([]string, optional, atomic list) and `Pip` ([]string, optional, atomic list).
10. `DevPodSpec` aggregates: `Image` (string, optional, default "docker.io/tcfwbper/dev-env:1.0.0"), `InitWorkspaceImage` (string, optional, default "docker.io/tcfwbper/dev-env:1.0.0-init-workspace"), `Auth` (AuthSpec, required), `Persistence` (PersistenceSpec, required), `NodePorts` ([]NodePortMapping, required, map list keyed on src, min 1 / max 16), `Packages` (PackagesSpec, optional), `Docker` (DockerSpec, optional).
11. `DevPodStatus` holds: `Conditions` ([]metav1.Condition, map list keyed on type), `ObservedGeneration` (int64), `ReadyReplicas` (int32), `PasswordSecret` (string), `SSHNodePort` (int32).

### Resource Types

12. `DevPod` is the top-level resource type embedding TypeMeta, ObjectMeta, Spec, and Status. It is registered as a root object with a status subresource. Short name: `dp`.
13. `DevPodList` is the corresponding list type.
14. Print columns expose: Username, SSH (node port), Ready (condition status), Reason (condition reason), Age.

### Helper Method

15. `DockerEnabled()` on `*DevPodSpec` returns `true` when `Docker.Enabled` is nil (nil means enabled by default) or when `*Docker.Enabled` is true. Returns `false` only when `Docker.Enabled` is explicitly set to false.

### Scheme Registration

16. `init()` calls `SchemeBuilder.Register()` with a function that adds `DevPod` and `DevPodList` to the given `runtime.Scheme` using `SchemeGroupVersion`.

## Inputs

This unit does not accept runtime inputs. Its "inputs" are the field values written by users via the Kubernetes API, constrained by:

| Field Path | Type | Constraints | Required |
|---|---|---|---|
| spec.image | string | MinLength=1 | No (default: "docker.io/tcfwbper/dev-env:1.0.0") |
| spec.initWorkspaceImage | string | MinLength=1 | No (default: "docker.io/tcfwbper/dev-env:1.0.0-init-workspace") |
| spec.auth.username | string | MinLength=1, MaxLength=31, Pattern=`^[a-z_][a-z0-9_-]*$`, immutable, not in reserved list | Yes |
| spec.persistence.storageClass | string | MinLength=1, immutable | Yes |
| spec.persistence.size | resource.Quantity | Immutable | No (default: "50Gi") |
| spec.persistence.reclaimPolicy | PVCReclaimPolicy | Enum: Retain, Delete | No (default: "Retain") |
| spec.nodePorts | []NodePortMapping | MinItems=1, MaxItems=16, must contain src=22, dest values unique | Yes |
| spec.nodePorts[].src | int32 | Min=1, Max=65535 | Yes |
| spec.nodePorts[].dest | int32 | Min=30000, Max=32767 | Yes |
| spec.packages.apt | []string | Atomic list | No (default: []) |
| spec.packages.pip | []string | Atomic list | No (default: []) |
| spec.docker.enabled | *bool | — | No (default: true) |
| spec.docker.image | string | MinLength=1 | No (default: "docker.io/docker:dind") |
| spec.docker.persistence.size | resource.Quantity | Immutable | No (default: "50Gi") |

## Outputs

This unit does not produce runtime outputs. Its "outputs" are the status fields written by the controller:

| Field Path | Type | Description |
|---|---|---|
| status.conditions | []metav1.Condition | Map list keyed on type; types: Ready, Progressing, Degraded |
| status.observedGeneration | int64 | Generation this status was computed from |
| status.readyReplicas | int32 | Mirrors StatefulSet readyReplicas |
| status.passwordSecret | string | Name of the owned Secret holding the ssh password |
| status.sshNodePort | int32 | The node port mapped to container port 22 |

## Invariants

- Must define exactly three condition types and six reason constants as package-level string constants.
- `PVCReclaimPolicy` must be validated by kubebuilder Enum marker to only accept "Retain" or "Delete".
- `spec.auth.username` must be immutable after creation (CEL: `self == oldSelf`).
- `spec.auth.username` must not collide with any account or group already existing in the DevPod base image. The reserved list is: root, daemon, bin, sys, sync, games, man, lp, mail, news, uucp, proxy, www-data, backup, list, irc, nobody, sshd, user, users, staff, adm, tty, disk, dialout, cdrom, floppy, tape, sudo, audio, video, plugdev, ssh, docker, nogroup, src, shadow, utmp, crontab, operator, _apt.
- `spec.persistence.storageClass` must be immutable after creation (CEL: `self == oldSelf`).
- `spec.persistence.size` must be immutable after creation (CEL: `self == oldSelf`).
- `spec.docker.persistence.size` must be immutable after creation (CEL: `self == oldSelf`).
- `spec.nodePorts` must contain at least one entry with `src == 22`.
- `spec.nodePorts` dest values must be unique across all entries.
- `spec.nodePorts` is a map list keyed on `src` (each src value appears at most once).
- `DockerEnabled()` must return `true` when `Docker.Enabled` is nil.
- `init()` must register both `DevPod` and `DevPodList` with the scheme.
- Must not import packages outside `k8s.io/apimachinery`.

## Edge Cases

- Condition: `Docker.Enabled` pointer is nil (field omitted by user).
  Expected: `DockerEnabled()` returns `true` (Docker sidecar enabled by default).

- Condition: `Persistence.Size` pointer is nil (field omitted by user).
  Expected: CRD defaulting applies "50Gi" via kubebuilder default marker.

- Condition: `spec.auth.username` is set to a value in the reserved list (e.g., "root").
  Expected: Admission is rejected by CEL XValidation rule with message "auth.username collides with an account or group that already exists in the DevPod image".

- Condition: User attempts to change `spec.auth.username` on an existing resource.
  Expected: Admission is rejected by CEL XValidation rule with message "auth.username is immutable".

- Condition: `spec.nodePorts` does not include an entry with src=22.
  Expected: Admission is rejected with message "nodePorts must contain an entry with src 22 (ssh)".

- Condition: Two entries in `spec.nodePorts` have the same dest value.
  Expected: Admission is rejected with message "nodePorts dest values must be unique".

## Related

- [GroupVersion Registration](./groupversion_info.md) — provides `SchemeBuilder` and `SchemeGroupVersion` consumed by this unit's `init()`.
- Controller (`internal/controller/`) — downstream consumer of all types defined here; owns reconciliation and status updates.
