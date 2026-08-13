package controller

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// =============================================================================
// specHash
// =============================================================================

// scaffolded: awaiting specHash from internal/controller/helpers.go

func TestSpecHash_DeterministicOutput(t *testing.T) {
	t.Skip("scaffolded: awaiting specHash from helpers.go")

	input := struct {
		Name string
		Port int
	}{Name: "test", Port: 8080}

	hash1, err1 := specHash(input)
	require.NoError(t, err1)

	hash2, err2 := specHash(input)
	require.NoError(t, err2)

	assert.Equal(t, hash1, hash2, "same input must produce same hash")
	assert.Len(t, hash1, 32, "hash should be 32 hex characters")
}

func TestSpecHash_DifferentInputsProduceDifferentHashes(t *testing.T) {
	t.Skip("scaffolded: awaiting specHash from helpers.go")

	input1 := struct{ Name string }{Name: "alpha"}
	input2 := struct{ Name string }{Name: "beta"}

	hash1, err1 := specHash(input1)
	require.NoError(t, err1)

	hash2, err2 := specHash(input2)
	require.NoError(t, err2)

	assert.NotEqual(t, hash1, hash2, "different inputs must produce different hashes")
}

func TestSpecHash_Returns32HexChars(t *testing.T) {
	t.Skip("scaffolded: awaiting specHash from helpers.go")

	input := struct{ Value int }{Value: 42}
	hash, err := specHash(input)
	require.NoError(t, err)

	matched := regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(hash)
	assert.True(t, matched, "hash %q should match ^[0-9a-f]{32}$", hash)
}

func TestSpecHash_MarshalError(t *testing.T) {
	t.Skip("scaffolded: awaiting specHash from helpers.go")

	// Channels cannot be JSON-marshaled
	input := make(chan int)
	_, err := specHash(input)
	assert.Error(t, err, "should return error for unmarshalable value")
}

// =============================================================================
// standardLabels
// =============================================================================

// scaffolded: awaiting standardLabels from internal/controller/helpers.go

func TestStandardLabels_ContainsExpectedKeys(t *testing.T) {
	t.Skip("scaffolded: awaiting standardLabels from helpers.go")

	dp := &appsv1.DevPod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod"},
	}
	labels := standardLabels(dp)

	assert.Equal(t, "devpod", labels["app.kubernetes.io/name"])
	assert.Equal(t, "my-pod", labels["app.kubernetes.io/instance"])
	assert.Equal(t, "devpod-operator", labels["app.kubernetes.io/managed-by"])
}

func TestStandardLabels_ReturnsNewMapEachCall(t *testing.T) {
	t.Skip("scaffolded: awaiting standardLabels from helpers.go")

	dp := &appsv1.DevPod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod"},
	}
	labels1 := standardLabels(dp)
	labels2 := standardLabels(dp)

	// Mutate first map
	labels1["extra"] = "value"

	// Second map should be unaffected
	_, hasExtra := labels2["extra"]
	assert.False(t, hasExtra, "mutation of first map must not affect second")
}

// =============================================================================
// selectorLabels
// =============================================================================

// scaffolded: awaiting selectorLabels from internal/controller/helpers.go

func TestSelectorLabels_ContainsSubsetOfStandard(t *testing.T) {
	t.Skip("scaffolded: awaiting selectorLabels from helpers.go")

	dp := &appsv1.DevPod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod"},
	}
	labels := selectorLabels(dp)

	assert.Equal(t, "devpod", labels["app.kubernetes.io/name"])
	assert.Equal(t, "my-pod", labels["app.kubernetes.io/instance"])
	_, hasManagedBy := labels["app.kubernetes.io/managed-by"]
	assert.False(t, hasManagedBy, "selectorLabels must not contain managed-by")
}

func TestSelectorLabels_ReturnsNewMapEachCall(t *testing.T) {
	t.Skip("scaffolded: awaiting selectorLabels from helpers.go")

	dp := &appsv1.DevPod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod"},
	}
	labels1 := selectorLabels(dp)
	labels2 := selectorLabels(dp)

	labels1["extra"] = "value"

	_, hasExtra := labels2["extra"]
	assert.False(t, hasExtra, "mutation of first map must not affect second")
}

// =============================================================================
// setAnnotation
// =============================================================================

// scaffolded: awaiting setAnnotation from internal/controller/helpers.go

func TestSetAnnotation_AddsAnnotation(t *testing.T) {
	t.Skip("scaffolded: awaiting setAnnotation from helpers.go")

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"existing-key": "val"},
		},
	}
	setAnnotation(obj, "new-key", "new-val")

	assert.Equal(t, "val", obj.Annotations["existing-key"])
	assert.Equal(t, "new-val", obj.Annotations["new-key"])
}

func TestSetAnnotation_OverwritesExistingKey(t *testing.T) {
	t.Skip("scaffolded: awaiting setAnnotation from helpers.go")

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"key": "old"},
		},
	}
	setAnnotation(obj, "key", "new")

	assert.Equal(t, "new", obj.Annotations["key"])
}

func TestSetAnnotation_NilAnnotationsMap(t *testing.T) {
	t.Skip("scaffolded: awaiting setAnnotation from helpers.go")

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{},
	}
	setAnnotation(obj, "k", "v")

	require.NotNil(t, obj.Annotations)
	assert.Equal(t, "v", obj.Annotations["k"])
}

// =============================================================================
// portName
// =============================================================================

// scaffolded: awaiting portName from internal/controller/helpers.go

func TestPortName_SSH(t *testing.T) {
	t.Skip("scaffolded: awaiting portName from helpers.go")

	assert.Equal(t, "ssh", portName(22))
}

func TestPortName_NonSSH(t *testing.T) {
	t.Skip("scaffolded: awaiting portName from helpers.go")

	assert.Equal(t, "port-8080", portName(8080))
}

func TestPortName_Zero(t *testing.T) {
	t.Skip("scaffolded: awaiting portName from helpers.go")

	assert.Equal(t, "port-0", portName(0))
}

func TestPortName_MaxPort(t *testing.T) {
	t.Skip("scaffolded: awaiting portName from helpers.go")

	assert.Equal(t, "port-65535", portName(65535))
}

// =============================================================================
// sshNodePort
// =============================================================================

// scaffolded: awaiting sshNodePort from internal/controller/helpers.go

func TestSSHNodePort_Found(t *testing.T) {
	t.Skip("scaffolded: awaiting sshNodePort from helpers.go")

	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 22, NodePort: 30022},
				{Port: 80, NodePort: 30080},
			},
		},
	}
	assert.Equal(t, int32(30022), sshNodePort(svc))
}

func TestSSHNodePort_NoPort22(t *testing.T) {
	t.Skip("scaffolded: awaiting sshNodePort from helpers.go")

	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 80, NodePort: 30080},
			},
		},
	}
	assert.Equal(t, int32(0), sshNodePort(svc))
}

func TestSSHNodePort_EmptyPorts(t *testing.T) {
	t.Skip("scaffolded: awaiting sshNodePort from helpers.go")

	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Ports: nil,
		},
	}
	assert.Equal(t, int32(0), sshNodePort(svc))
}
