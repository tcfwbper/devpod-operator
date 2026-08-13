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

func TestConditionTypeConstants(t *testing.T) {
	assert.Equal(t, "Ready", ConditionReady)
	assert.Equal(t, "Progressing", ConditionProgressing)
	assert.Equal(t, "Degraded", ConditionDegraded)
}

func TestReasonConstants(t *testing.T) {
	assert.Equal(t, "PasswordNotSet", ReasonPasswordNotSet)
	assert.Equal(t, "InvalidPassword", ReasonInvalidPassword)
	assert.Equal(t, "StorageClassNotFound", ReasonStorageClassNotFound)
	assert.Equal(t, "ReconcileError", ReasonReconcileError)
	assert.Equal(t, "WaitingForStatefulSet", ReasonWaitingForStatefulSet)
	assert.Equal(t, "DevPodReady", ReasonDevPodReady)
}

func TestPVCReclaimPolicyValues(t *testing.T) {
	assert.Equal(t, PVCReclaimPolicy("Retain"), PVCReclaimRetain)
	assert.Equal(t, PVCReclaimPolicy("Delete"), PVCReclaimDelete)
}

// =============================================================================
// DevPodSpec — DockerEnabled
// =============================================================================

func TestDevPodSpec_DockerEnabled_NilDockerEnabled(t *testing.T) {
	spec := &DevPodSpec{
		Docker: DockerSpec{Enabled: nil},
	}
	assert.True(t, spec.DockerEnabled())
}

func TestDevPodSpec_DockerEnabled_ExplicitTrue(t *testing.T) {
	enabled := true
	spec := &DevPodSpec{
		Docker: DockerSpec{Enabled: &enabled},
	}
	assert.True(t, spec.DockerEnabled())
}

func TestDevPodSpec_DockerEnabled_ExplicitFalse(t *testing.T) {
	enabled := false
	spec := &DevPodSpec{
		Docker: DockerSpec{Enabled: &enabled},
	}
	assert.False(t, spec.DockerEnabled())
}

func TestDevPodSpec_DockerEnabled_ZeroValueDockerSpec(t *testing.T) {
	spec := &DevPodSpec{
		Docker: DockerSpec{},
	}
	assert.True(t, spec.DockerEnabled())
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
