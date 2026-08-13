package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
)

const specHashAnnotation = "devpod.com/spec-hash"

// reconcileService ensures an owned NodePort Service exists for the DevPod.
func (r *DevPodReconciler) reconcileService(ctx context.Context, dp *appsv1.DevPod) (*corev1.Service, error) {
	// Build ports from spec
	svcPorts := make([]corev1.ServicePort, 0, len(dp.Spec.NodePorts))
	for _, np := range dp.Spec.NodePorts {
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name:       portName(np.Src),
			Protocol:   corev1.ProtocolTCP,
			Port:       np.Src,
			TargetPort: intstr.FromInt32(np.Src),
			NodePort:   np.Dest,
		})
	}

	// Construct desired Service
	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dp.Name,
			Namespace: dp.Namespace,
			Labels:    standardLabels(dp),
		},
		Spec: corev1.ServiceSpec{
			Type:                  corev1.ServiceTypeNodePort,
			Selector:              selectorLabels(dp),
			Ports:                 svcPorts,
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
			SessionAffinity:       corev1.ServiceAffinityNone,
		},
	}

	// Compute spec-hash
	hash, err := specHash(desired.Spec)
	if err != nil {
		return nil, fmt.Errorf("computing Service spec hash: %w", err)
	}
	setAnnotation(desired, specHashAnnotation, hash)

	// Set owner reference
	if err := ctrl.SetControllerReference(dp, desired, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting controller reference on Service: %w", err)
	}

	// Try to get existing
	existing := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: dp.Name, Namespace: dp.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("creating Service: %w", err)
		}
		return desired, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting Service: %w", err)
	}

	// Check if hash matches (no-op)
	existingHash := ""
	if existing.Annotations != nil {
		existingHash = existing.Annotations[specHashAnnotation]
	}
	if existingHash == hash {
		return existing, nil
	}

	// Update mutable fields, preserving ClusterIP
	existing.Spec.Type = desired.Spec.Type
	existing.Spec.Selector = desired.Spec.Selector
	existing.Spec.Ports = desired.Spec.Ports
	existing.Spec.ExternalTrafficPolicy = desired.Spec.ExternalTrafficPolicy
	existing.Spec.SessionAffinity = desired.Spec.SessionAffinity
	existing.Labels = desired.Labels
	setAnnotation(existing, specHashAnnotation, hash)

	if err := r.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("updating Service: %w", err)
	}
	return existing, nil
}
