package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
)

const requeueInterval = 10 * time.Second

// conditionSet groups the status of all three conditions to enforce mutual exclusion.
type conditionSet struct {
	ready       metav1.ConditionStatus
	progressing metav1.ConditionStatus
	degraded    metav1.ConditionStatus
	reason      string
	message     string
}

// setConditions applies the conditionSet to the DevPod's status conditions.
func setConditions(dp *appsv1.DevPod, cs conditionSet) {
	gen := dp.Generation

	meta.SetStatusCondition(&dp.Status.Conditions, metav1.Condition{
		Type:               appsv1.ConditionReady,
		Status:             cs.ready,
		ObservedGeneration: gen,
		Reason:             cs.reason,
		Message:            cs.message,
	})
	meta.SetStatusCondition(&dp.Status.Conditions, metav1.Condition{
		Type:               appsv1.ConditionProgressing,
		Status:             cs.progressing,
		ObservedGeneration: gen,
		Reason:             cs.reason,
		Message:            cs.message,
	})
	meta.SetStatusCondition(&dp.Status.Conditions, metav1.Condition{
		Type:               appsv1.ConditionDegraded,
		Status:             cs.degraded,
		ObservedGeneration: gen,
		Reason:             cs.reason,
		Message:            cs.message,
	})
}

// updateStatus writes observedGeneration and issues a status subresource update.
func (r *DevPodReconciler) updateStatus(ctx context.Context, dp *appsv1.DevPod) error {
	dp.Status.ObservedGeneration = dp.Generation
	return r.Status().Update(ctx, dp)
}

// ready reports the DevPod as fully operational.
func (r *DevPodReconciler) ready(ctx context.Context, dp *appsv1.DevPod) (ctrl.Result, error) {
	setConditions(dp, conditionSet{
		ready:       metav1.ConditionTrue,
		progressing: metav1.ConditionFalse,
		degraded:    metav1.ConditionFalse,
		reason:      appsv1.ReasonDevPodReady,
		message:     fmt.Sprintf("DevPod is reachable on node port %d", dp.Status.SSHNodePort),
	})
	if err := r.updateStatus(ctx, dp); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// progressing reports the DevPod as rolling out and requeues after requeueInterval.
func (r *DevPodReconciler) progressing(ctx context.Context, dp *appsv1.DevPod, reason, message string) (ctrl.Result, error) {
	setConditions(dp, conditionSet{
		ready:       metav1.ConditionFalse,
		progressing: metav1.ConditionTrue,
		degraded:    metav1.ConditionFalse,
		reason:      reason,
		message:     message,
	})
	if err := r.updateStatus(ctx, dp); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

// pending reports the DevPod as waiting for human input (no requeue).
func (r *DevPodReconciler) pending(ctx context.Context, dp *appsv1.DevPod, reason, message string) (ctrl.Result, error) {
	setConditions(dp, conditionSet{
		ready:       metav1.ConditionFalse,
		progressing: metav1.ConditionFalse,
		degraded:    metav1.ConditionFalse,
		reason:      reason,
		message:     message,
	})
	if err := r.updateStatus(ctx, dp); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// degraded reports the DevPod as failed. If cause is non-nil, returns it for
// exponential backoff. If nil, requeues after requeueInterval.
func (r *DevPodReconciler) degraded(ctx context.Context, dp *appsv1.DevPod, reason, message string, cause error) (ctrl.Result, error) {
	setConditions(dp, conditionSet{
		ready:       metav1.ConditionFalse,
		progressing: metav1.ConditionFalse,
		degraded:    metav1.ConditionTrue,
		reason:      reason,
		message:     message,
	})
	if err := r.updateStatus(ctx, dp); err != nil {
		return ctrl.Result{}, err
	}
	if cause != nil {
		return ctrl.Result{}, cause
	}
	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}
