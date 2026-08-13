/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// =============================================================================
// Constants
// =============================================================================

// Status: scaffolded
// Blocker: Constants ConditionReady, ConditionProgressing, ConditionDegraded are
// not yet defined in devpod_types.go.
func TestConditionTypeConstants(t *testing.T) {
	t.Skip("scaffolded: ConditionReady, ConditionProgressing, ConditionDegraded constants not yet defined in devpod_types.go")

	// Once the constants are defined, uncomment and verify:
	// assert.Equal(t, "Ready", ConditionReady)
	// assert.Equal(t, "Progressing", ConditionProgressing)
	// assert.Equal(t, "Degraded", ConditionDegraded)
}

// Status: scaffolded
// Blocker: Reason constants (ReasonPasswordNotSet, ReasonInvalidPassword,
// ReasonStorageClassNotFound, ReasonReconcileError, ReasonWaitingForStatefulSet,
// ReasonDevPodReady) are not yet defined in devpod_types.go.
func TestReasonConstants(t *testing.T) {
	t.Skip("scaffolded: Reason constants (ReasonPasswordNotSet, ReasonInvalidPassword, ReasonStorageClassNotFound, ReasonReconcileError, ReasonWaitingForStatefulSet, ReasonDevPodReady) not yet defined in devpod_types.go")

	// Once the constants are defined, uncomment and verify:
	// assert.Equal(t, "PasswordNotSet", ReasonPasswordNotSet)
	// assert.Equal(t, "InvalidPassword", ReasonInvalidPassword)
	// assert.Equal(t, "StorageClassNotFound", ReasonStorageClassNotFound)
	// assert.Equal(t, "ReconcileError", ReasonReconcileError)
	// assert.Equal(t, "WaitingForStatefulSet", ReasonWaitingForStatefulSet)
	// assert.Equal(t, "DevPodReady", ReasonDevPodReady)
}

// Status: scaffolded
// Blocker: PVCReclaimPolicy type and constants (PVCReclaimRetain, PVCReclaimDelete)
// are not yet defined in devpod_types.go.
func TestPVCReclaimPolicyValues(t *testing.T) {
	t.Skip("scaffolded: PVCReclaimPolicy type and constants (PVCReclaimRetain, PVCReclaimDelete) not yet defined in devpod_types.go")

	// Once the type and constants are defined, uncomment and verify:
	// assert.Equal(t, PVCReclaimPolicy("Retain"), PVCReclaimRetain)
	// assert.Equal(t, PVCReclaimPolicy("Delete"), PVCReclaimDelete)
}

// =============================================================================
// DevPodSpec — DockerEnabled
// =============================================================================

// Status: scaffolded
// Blocker: DockerEnabled() method and DockerSpec/Docker field on DevPodSpec are
// not yet defined in devpod_types.go.
func TestDevPodSpec_DockerEnabled_NilDockerEnabled(t *testing.T) {
	t.Skip("scaffolded: DockerEnabled() method and Docker field on DevPodSpec not yet defined in devpod_types.go")

	// Once DockerSpec and DockerEnabled() are implemented:
	// spec := &DevPodSpec{
	//     Docker: DockerSpec{Enabled: nil},
	// }
	// assert.True(t, spec.DockerEnabled())
}

// Status: scaffolded
// Blocker: DockerEnabled() method and DockerSpec/Docker field on DevPodSpec are
// not yet defined in devpod_types.go.
func TestDevPodSpec_DockerEnabled_ExplicitTrue(t *testing.T) {
	t.Skip("scaffolded: DockerEnabled() method and Docker field on DevPodSpec not yet defined in devpod_types.go")

	// Once DockerSpec and DockerEnabled() are implemented:
	// enabled := true
	// spec := &DevPodSpec{
	//     Docker: DockerSpec{Enabled: &enabled},
	// }
	// assert.True(t, spec.DockerEnabled())
}

// Status: scaffolded
// Blocker: DockerEnabled() method and DockerSpec/Docker field on DevPodSpec are
// not yet defined in devpod_types.go.
func TestDevPodSpec_DockerEnabled_ExplicitFalse(t *testing.T) {
	t.Skip("scaffolded: DockerEnabled() method and Docker field on DevPodSpec not yet defined in devpod_types.go")

	// Once DockerSpec and DockerEnabled() are implemented:
	// enabled := false
	// spec := &DevPodSpec{
	//     Docker: DockerSpec{Enabled: &enabled},
	// }
	// assert.False(t, spec.DockerEnabled())
}

// Status: scaffolded
// Blocker: DockerEnabled() method and DockerSpec/Docker field on DevPodSpec are
// not yet defined in devpod_types.go.
func TestDevPodSpec_DockerEnabled_ZeroValueDockerSpec(t *testing.T) {
	t.Skip("scaffolded: DockerEnabled() method and Docker field on DevPodSpec not yet defined in devpod_types.go")

	// Once DockerSpec and DockerEnabled() are implemented:
	// spec := &DevPodSpec{
	//     Docker: DockerSpec{},
	// }
	// assert.True(t, spec.DockerEnabled())
}

// =============================================================================
// Scheme Registration
// =============================================================================

// Status: concrete
func TestSchemeRegistration_DevPodRegistered(t *testing.T) {
	scheme := runtime.NewScheme()
	err := AddToScheme(scheme)
	require.NoError(t, err)

	gvk := schema.GroupVersionKind{
		Group:   "apps.devpod.com",
		Version: "v1",
		Kind:    "DevPod",
	}
	knownTypes := scheme.AllKnownTypes()
	_, found := knownTypes[gvk]
	assert.True(t, found, "DevPod type should be registered in scheme with GVK %v", gvk)
}

// Status: concrete
func TestSchemeRegistration_DevPodListRegistered(t *testing.T) {
	scheme := runtime.NewScheme()
	err := AddToScheme(scheme)
	require.NoError(t, err)

	gvk := schema.GroupVersionKind{
		Group:   "apps.devpod.com",
		Version: "v1",
		Kind:    "DevPodList",
	}
	knownTypes := scheme.AllKnownTypes()
	_, found := knownTypes[gvk]
	assert.True(t, found, "DevPodList type should be registered in scheme with GVK %v", gvk)
}

// Status: concrete
func TestSchemeRegistration_Idempotent(t *testing.T) {
	scheme := runtime.NewScheme()

	err1 := AddToScheme(scheme)
	require.NoError(t, err1)

	typesAfterFirst := scheme.AllKnownTypes()

	err2 := AddToScheme(scheme)
	require.NoError(t, err2)

	typesAfterSecond := scheme.AllKnownTypes()
	assert.Equal(t, len(typesAfterFirst), len(typesAfterSecond),
		"calling AddToScheme twice should not duplicate registrations")
}
