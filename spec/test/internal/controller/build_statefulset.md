# Test Specification: `build_statefulset_test.go`

## Source File Under Test

`internal/controller/build_statefulset.go`

## Test File

`internal/controller/build_statefulset_test.go`

---

## `buildStatefulSet`

### Happy Path — buildStatefulSet

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestBuildStatefulSet_BasicStructure` | `unit` | Produces a StatefulSet with correct top-level fields. | | A DevPod with `Name = "dev1"`, `Namespace = "ns"`, `Spec.Image = "img:1"`, `Spec.Auth.Username = "alice"`, Docker enabled, `Spec.NodePorts = [{Src:22, Dest:30022}]` | StatefulSet has `Name = "dev1"`, `Namespace = "ns"`, `Replicas = 1`, `ServiceName = "dev1"`, `PodManagementPolicy = Parallel`, `UpdateStrategy.Type = RollingUpdate` |
| `TestBuildStatefulSet_SelectorLabels` | `unit` | Selector uses selectorLabels. | | Same DevPod as above | `Spec.Selector.MatchLabels` equals `selectorLabels(devpod)` |
| `TestBuildStatefulSet_PodTemplateLabels` | `unit` | Pod template uses standardLabels. | | Same DevPod as above | `Spec.Template.Labels` equals `standardLabels(devpod)` |
| `TestBuildStatefulSet_PodSecurityContext` | `unit` | Pod template has correct security context. | | Same DevPod as above | `Spec.Template.Spec.SecurityContext.FSGroup = 1001`, `FSGroupChangePolicy = OnRootMismatch` |
| `TestBuildStatefulSet_PodServiceAccount` | `unit` | Pod template references the DevPod name as service account. | | Same DevPod as above | `Spec.Template.Spec.ServiceAccountName = "dev1"`, `AutomountServiceAccountToken = false`, `EnableServiceLinks = true` |
| `TestBuildStatefulSet_TerminationGracePeriod` | `unit` | Termination grace period is 10 seconds. | | Same DevPod as above | `Spec.Template.Spec.TerminationGracePeriodSeconds = 10` |

### Happy Path — devpod container

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestBuildStatefulSet_DevpodContainer_Basic` | `unit` | Main container has correct name, image, and pull policy. | | DevPod with `Spec.Image = "myimg:v1"` | Container named `"devpod"` with `Image = "myimg:v1"`, `ImagePullPolicy = IfNotPresent` |
| `TestBuildStatefulSet_DevpodContainer_SecurityContext` | `unit` | Main container runs as root, non-privileged. | | Standard DevPod | Container SecurityContext: `RunAsUser=0`, `RunAsGroup=0`, `RunAsNonRoot=false`, `Privileged=false`, `SeccompProfile.Type=RuntimeDefault` |
| `TestBuildStatefulSet_DevpodContainer_Ports` | `unit` | Container ports match spec.nodePorts with correct names. | | DevPod with `NodePorts = [{Src:22, Dest:30022}, {Src:8080, Dest:30080}]` | Container ports: `{Name:"ssh", ContainerPort:22, Protocol:TCP}`, `{Name:"port-8080", ContainerPort:8080, Protocol:TCP}` |
| `TestBuildStatefulSet_DevpodContainer_Probes` | `unit` | Liveness and readiness probes check TCP 22. | | Standard DevPod | Both probes: TCPSocket port 22, InitialDelaySeconds=10, TimeoutSeconds=20, PeriodSeconds=30, FailureThreshold=3, SuccessThreshold=1 |
| `TestBuildStatefulSet_DevpodContainer_Resources` | `unit` | Resource requests and limits match the preset. | | Standard DevPod | Requests: CPU=1, Memory=3072Mi, Ephemeral=50Mi; Limits: CPU=3, Memory=6144Mi, Ephemeral=2Gi |
| `TestBuildStatefulSet_DevpodContainer_VolumeMount` | `unit` | Workspace volume is mounted at /home/username. | | DevPod with `Spec.Auth.Username = "bob"` | VolumeMount: `Name="workspace"`, `MountPath="/home/bob"`, `SubPath="workspace"` |
| `TestBuildStatefulSet_DevpodContainer_EnvPassword` | `unit` | UBUNTU_PASSWORD env var sourced from Secret. | | DevPod with `Name = "dev1"` | Env: `Name="UBUNTU_PASSWORD"`, ValueFrom SecretKeyRef with `Name="dev1"`, `Key="ubuntu-password"` |
| `TestBuildStatefulSet_DevpodContainer_Lifecycle` | `unit` | PostStart lifecycle hook calls postStartScript. | | Standard DevPod | Lifecycle.PostStart.Exec.Command contains `["sh", "-c", <postStartScript output>]` |

