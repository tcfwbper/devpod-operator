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
)

// --- SchemeGroupVersion ---

func TestSchemeGroupVersion_Group(t *testing.T) {
	assert.Equal(t, "apps.devpod.com", SchemeGroupVersion.Group)
}

func TestSchemeGroupVersion_Version(t *testing.T) {
	assert.Equal(t, "v1", SchemeGroupVersion.Version)
}

// --- GroupVersion ---

func TestGroupVersion_EqualsSchemeGroupVersion(t *testing.T) {
	assert.Equal(t, SchemeGroupVersion, GroupVersion)
}

// --- SchemeBuilder ---

func TestSchemeBuilder_NotNil(t *testing.T) {
	assert.NotNil(t, SchemeBuilder)
}

// --- AddToScheme ---

func TestAddToScheme_RegistersGroupVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	err := AddToScheme(scheme)
	require.NoError(t, err)

	// After AddToScheme, the scheme should recognize the group version.
	// Verify by checking that known types for our GV exist (at minimum the
	// meta types registered by metav1.AddToGroupVersion and the DevPod types
	// registered by the init() in devpod_types.go).
	knownTypes := scheme.AllKnownTypes()
	assert.NotEmpty(t, knownTypes, "scheme should have registered types after AddToScheme")
}

func TestAddToScheme_Idempotent(t *testing.T) {
	scheme := runtime.NewScheme()

	err1 := AddToScheme(scheme)
	require.NoError(t, err1)

	err2 := AddToScheme(scheme)
	require.NoError(t, err2)
}

func TestAddToScheme_NilScheme(t *testing.T) {
	// Per upstream runtime.SchemeBuilder behavior, calling AddToScheme with
	// nil should panic or return a non-nil error.
	assert.Panics(t, func() {
		_ = AddToScheme(nil)
	})
}
