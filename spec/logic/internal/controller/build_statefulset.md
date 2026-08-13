# StatefulSet Builder

## Overview

Pure-transformation unit that renders a complete `StatefulSet` manifest (including pod template, init containers, sidecars, volume claims, and the postStart lifecycle script) from a `DevPod` spec. It encodes all workload construction decisions — container images, resource presets, security contexts, probes, volume mounts, and the shell script that bootstraps the user account.

Does **not** perform any I/O. Does **not** decide whether to Create or Update — that is owned by `reconcile_statefulset`.

## Boundaries

- Owns: the complete StatefulSet manifest shape (ObjectMeta, StatefulSetSpec, PodTemplateSpec, containers, init containers, volume claims).
- Owns: the `postStartScript` generation (account rename, password application, package installation).
- Owns: volume claim construction (workspace volume, docker volume).
- Owns: container security context, resource presets, and probe definitions.
- Owns: `shellQuoteAll` (shell-safe quoting of package names).
- Owns: `volumeSize` resolution (fall back to `defaultVolumeSize` when nil/zero).
- Delegates: label generation to `helpers.standardLabels` and `helpers.selectorLabels`.
- Delegates: port naming to `helpers.portName`.
- Must not: interact with the Kubernetes API (no Get/Create/Update/Delete).
- Must not: set owner references (that is done by the caller).
- Must not: compute or write spec-hash annotations (that is done by the caller).

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `helpers.standardLabels` | Label generation | Call to produce StatefulSet and Pod labels | — |
| `helpers.selectorLabels` | Selector labels | Call to produce `Selector.MatchLabels` and volume claim labels | — |
| `helpers.portName` | Port naming | Call for container port names | — |
| `devpodv1.DevPod` | Input data | Read `Spec` fields | Must not modify the DevPod |

Construction constraints:
- Implemented as a method on `DevPodReconciler` (access to receiver is not used for any field), or as a package-level function. Either is acceptable; the key constraint is that no `client.Client` interaction occurs.

## Behavior

### buildStatefulSet

1. Reads `spec.auth.username` and `spec.DockerEnabled()`.
2. Builds container ports from `spec.nodePorts` (one `ContainerPort` per mapping, using `portName(src)`).
3. Defines the workspace volume mount: name = `"workspace"`, mountPath = `/home/<username>`, subPath = `"workspace"`.
4. Constructs the main devpod container:
   - Name: `"devpod"`
   - Image: `spec.image`
   - ImagePullPolicy: `IfNotPresent`
   - SecurityContext: `containerSecurityContext(false)` (root, non-privileged)
   - Lifecycle postStart: exec `postStartScript(devpod)`
   - Env: `UBUNTU_PASSWORD` sourced from the DevPod's owned Secret key `"ubuntu-password"`
   - Ports: the container ports from step 2
   - LivenessProbe and ReadinessProbe: TCP check on port 22
   - Resources: preset `("1", "3072Mi", "3", "6144Mi")`
   - VolumeMounts: workspace mount
5. If Docker is enabled, constructs the docker-dind sidecar container:
   - Name: `"docker-daemon"`
   - Image: `spec.docker.image`
   - ImagePullPolicy: `IfNotPresent`
   - SecurityContext: `containerSecurityContext(true)` (privileged)
   - Command: `["dockerd", "-H", "tcp://0.0.0.0:2375"]`
   - Ports: `[{2375/TCP}]`
   - Resources: preset `("250m", "256Mi", "375m", "384Mi")`
   - VolumeMounts: `[{name: "docker-storage", mountPath: "/var/lib/docker"}]`
6. Builds VolumeClaimTemplates:
   - Always: workspace volume claim (name = `"workspace"`, size from `spec.persistence.size`).
   - If Docker enabled: docker volume claim (name = `"docker-storage"`, size from `spec.docker.persistence.size`).
7. Constructs the init container:
   - Name: `"init-workspace"`
   - Image: `spec.initWorkspaceImage`
   - SecurityContext: `containerSecurityContext(false)`
   - Resources: preset `("100m", "128Mi", "150m", "192Mi")`
   - VolumeMounts: workspace volume at `/tmp/workspace` with subPath `"workspace"`
8. Assembles the StatefulSet:
   - Replicas: 1
   - ServiceName: DevPod name
   - PodManagementPolicy: `Parallel`
   - UpdateStrategy: `RollingUpdate`
   - Selector: `selectorLabels(devpod)`
   - PodTemplate: ServiceAccountName = DevPod name, AutomountServiceAccountToken = false, EnableServiceLinks = true, TerminationGracePeriodSeconds = 10, FSGroup = 1001, FSGroupChangePolicy = OnRootMismatch, InitContainers, Containers.
   - VolumeClaimTemplates: from step 6.

### postStartScript

9. Generates a shell script that:
   - Renames the `"user"` account to `spec.auth.username` (usermod -l, usermod -d, groupmod -n, usermod -g).
   - Sets ownership of the home directory (chown, chgrp).
   - Applies the password via `chpasswd` (reads `$UBUNTU_PASSWORD` env var).
   - If `spec.packages.apt` is non-empty: runs `apt-get update && apt-get install -y <packages> || true && apt-get clean`.
   - If `spec.packages.pip` is non-empty: runs `pip install --no-cache-dir <packages> || true`.