### Happy Path — docker-dind sidecar

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestBuildStatefulSet_DockerSidecar_Present` | `unit` | Docker sidecar is included when Docker is enabled. | | DevPod with Docker enabled, `Spec.Docker.Image = "docker:dind"` | Second container named `"docker-daemon"` with `Image = "docker:dind"`, `ImagePullPolicy = IfNotPresent` |
| `TestBuildStatefulSet_DockerSidecar_Privileged` | `unit` | Docker sidecar runs privileged. | | DevPod with Docker enabled | Container SecurityContext: `Privileged=true`, `SeccompProfile.Type=RuntimeDefault` |
| `TestBuildStatefulSet_DockerSidecar_Command` | `unit` | Docker sidecar has correct command. | | DevPod with Docker enabled | Command: `["dockerd", "-H", "tcp://0.0.0.0:2375"]` |
| `TestBuildStatefulSet_DockerSidecar_Port` | `unit` | Docker sidecar exposes port 2375. | | DevPod with Docker enabled | Ports: `[{ContainerPort: 2375, Protocol: TCP}]` |
| `TestBuildStatefulSet_DockerSidecar_Resources` | `unit` | Docker sidecar resource preset matches spec. | | DevPod with Docker enabled | Requests: CPU=250m, Memory=256Mi, Ephemeral=50Mi; Limits: CPU=375m, Memory=384Mi, Ephemeral=2Gi |
| `TestBuildStatefulSet_DockerSidecar_VolumeMount` | `unit` | Docker sidecar mounts docker-storage volume. | | DevPod with Docker enabled | VolumeMount: `Name="docker-storage"`, `MountPath="/var/lib/docker"` |
| `TestBuildStatefulSet_DockerDisabled_NoSidecar` | `unit` | No docker-daemon container when Docker is disabled. | | DevPod with `Docker.Enabled = false` | Only one container (`"devpod"`) present; no `"docker-daemon"` container |

### Happy Path — init container

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestBuildStatefulSet_InitContainer` | `unit` | Init container has correct name, image, security, and mount. | | DevPod with `Spec.InitWorkspaceImage = "init:1"` | InitContainer: `Name="init-workspace"`, `Image="init:1"`, SecurityContext non-privileged root, VolumeMount: `Name="workspace"`, `MountPath="/tmp/workspace"`, `SubPath="workspace"` |
| `TestBuildStatefulSet_InitContainer_Resources` | `unit` | Init container resource preset matches spec. | | Standard DevPod | Requests: CPU=100m, Memory=128Mi, Ephemeral=50Mi; Limits: CPU=150m, Memory=192Mi, Ephemeral=2Gi |

