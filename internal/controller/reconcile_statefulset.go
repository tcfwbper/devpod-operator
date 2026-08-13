package controller

import (
	"context"
	"fmt"

	kappsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
)

// reconcileStatefulSet ensures an owned StatefulSet exists for the DevPod and
// keeps its mutable fields in sync with the desired state.
func (r *DevPodReconciler) reconcileStatefulSet(ctx context.Context, dp *appsv1.DevPod) (*kappsv1.StatefulSet, error) {
	// Build desired StatefulSet
	desired := buildStatefulSet(dp)

	// Compute spec-hash
	hash, err := specHash(desired.Spec)
	if err != nil {
		return nil, fmt.Errorf("computing StatefulSet spec hash: %w", err)
	}
	setAnnotation(desired, specHashAnnotation, hash)

	// Set owner reference
	if err := ctrl.SetControllerReference(dp, desired, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting controller reference on StatefulSet: %w", err)
	}

	// Try to get existing
	existing := &kappsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: dp.Name, Namespace: dp.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("creating StatefulSet: %w", err)
		}
		return desired, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting StatefulSet: %w", err)
	}

	// Check if hash matches (no-op)
	existingHash := ""
	if existing.Annotations != nil {
		existingHash = existing.Annotations[specHashAnnotation]
	}
	if existingHash == hash {
		return existing, nil
	}

	// Update only mutable fields — Selector, ServiceName, PodManagementPolicy,
	// VolumeClaimTemplates are immutable in Kubernetes.
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.UpdateStrategy = desired.Spec.UpdateStrategy
	existing.Spec.Template = desired.Spec.Template
	existing.Labels = desired.Labels
	setAnnotation(existing, specHashAnnotation, hash)

	if err := r.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("updating StatefulSet: %w", err)
	}
	return existing, nil
}
