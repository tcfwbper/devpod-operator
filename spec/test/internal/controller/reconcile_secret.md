# Test Specification: `reconcile_secret_test.go`

## Source File Under Test

`internal/controller/reconcile_secret.go`

## Test File

`internal/controller/reconcile_secret_test.go`

---

## `reconcileSecret`

### Happy Path — reconcileSecret

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileSecret_CreatesWhenNotFound` | `unit` | Creates a new Secret when none exists. | Fake client with no existing Secret; DevPod `Name="dev1"`, `Namespace="ns"` | `ctx`, `devpod` | Returns non-nil Secret with `Name="dev1"`, `Namespace="ns"`, Type=Opaque, Data contains `"ubuntu-password"` key with empty value; Secret has owner reference pointing to DevPod; labels match `standardLabels(devpod)` |
| `TestReconcileSecret_ReturnsExistingUnchanged` | `unit` | Returns existing Secret when key is present and populated. | Fake client with existing Secret containing `Data["ubuntu-password"] = []byte("s3cret!!")` | `ctx`, `devpod` | Returns the existing Secret with password value unchanged (`"s3cret!!"`) |
| `TestReconcileSecret_RestoresKeyWhenMissing` | `unit` | Adds the ubuntu-password key back when it was removed from an existing Secret. | Fake client with existing Secret whose Data map exists but has no `"ubuntu-password"` key | `ctx`, `devpod` | Returns the Secret with `"ubuntu-password"` key set to empty value; Update is issued |

### Null / Empty Input

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileSecret_NilDataMap` | `unit` | Handles existing Secret with nil Data map. | Fake client with existing Secret whose `.Data` is nil | `ctx`, `devpod` | Returns Secret with `Data` initialized and `"ubuntu-password"` key set to empty value; Update is issued |
| `TestReconcileSecret_EmptyPasswordNotOverwritten` | `unit` | Does not overwrite when key exists with empty value. | Fake client with existing Secret containing `Data["ubuntu-password"] = []byte("")` | `ctx`, `devpod` | Returns Secret unchanged; no Update issued |

### Error Propagation

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileSecret_GetError` | `unit` | Returns error when Get fails with a non-NotFound error. | Fake client configured to return an error on Get | `ctx`, `devpod` | Returns nil Secret and non-nil error |
| `TestReconcileSecret_CreateError` | `unit` | Returns error when Create fails. | Fake client configured to return NotFound on Get and error on Create | `ctx`, `devpod` | Returns nil Secret and non-nil error |
| `TestReconcileSecret_UpdateError` | `unit` | Returns error when Update fails. | Fake client with existing Secret (missing key), configured to return error on Update | `ctx`, `devpod` | Returns nil Secret and non-nil error |

### Mock / Dependency Interaction

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestReconcileSecret_SetsOwnerReference` | `unit` | Sets DevPod as controller owner on creation. | Fake client with no existing Secret; scheme registered with DevPod types | `ctx`, `devpod` | Created Secret has OwnerReferences with Controller=true pointing to the DevPod |

---

## `validatePassword`

### Happy Path — validatePassword

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestValidatePassword_ValidMinLength` | `unit` | Accepts password with exactly 8 characters. | | `password = "abcd1234"` | Returns `nil` |
| `TestValidatePassword_ValidLong` | `unit` | Accepts a longer valid password. | | `password = "a-very-secure-passphrase"` | Returns `nil` |

### Validation Failures

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestValidatePassword_TooShort` | `unit` | Rejects password shorter than 8 characters. | | `password = "short"` | Returns non-nil error containing "at least 8 characters" |
| `TestValidatePassword_ContainsColon` | `unit` | Rejects password containing a colon. | | `password = "pass:word1"` | Returns non-nil error containing `":"` |
| `TestValidatePassword_ContainsNewline` | `unit` | Rejects password containing a newline. | | `password = "pass\nword1"` | Returns non-nil error containing "line break" |
| `TestValidatePassword_ContainsCarriageReturn` | `unit` | Rejects password containing a carriage return. | | `password = "pass\rword1"` | Returns non-nil error containing "line break" |

### Boundary Values — password

| Test ID | Category | Description | Setup | Input | Expected |
|---|---|---|---|---|---|
| `TestValidatePassword_ExactlySevenChars` | `unit` | Rejects password of length 7. | | `password = "1234567"` | Returns non-nil error |
| `TestValidatePassword_ExactlyEightChars` | `unit` | Accepts password of length 8. | | `password = "12345678"` | Returns `nil` |