### Happy Path — volume claims

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestBuildStatefulSet_VolumeClaimWorkspace` | `unit` | Workspace volume claim is always present with correct settings. | | DevPod with `Spec.Persistence.StorageClass = "standard"`, `Spec.Persistence.Size = "100Gi"` | VolumeClaimTemplate: `Name="workspace"`, Labels = selectorLabels, AccessMode = ReadWriteOnce, StorageClassName = "standard", Storage = "100Gi" |
| `TestBuildStatefulSet_VolumeClaimDocker` | `unit` | Docker volume claim is present when Docker enabled. | | DevPod with Docker enabled, `Spec.Docker.Persistence.Size = "200Gi"` | Second VolumeClaimTemplate: `Name="docker-storage"`, Storage = "200Gi" |
| `TestBuildStatefulSet_VolumeClaimDocker_Absent` | `unit` | No docker-storage volume claim when Docker disabled. | | DevPod with `Docker.Enabled = false` | Only 1 VolumeClaimTemplate (`"workspace"`) |

---

## `postStartScript`

### Happy Path — postStartScript

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestPostStartScript_AccountRename` | `unit` | Script contains usermod/groupmod commands for the target username. | | DevPod with `Spec.Auth.Username = "alice"` | Script contains `usermod -l alice`, `usermod -d /home/alice`, `groupmod -n alice`, `usermod -g alice`, `chown`, `chgrp` commands |
| `TestPostStartScript_PasswordApplication` | `unit` | Script always applies password via chpasswd. | | DevPod with `Spec.Auth.Username = "alice"`, no packages | Script contains `chpasswd` referencing `$UBUNTU_PASSWORD` |
| `TestPostStartScript_AptPackages` | `unit` | Script installs apt packages when specified. | | DevPod with `Spec.Packages.Apt = ["curl", "git"]` | Script contains `DEBIAN_FRONTEND=noninteractive`, `apt-get update`, `apt-get install -y`, quoted package names, `|| true`, `apt-get clean`, and unsets `DEBIAN_FRONTEND` |
| `TestPostStartScript_PipPackages` | `unit` | Script installs pip packages when specified. | | DevPod with `Spec.Packages.Pip = ["numpy", "pandas"]` | Script contains `pip install --no-cache-dir`, quoted package names, `|| true` |
| `TestPostStartScript_NoPackages` | `unit` | Script has no apt/pip commands when packages empty. | | DevPod with `Spec.Packages.Apt = []`, `Spec.Packages.Pip = []` | Script does NOT contain `apt-get` or `pip install` |
| `TestPostStartScript_BothPackageTypes` | `unit` | Script includes both apt and pip sections. | | DevPod with `Spec.Packages.Apt = ["curl"]`, `Spec.Packages.Pip = ["flask"]` | Script contains both `apt-get install` and `pip install` sections |

---

## `volumeSize`

### Happy Path — volumeSize

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestVolumeSize_NonNilNonZero` | `unit` | Returns the provided quantity when non-nil and non-zero. | | `size` = pointer to `resource.MustParse("100Gi")` | Returns `resource.MustParse("100Gi")` |

### Null / Empty Input

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestVolumeSize_Nil` | `unit` | Returns default 50Gi when nil. | | `size = nil` | Returns `resource.MustParse("50Gi")` |
| `TestVolumeSize_Zero` | `unit` | Returns default 50Gi when zero value. | | `size` = pointer to zero Quantity | Returns `resource.MustParse("50Gi")` |

---

## `containerSecurityContext`

### Happy Path — containerSecurityContext

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestContainerSecurityContext_NonPrivileged` | `unit` | Returns correct context for non-privileged container. | | `privileged = false` | RunAsUser=0, RunAsGroup=0, RunAsNonRoot=false, AllowPrivilegeEscalation=true, ReadOnlyRootFilesystem=false, Privileged=false, SeccompProfile.Type=RuntimeDefault |
| `TestContainerSecurityContext_Privileged` | `unit` | Returns correct context for privileged container. | | `privileged = true` | Same as above except Privileged=true |

---

## `resourcePreset`

### Happy Path — resourcePreset

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestResourcePreset_Values` | `unit` | Returns ResourceRequirements with given values plus ephemeral defaults. | | `reqCPU="1"`, `reqMem="3072Mi"`, `limCPU="3"`, `limMem="6144Mi"` | Requests: CPU=1, Memory=3072Mi, EphemeralStorage=50Mi; Limits: CPU=3, Memory=6144Mi, EphemeralStorage=2Gi |

---

## `sshProbe`

### Happy Path — sshProbe

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestSSHProbe_Fields` | `unit` | Returns probe with correct TCP and timing settings. | | | Probe: TCPSocket.Port=22, InitialDelaySeconds=10, TimeoutSeconds=20, PeriodSeconds=30, FailureThreshold=3, SuccessThreshold=1 |

---

## `shellQuoteAll`

### Happy Path — shellQuoteAll

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestShellQuoteAll_MultiplePackages` | `unit` | Returns space-prefixed, quoted package names. | | `["curl", "git"]` | Returns `" \"curl\" \"git\""` |
| `TestShellQuoteAll_SinglePackage` | `unit` | Works for a single package. | | `["curl"]` | Returns `" \"curl\""` |
| `TestShellQuoteAll_MetaCharacters` | `unit` | Neutralizes shell metacharacters via Go %q formatting. | | `["pkg; rm -rf /"]` | Returns a string with the package name escaped (e.g., `" \"pkg; rm -rf /\""`) |

### Null / Empty Input

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestShellQuoteAll_EmptySlice` | `unit` | Returns empty string for empty input. | | `[]string{}` | Returns `""` |
