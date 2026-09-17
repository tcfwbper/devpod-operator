# DevPod Operator

A Kubernetes operator that provisions on-demand development environments. Each `DevPod` custom resource creates a fully isolated workspace backed by a StatefulSet with SSH access, persistent storage, and an optional Docker-in-Docker sidecar.

## Architecture

The operator watches `DevPod` resources (`apps.devpod.com/v1`) and reconciles the following sub-resources for each CR:

| Sub-resource   | Kind              | Purpose                                       |
|----------------|-------------------|-----------------------------------------------|
| Secret         | `core/v1`         | SSH password for the dev user                  |
| ServiceAccount | `core/v1`         | Dedicated SA with automount disabled           |
| Service        | `core/v1`         | NodePort service for SSH and custom ports      |
| StatefulSet    | `apps/v1`         | Dev container with persistent volume claims    |

A finalizer (`apps.devpod.com/finalizer`) handles PVC cleanup on deletion, respecting the configured `reclaimPolicy`.

## Prerequisites

- Kubernetes cluster v1.28+
- kubectl v1.28+

## Installation

### From a GitHub Release (recommended)

Apply the latest release manifest directly:

```sh
kubectl apply -f https://github.com/tcfwbper/devpod-operator/releases/latest/download/install.yaml
```

Or pin a specific version:

```sh
kubectl apply -f https://github.com/tcfwbper/devpod-operator/releases/download/v1.0.0/install.yaml
```

### From Source

```sh
export IMG=ghcr.io/tcfwbper/devpod-operator:latest

make docker-build docker-push IMG=$IMG
make install
make deploy IMG=$IMG
```

## Configuration

### DevPod Custom Resource

| Field                    | Type     | Required | Default                                       | Description                                                     |
|--------------------------|----------|----------|-----------------------------------------------|-----------------------------------------------------------------|
| `spec.image`             | string   | No       | `docker.io/tcfwbper/dev-env:1.0.0`            | Main dev container image                                        |
| `spec.initWorkspaceImage`| string   | No       | `docker.io/tcfwbper/dev-env:1.0.0-init-workspace` | Init container image for workspace setup                    |
| `spec.auth.username`     | string   | Yes      | —                                             | Linux account name (immutable, 1-31 chars, no reserved names)   |
| `spec.persistence.storageClass` | string | Yes | —                                             | StorageClass name (immutable)                                   |
| `spec.persistence.size`  | quantity | No       | `50Gi`                                        | Workspace volume size (immutable)                               |
| `spec.persistence.reclaimPolicy` | enum | No   | `Retain`                                      | `Retain` or `Delete` — PVC behavior on DevPod deletion          |
| `spec.nodePorts`         | list     | Yes      | —                                             | Port mappings (`src`/`dest`); must include `src: 22`; max 16    |
| `spec.packages.apt`      | list     | No       | `[]`                                          | APT packages to install at startup                              |
| `spec.packages.pip`      | list     | No       | `[]`                                          | pip packages to install at startup                              |
| `spec.docker.enabled`    | bool     | No       | `true`                                        | Enable Docker-in-Docker sidecar                                 |
| `spec.docker.image`      | string   | No       | `docker.io/docker:dind`                       | DinD sidecar image                                              |
| `spec.docker.persistence.size` | quantity | No | `50Gi`                                        | Docker data volume size (immutable)                             |

### Example

```yaml
apiVersion: apps.devpod.com/v1
kind: DevPod
metadata:
  name: my-devpod
  namespace: devpod-test
spec:
  image: docker.io/tcfwbper/dev-env:1.0.0
  initWorkspaceImage: docker.io/tcfwbper/dev-env:1.0.0-init-workspace
  auth:
    username: devuser
  persistence:
    storageClass: local-path
    size: 50Gi
    reclaimPolicy: Retain
  nodePorts:
    - src: 22
      dest: 30022
  packages:
    apt:
      - htop
      - python3-venv
    pip:
      - requests
  docker:
    enabled: true
    image: docker.io/docker:dind
    persistence:
      size: 50Gi
```

### Password Setup

After creating a DevPod, the operator creates a Secret named after the DevPod. You must patch this Secret with a valid password (8+ characters, no colons or newlines) before the workspace pod will start:

```sh
kubectl patch secret my-devpod -n devpod-test -p \
  '{"data":{"devuser-password":"'"$(echo -n 'your-password' | base64)"'"}}'
```

The key name follows the pattern `<username>-password`.

## Make Targets

| Target             | Description                                                    |
|--------------------|----------------------------------------------------------------|
| `make build`       | Build manager binary to `bin/manager`                          |
| `make run`         | Run controller locally against current kubeconfig              |
| `make docker-build`| Build Docker image (`IMG=...` to set tag)                      |
| `make docker-push` | Push Docker image                                              |
| `make docker-buildx`| Multi-platform build and push (amd64, arm64, s390x, ppc64le) |
| `make install`     | Install CRDs into cluster                                      |
| `make uninstall`   | Remove CRDs from cluster                                       |
| `make deploy`      | Deploy controller to cluster (`IMG=...` to set image)          |
| `make undeploy`    | Remove controller from cluster                                 |
| `make manifests`   | Generate CRD and RBAC manifests from markers                   |
| `make generate`    | Generate DeepCopy implementations                              |
| `make build-installer` | Generate consolidated `dist/install.yaml`                  |
| `make test`        | Run unit tests with envtest                                    |
| `make test-e2e`    | Run e2e tests on a Kind cluster                                |
| `make lint`        | Run golangci-lint                                              |
| `make lint-fix`    | Run golangci-lint with auto-fix                                |
| `make fmt`         | Run `go fmt`                                                   |
| `make vet`         | Run `go vet`                                                   |
| `make help`        | Show all available targets                                     |

Pass `VERSION=x.y.z` to inject a semver into the binary via ldflags (defaults to `dev`).

## Development

### Running Tests

```sh
make test              # Unit tests (envtest — no real cluster needed)
make test-e2e          # E2E tests (creates a Kind cluster automatically)
make lint              # Linter
```

### Local Development

```sh
make install           # Install CRDs
make run               # Run the controller locally
```

## Uninstall

```sh
kubectl delete -k config/samples/
make undeploy
make uninstall
```

## License

Copyright 2026. Licensed under the Apache License, Version 2.0.
