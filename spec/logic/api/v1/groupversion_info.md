# GroupVersion Registration

## Overview

Declares the API group identity (`apps.devpod.com/v1`) and provides the package-level scheme registration infrastructure (`SchemeBuilder`, `AddToScheme`) that all types in this package use to register themselves with a `runtime.Scheme`.

Does **not** define any resource types or register them — that is delegated to `devpod_types.go`'s `init()`.

## Boundaries

- Owns: the canonical `SchemeGroupVersion` and `GroupVersion` variables (the single source of truth for group name and version string).
- Owns: the initial `SchemeBuilder` instance (seeded with `metav1.AddToGroupVersion`).
- Owns: the `AddToScheme` function variable that external consumers call to register the entire package.
- Delegates: type registration (`DevPod`, `DevPodList`) to `devpod_types.go` via `SchemeBuilder.Register()`.
- Must not: define or reference any resource types.
- Must not: import packages outside `k8s.io/apimachinery`.

## Dependencies

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `k8s.io/apimachinery/pkg/runtime` | Scheme builder factory | Call `runtime.NewSchemeBuilder()` to create the builder | Must not instantiate `runtime.Scheme` |
| `k8s.io/apimachinery/pkg/runtime/schema` | GroupVersion type | Use `schema.GroupVersion{}` literal to define the group identity | — |
| `k8s.io/apimachinery/pkg/apis/meta/v1` | Group version utilities | Call `metav1.AddToGroupVersion()` as the SchemeBuilder's seed function | Must not use any type definitions from metav1 |

Construction constraints:
- `SchemeBuilder` is constructed via `runtime.NewSchemeBuilder()` at package init time. It must not be re-assigned after package initialization.
- The seed function passed to `NewSchemeBuilder` calls `metav1.AddToGroupVersion(scheme, SchemeGroupVersion)` to register the standard meta types for this group version.

## Behavior

1. Declares `SchemeGroupVersion` as `schema.GroupVersion{Group: "apps.devpod.com", Version: "v1"}`.
2. Declares `GroupVersion` as an alias for `SchemeGroupVersion` for backward compatibility.
3. Creates `SchemeBuilder` via `runtime.NewSchemeBuilder()` seeded with a function that calls `metav1.AddToGroupVersion(scheme, SchemeGroupVersion)`.
4. Exposes `AddToScheme` as `SchemeBuilder.AddToScheme` — a single function that callers (e.g., `main.go`) invoke to register all types in this package with a target Scheme.

## Inputs

This unit has no runtime inputs. Its package-level declarations are initialized at import time.

## Outputs

| Name | Type | Description |
|---|---|---|
| SchemeGroupVersion | schema.GroupVersion | The canonical group+version identity: `apps.devpod.com/v1` |
| GroupVersion | schema.GroupVersion | Alias for SchemeGroupVersion |
| SchemeBuilder | runtime.SchemeBuilder | Accumulator of type registration functions; other files in this package call `.Register()` on it |
| AddToScheme | func(*runtime.Scheme) error | Convenience function that applies all registered builders to a given Scheme |

## Invariants

- `SchemeGroupVersion.Group` must be `"apps.devpod.com"`.
- `SchemeGroupVersion.Version` must be `"v1"`.
- `GroupVersion` must always equal `SchemeGroupVersion`.
- `SchemeBuilder` must be initialized exactly once at package level; it must not be nil or re-assigned.
- `AddToScheme` must equal `SchemeBuilder.AddToScheme` (a direct assignment, not a wrapper).
- The package-level doc comment must carry `+kubebuilder:object:generate=true` and `+groupName=apps.devpod.com` markers for controller-gen.
- Must not import packages outside `k8s.io/apimachinery`.

## Edge Cases

- Condition: `AddToScheme` is called with a nil Scheme.
  Expected: Behavior is defined by upstream `runtime.SchemeBuilder` — this unit does not add nil-checking; it propagates whatever error the upstream returns.

- Condition: `AddToScheme` is called multiple times on the same Scheme.
  Expected: Idempotent — `SchemeBuilder.AddToScheme` re-registers without error per upstream contract.

## Related

- [DevPod CRD Schema](./devpod_types.md) — registers `DevPod` and `DevPodList` types through this unit's `SchemeBuilder`.
- `main.go` / manager setup — calls `AddToScheme` during bootstrap to wire types into the manager's Scheme.
