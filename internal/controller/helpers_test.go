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

func TestSpecHash_DeterministicOutput(t *testing.T) {

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

	input1 := struct{ Name string }{Name: "alpha"}
	input2 := struct{ Name string }{Name: "beta"}

	hash1, err1 := specHash(input1)
	require.NoError(t, err1)

	hash2, err2 := specHash(input2)
	require.NoError(t, err2)

	assert.NotEqual(t, hash1, hash2, "different inputs must produce different hashes")
}

func TestSpecHash_Returns32HexChars(t *testing.T) {

	input := struct{ Value int }{Value: 42}
	hash, err := specHash(input)
	require.NoError(t, err)

	matched := regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(hash)
	assert.True(t, matched, "hash %q should match ^[0-9a-f]{32}$", hash)
}

func TestSpecHash_MarshalError(t *testing.T) {

	// Channels cannot be JSON-marshaled
	input := make(chan int)
	_, err := specHash(input)
	assert.Error(t, err, "should return error for unmarshalable value")
}

// =============================================================================
// secretDataHash
// =============================================================================

func TestSecretDataHash_DeterministicOutput(t *testing.T) {
	data := map[string][]byte{testSecretKey: []byte("val")}
	hash1 := secretDataHash(data)
	hash2 := secretDataHash(data)
	assert.Equal(t, hash1, hash2, "same input must produce same hash")
	assert.Len(t, hash1, 32, "hash should be 32 hex characters")
}

func TestSecretDataHash_Returns32HexChars(t *testing.T) {
	data := map[string][]byte{"k": []byte("v")}
	hash := secretDataHash(data)
	matched := regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(hash)
	assert.True(t, matched, "hash %q should match ^[0-9a-f]{32}$", hash)
}

func TestSecretDataHash_DifferentDataProducesDifferentHashes(t *testing.T) {
	data1 := map[string][]byte{testSecretKey: []byte("value1")}
	data2 := map[string][]byte{testSecretKey: []byte("value2")}
	hash1 := secretDataHash(data1)
	hash2 := secretDataHash(data2)
	assert.NotEqual(t, hash1, hash2, "different data must produce different hashes")
}

func TestSecretDataHash_OrderIndependent(t *testing.T) {
	data1 := map[string][]byte{"a": []byte("1"), "b": []byte("2")}
	data2 := map[string][]byte{"b": []byte("2"), "a": []byte("1")}
	hash1 := secretDataHash(data1)
	hash2 := secretDataHash(data2)
	assert.Equal(t, hash1, hash2, "hash must be independent of map iteration order")
}

func TestSecretDataHash_NilMap(t *testing.T) {
	hash := secretDataHash(nil)
	matched := regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(hash)
	assert.True(t, matched, "nil input should return a valid 32-char hex string, got %q", hash)
}

func TestSecretDataHash_EmptyMap(t *testing.T) {
	hashNil := secretDataHash(nil)
	hashEmpty := secretDataHash(map[string][]byte{})
	assert.Equal(t, hashNil, hashEmpty, "nil and empty map should produce the same hash")
}

// =============================================================================
// standardLabels
// =============================================================================

func TestStandardLabels_ContainsExpectedKeys(t *testing.T) {

	dp := &appsv1.DevPod{
		ObjectMeta: metav1.ObjectMeta{Name: testPodName},
	}
	labels := standardLabels(dp)

	assert.Equal(t, labelValueName, labels[labelKeyName])
	assert.Equal(t, testPodName, labels[labelKeyInstance])
	assert.Equal(t, "devpod-operator", labels["app.kubernetes.io/managed-by"])
}

func TestStandardLabels_ReturnsNewMapEachCall(t *testing.T) {

	dp := &appsv1.DevPod{
		ObjectMeta: metav1.ObjectMeta{Name: testPodName},
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

func TestSelectorLabels_ContainsSubsetOfStandard(t *testing.T) {

	dp := &appsv1.DevPod{
		ObjectMeta: metav1.ObjectMeta{Name: testPodName},
	}
	labels := selectorLabels(dp)

	assert.Equal(t, labelValueName, labels[labelKeyName])
	assert.Equal(t, testPodName, labels[labelKeyInstance])
	_, hasManagedBy := labels["app.kubernetes.io/managed-by"]
	assert.False(t, hasManagedBy, "selectorLabels must not contain managed-by")
}

func TestSelectorLabels_ReturnsNewMapEachCall(t *testing.T) {

	dp := &appsv1.DevPod{
		ObjectMeta: metav1.ObjectMeta{Name: testPodName},
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

func TestSetAnnotation_AddsAnnotation(t *testing.T) {

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

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"key": "old"},
		},
	}
	setAnnotation(obj, "key", "new")

	assert.Equal(t, "new", obj.Annotations["key"])
}

func TestSetAnnotation_NilAnnotationsMap(t *testing.T) {

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

func TestPortName_SSH(t *testing.T) {

	assert.Equal(t, portNameSSH, portName(22))
}

func TestPortName_NonSSH(t *testing.T) {

	assert.Equal(t, "port-8080", portName(8080))
}

func TestPortName_Zero(t *testing.T) {

	assert.Equal(t, "port-0", portName(0))
}

func TestPortName_MaxPort(t *testing.T) {

	assert.Equal(t, "port-65535", portName(65535))
}

// =============================================================================
// sshNodePort
// =============================================================================

func TestSSHNodePort_Found(t *testing.T) {

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

	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Ports: nil,
		},
	}
	assert.Equal(t, int32(0), sshNodePort(svc))
}
