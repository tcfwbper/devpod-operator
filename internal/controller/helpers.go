package controller

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
)

// specHash JSON-marshals the provided value and returns the first 32 hex
// characters of its SHA-256 hash. Returns an error if marshaling fails.
func specHash(spec interface{}) (string, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("computing spec hash: %w", err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)[:32], nil
}

// secretDataHash computes a deterministic hash of a Kubernetes Secret's .Data
// field. Keys are sorted lexicographically before hashing to ensure the result
// is independent of Go map iteration order. Returns the first 32 hex characters
// of the SHA-256 hash.
func secretDataHash(data map[string][]byte) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf []byte
	for _, k := range keys {
		buf = append(buf, []byte(k)...)
		buf = append(buf, data[k]...)
	}

	sum := sha256.Sum256(buf)
	return fmt.Sprintf("%x", sum)[:32]
}

// standardLabels returns the full set of labels applied to all resources owned
// by the given DevPod.
func standardLabels(dp *appsv1.DevPod) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "devpod",
		"app.kubernetes.io/instance":   dp.Name,
		"app.kubernetes.io/managed-by": "devpod-operator",
	}
}

// selectorLabels returns the stable subset of labels used for pod selectors.
// It excludes managed-by because selectors must be immutable.
func selectorLabels(dp *appsv1.DevPod) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "devpod",
		"app.kubernetes.io/instance": dp.Name,
	}
}

// setAnnotation sets a single annotation on the given Kubernetes object,
// initializing the annotations map if nil.
func setAnnotation(obj client.Object, key, value string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = value
	obj.SetAnnotations(annotations)
}

// portName returns a DNS-1123 compliant port name. Port 22 gets the name "ssh";
// all others get "port-<n>".
func portName(port int32) string {
	if port == 22 {
		return "ssh"
	}
	return fmt.Sprintf("port-%d", port)
}

// sshNodePort returns the NodePort of the first Service port entry whose Port
// equals 22. Returns 0 if no such entry exists.
func sshNodePort(svc *corev1.Service) int32 {
	for _, p := range svc.Spec.Ports {
		if p.Port == 22 {
			return p.NodePort
		}
	}
	return 0
}
