# Test Specification: `helpers_test.go`

## Source File Under Test

`internal/controller/helpers.go`

## Test File

`internal/controller/helpers_test.go`

---

## `specHash`

### Happy Path — specHash

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestSpecHash_DeterministicOutput` | `unit` | Same input struct always produces the same hash string. | | A fixed struct value passed twice | Both calls return identical 32-character hex strings and nil error |
| `TestSpecHash_DifferentInputsProduceDifferentHashes` | `unit` | Different structs produce different hashes. | | Two structs with differing field values | Returned hash strings differ |
| `TestSpecHash_Returns32HexChars` | `unit` | Output is exactly 32 hex characters. | | A valid struct | Returned string matches regexp `^[0-9a-f]{32}$` |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestSpecHash_MarshalError` | `unit` | Returns error when JSON marshaling fails. | | A value containing a channel field (cannot be marshaled) | Returns non-nil error |

---

## `standardLabels`

### Happy Path — standardLabels

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestStandardLabels_ContainsExpectedKeys` | `unit` | Returns a map with all three expected label keys. | | A `DevPod` with `Name = "my-pod"` | Map contains `app.kubernetes.io/name` = `"devpod"`, `app.kubernetes.io/instance` = `"my-pod"`, `app.kubernetes.io/managed-by` = `"devpod-operator"` |
| `TestStandardLabels_ReturnsNewMapEachCall` | `unit` | Each invocation returns an independent map instance. | | Same `DevPod` passed twice | Mutating the first returned map does not affect the second |

---

## `selectorLabels`

### Happy Path — selectorLabels

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestSelectorLabels_ContainsSubsetOfStandard` | `unit` | Returns only the two selector keys. | | A `DevPod` with `Name = "my-pod"` | Map contains `app.kubernetes.io/name` = `"devpod"`, `app.kubernetes.io/instance` = `"my-pod"`; does NOT contain `app.kubernetes.io/managed-by` |
| `TestSelectorLabels_ReturnsNewMapEachCall` | `unit` | Each invocation returns an independent map instance. | | Same `DevPod` passed twice | Mutating the first returned map does not affect the second |

---

## `setAnnotation`

### Happy Path — setAnnotation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestSetAnnotation_AddsAnnotation` | `unit` | Sets the annotation on an object that already has an annotations map. | Object with existing annotations `{"existing-key": "val"}` | `key = "new-key"`, `value = "new-val"` | Object annotations contain both `"existing-key"` and `"new-key"` with correct values |
| `TestSetAnnotation_OverwritesExistingKey` | `unit` | Overwrites the value of an already-present key. | Object with annotations `{"key": "old"}` | `key = "key"`, `value = "new"` | Annotation `"key"` has value `"new"` |

### Null / Empty Input

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestSetAnnotation_NilAnnotationsMap` | `unit` | Initializes the annotations map when it is nil. | Object with `GetAnnotations()` returning nil | `key = "k"`, `value = "v"` | Object annotations are non-nil and contain `"k" = "v"` |

---

## `portName`

### Happy Path — portName

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestPortName_SSH` | `unit` | Returns "ssh" for port 22. | | `port = 22` | Returns `"ssh"` |
| `TestPortName_NonSSH` | `unit` | Returns "port-N" for a non-22 port. | | `port = 8080` | Returns `"port-8080"` |

### Boundary Values — port

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestPortName_Zero` | `unit` | Returns "port-0" for invalid port 0. | | `port = 0` | Returns `"port-0"` |
| `TestPortName_MaxPort` | `unit` | Returns "port-65535" for max valid port. | | `port = 65535` | Returns `"port-65535"` |

---

## `sshNodePort`

### Happy Path — sshNodePort

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestSSHNodePort_Found` | `unit` | Returns the NodePort of the first entry with Port=22. | A Service with `Spec.Ports` containing `{Port: 22, NodePort: 30022}` and `{Port: 80, NodePort: 30080}` | | Returns `int32(30022)` |

### Null / Empty Input

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestSSHNodePort_NoPort22` | `unit` | Returns 0 when no port 22 entry exists. | A Service with `Spec.Ports` containing only `{Port: 80, NodePort: 30080}` | | Returns `int32(0)` |
| `TestSSHNodePort_EmptyPorts` | `unit` | Returns 0 when ports slice is empty. | A Service with `Spec.Ports = nil` | | Returns `int32(0)` |