10. Package install commands are best-effort (`|| true`) — a bad package name must not prevent the pod from starting.
11. `DEBIAN_FRONTEND=noninteractive` is exported before apt commands and unset after.

### volumeClaim

12. Renders a `PersistentVolumeClaim` with: name, labels = `selectorLabels(devpod)`, accessMode = `ReadWriteOnce`, storageClassName from `spec.persistence.storageClass`, storage request from `volumeSize(size)`.

### volumeSize

13. If the provided `*resource.Quantity` is nil or zero, returns `resource.MustParse("50Gi")`. Otherwise returns the dereferenced value.

### containerSecurityContext

14. Returns a `SecurityContext` with: RunAsUser=0, RunAsGroup=0, RunAsNonRoot=false, AllowPrivilegeEscalation=true, ReadOnlyRootFilesystem=false, Privileged=parameter, SeccompProfile=RuntimeDefault.

### resourcePreset

15. Returns `ResourceRequirements` with the given request CPU/memory and limit CPU/memory, plus hardcoded ephemeral-storage: request=50Mi, limit=2Gi.

### sshProbe

16. Returns a `Probe` with: TCPSocket on port 22, InitialDelaySeconds=10, TimeoutSeconds=20, PeriodSeconds=30, FailureThreshold=3, SuccessThreshold=1.

### shellQuoteAll

17. Returns a space-prefixed string of shell-quoted package names (using Go `%q` formatting). E.g., input `["curl", "git"]` produces `" \"curl\" \"git\""`.

## Inputs

### buildStatefulSet

| Field | Type | Constraints | Required |
|---|---|---|---|
| devpod | *devpodv1.DevPod | Fully populated Spec (after CRD defaults applied) | Yes |

### postStartScript

| Field | Type | Constraints | Required |
|---|---|---|---|
| devpod | *devpodv1.DevPod | Spec.Auth.Username and Spec.Packages must be set | Yes |

### volumeClaim

| Field | Type | Constraints | Required |
|---|---|---|---|
| devpod | *devpodv1.DevPod | Spec.Persistence.StorageClass must be set | Yes |
| name | string | Volume name ("workspace" or "docker-storage") | Yes |
| size | *resource.Quantity | Nullable; falls back to defaultVolumeSize | No |

### volumeSize

| Field | Type | Constraints | Required |
|---|---|---|---|
| size | *resource.Quantity | Nullable | No |

## Outputs

### buildStatefulSet

| Field | Type | Description |
|---|---|---|
| *appsv1.StatefulSet | pointer | Complete desired StatefulSet manifest (without owner ref or spec-hash annotation) |

### postStartScript

| Field | Type | Description |
|---|---|---|
| string | — | Multi-line shell script, newline-terminated |

### volumeClaim

| Field | Type | Description |
|---|---|---|
| corev1.PersistentVolumeClaim | value | PVC spec for a VolumeClaimTemplate entry |

### volumeSize

| Field | Type | Description |
|---|---|---|
| resource.Quantity | value | Resolved storage capacity |

## Invariants

- `buildStatefulSet` must be a pure function: same DevPod input always produces the same StatefulSet output.
- Replicas must always be 1. A DevPod is a single-user environment; scaling is not supported.
- The devpod container must always run as root (uid 0) to allow postStart account management.
- The docker-daemon sidecar must be privileged (required by Docker-in-Docker).
- `postStartScript` must always apply the password, even if packages are empty.
- Package installation must always be wrapped with `|| true` to prevent pod start failure.
- Volume claims must use `selectorLabels` (not `standardLabels`) so the finalizer can find them.
- `containerSecurityContext` must always set `SeccompProfile = RuntimeDefault`.
- `defaultVolumeSize` is `"50Gi"` (hardcoded constant).
- FSGroup must be 1001 (matches the uid/gid in the DevPod base image).

## Edge Cases

- Condition: `spec.docker.enabled` is explicitly false.
  Expected: No docker-daemon container is added. No docker-storage volume claim is created. Only the workspace volume claim is present.

- Condition: `spec.packages.apt` and `spec.packages.pip` are both empty.
  Expected: postStartScript contains only account rename and password lines. No apt/pip commands.

- Condition: `spec.persistence.size` is nil (CRD default not applied for some reason).
  Expected: `volumeSize` returns `resource.MustParse("50Gi")`.

- Condition: Package name contains shell metacharacters (e.g., `"pkg; rm -rf /"`).
  Expected: `shellQuoteAll` wraps each name in Go-style double quotes (`%q`), neutralizing metacharacters.

- Condition: `spec.nodePorts` has multiple entries including port 22.
  Expected: All ports appear as container ports. Port 22 gets the name `"ssh"`; others get `"port-<n>"`.

## Related

- [reconcile_statefulset](./reconcile_statefulset.md) — caller; handles CRUD using the manifest this unit produces.
- [helpers](./helpers.md) — provides `standardLabels`, `selectorLabels`, `portName`.
- [DevPod CRD Schema](../../api/v1/devpod_types.md) — defines the input types consumed by this unit.
