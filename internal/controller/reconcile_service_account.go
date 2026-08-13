package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
)

// reconcileServiceAccount ensures an owned ServiceAccount exists for the DevPod
// with AutomountServiceAccountToken=false.
func (r *DevPodReconciler) reconcileServiceAccount(ctx context.Context, dp *appsv1.DevPod) error {
	automount := false
	desired := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dp.Name,
			Namespace: dp.Namespace,
			Labels:    standardLabels(dp),
		},
		AutomountServiceAccountToken: &automount,
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(dp, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting controller reference on ServiceAccount: %w", err)
	}

	// Try to get existing
	existing := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: dp.Name, Namespace: dp.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating ServiceAccount: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting ServiceAccount: %w", err)
	}

	// Ensure AutomountServiceAccountToken is false
	if existing.AutomountServiceAccountToken == nil || *existing.AutomountServiceAccountToken {
		existing.AutomountServiceAccountToken = &automount
		if err := r.Update(ctx, existing); err != nil {
			return fmt.Errorf("updating ServiceAccount: %w", err)
		}
	}

	return nil
}
