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

package controller

import (
	"context"
	"fmt"

	kappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
)

const finalizerName = "apps.devpod.com/finalizer"

// DevPodReconciler reconciles a DevPod object
type DevPodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.devpod.com,resources=devpods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.devpod.com,resources=devpods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.devpod.com,resources=devpods/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets;serviceaccounts;services,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DevPodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the DevPod CR
	dp := &appsv1.DevPod{}
	if err := r.Get(ctx, req.NamespacedName, dp); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 2. Check if being deleted
	if !dp.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, dp)
	}

	// 3. Ensure finalizer
	if !controllerutil.ContainsFinalizer(dp, finalizerName) {
		controllerutil.AddFinalizer(dp, finalizerName)
		if err := r.Update(ctx, dp); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// 4. Reconcile Secret
	secret, err := r.reconcileSecret(ctx, dp)
	if err != nil {
		log.Error(err, "reconciling Secret")
		return r.degraded(ctx, dp, appsv1.ReasonReconcileError, "failed to reconcile Secret", err)
	}
	dp.Status.PasswordSecret = secret.Name

	// 5. Check password
	password := string(secret.Data["ubuntu-password"])
	if password == "" {
		return r.pending(ctx, dp, appsv1.ReasonPasswordNotSet, "password not yet set in Secret")
	}
	if err := validatePassword(password); err != nil {
		return r.degraded(ctx, dp, appsv1.ReasonInvalidPassword, err.Error(), nil)
	}

	// 6. Verify StorageClass exists
	sc := &storagev1.StorageClass{}
	if err := r.Get(ctx, types.NamespacedName{Name: dp.Spec.Persistence.StorageClass}, sc); err != nil {
		if apierrors.IsNotFound(err) {
			return r.degraded(ctx, dp, appsv1.ReasonStorageClassNotFound,
				fmt.Sprintf("StorageClass %q not found", dp.Spec.Persistence.StorageClass), nil)
		}
		return r.degraded(ctx, dp, appsv1.ReasonReconcileError, "failed to check StorageClass", err)
	}

	// 7. Reconcile ServiceAccount
	if err := r.reconcileServiceAccount(ctx, dp); err != nil {
		log.Error(err, "reconciling ServiceAccount")
		return r.degraded(ctx, dp, appsv1.ReasonReconcileError, "failed to reconcile ServiceAccount", err)
	}

	// 8. Reconcile Service
	svc, err := r.reconcileService(ctx, dp)
	if err != nil {
		log.Error(err, "reconciling Service")
		return r.degraded(ctx, dp, appsv1.ReasonReconcileError, "failed to reconcile Service", err)
	}
	dp.Status.SSHNodePort = sshNodePort(svc)

	// 9. Reconcile StatefulSet
	sts, err := r.reconcileStatefulSet(ctx, dp, secret)
	if err != nil {
		log.Error(err, "reconciling StatefulSet")
		return r.degraded(ctx, dp, appsv1.ReasonReconcileError, "failed to reconcile StatefulSet", err)
	}
	dp.Status.ReadyReplicas = sts.Status.ReadyReplicas

	// 10. Check rollout status
	if sts.Status.ObservedGeneration < sts.Generation || sts.Status.ReadyReplicas < 1 {
		return r.progressing(ctx, dp, appsv1.ReasonWaitingForStatefulSet, "waiting for StatefulSet to be ready")
	}

	// 11. Ready
	return r.ready(ctx, dp)
}

// finalize handles cleanup when the DevPod is being deleted.
func (r *DevPodReconciler) finalize(ctx context.Context, dp *appsv1.DevPod) (ctrl.Result, error) {
	// If finalizer is absent, nothing to do
	if !controllerutil.ContainsFinalizer(dp, finalizerName) {
		return ctrl.Result{}, nil
	}

	// Delete PVCs if reclaim policy is Delete
	if dp.Spec.Persistence.ReclaimPolicy == appsv1.PVCReclaimDelete {
		pvcList := &corev1.PersistentVolumeClaimList{}
		labels := selectorLabels(dp)
		if err := r.List(ctx, pvcList, client.InNamespace(dp.Namespace), client.MatchingLabels(labels)); err != nil {
			return ctrl.Result{}, fmt.Errorf("listing PVCs for deletion: %w", err)
		}
		for i := range pvcList.Items {
			pvc := &pvcList.Items[i]
			if err := r.Delete(ctx, pvc); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return ctrl.Result{}, fmt.Errorf("deleting PVC %s: %w", pvc.Name, err)
			}
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(dp, finalizerName)
	if err := r.Update(ctx, dp); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DevPodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.DevPod{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&kappsv1.StatefulSet{}).
		Named("devpod").
		Complete(r)
}
