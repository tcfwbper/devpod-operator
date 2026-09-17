package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
)

const minPasswordLength = 8

// reconcileSecret ensures an owned Secret exists for the DevPod with the
// secretKeyPassword key. It never overwrites an existing password value.
func (r *DevPodReconciler) reconcileSecret(ctx context.Context, dp *appsv1.DevPod) (*corev1.Secret, error) {
	// Construct desired Secret
	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dp.Name,
			Namespace: dp.Namespace,
			Labels:    standardLabels(dp),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			secretKeyPassword: {},
		},
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(dp, desired, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting controller reference on Secret: %w", err)
	}

	// Try to get existing
	existing := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: dp.Name, Namespace: dp.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("creating Secret: %w", err)
		}
		return desired, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting Secret: %w", err)
	}

	// Ensure the ubuntu-password key exists
	if existing.Data == nil {
		existing.Data = map[string][]byte{}
	}
	if _, ok := existing.Data[secretKeyPassword]; !ok {
		existing.Data[secretKeyPassword] = []byte("")
		if err := r.Update(ctx, existing); err != nil {
			return nil, fmt.Errorf("updating Secret to restore key: %w", err)
		}
	}

	return existing, nil
}

// validatePassword checks whether a password is acceptable for use with chpasswd.
func validatePassword(password string) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("password must be at least 8 characters long")
	}
	if strings.Contains(password, ":") {
		return fmt.Errorf("password must not contain \":\"")
	}
	if strings.Contains(password, "\n") || strings.Contains(password, "\r") {
		return fmt.Errorf("password must not contain a line break")
	}
	return nil
}
