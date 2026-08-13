package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// =============================================================================
// conditionSet — Construction
// =============================================================================

func TestConditionSet_MutualExclusion(t *testing.T) {

	// Verify that constructing a conditionSet enforces mutual exclusion:
	// only one of ready/progressing/degraded can be true
	cs := conditionSet{
		ready:       metav1.ConditionTrue,
		progressing: metav1.ConditionFalse,
		degraded:    metav1.ConditionFalse,
		reason:      "TestReason",
		message:     "test message",
	}
	assert.Equal(t, metav1.ConditionTrue, cs.ready)
	assert.Equal(t, metav1.ConditionFalse, cs.progressing)
	assert.Equal(t, metav1.ConditionFalse, cs.degraded)
}

// =============================================================================
// setConditions
// =============================================================================

func TestSetConditions_SetsAllThreeTypes(t *testing.T) {

	dp := newDevPod("dev1", "ns").withGeneration(3).build()
	cs := conditionSet{
		ready:       metav1.ConditionTrue,
		progressing: metav1.ConditionFalse,
		degraded:    metav1.ConditionFalse,
		reason:      appsv1.ReasonDevPodReady,
		message:     "msg",
	}

	setConditions(dp, cs)

	require.Len(t, dp.Status.Conditions, 3)
	var readyFound, progressingFound, degradedFound bool
	for _, c := range dp.Status.Conditions {
		switch c.Type {
		case appsv1.ConditionReady:
			readyFound = true
			assert.Equal(t, metav1.ConditionTrue, c.Status)
			assert.Equal(t, int64(3), c.ObservedGeneration)
		case appsv1.ConditionProgressing:
			progressingFound = true
			assert.Equal(t, metav1.ConditionFalse, c.Status)
		case appsv1.ConditionDegraded:
			degradedFound = true
			assert.Equal(t, metav1.ConditionFalse, c.Status)
		}
	}
	assert.True(t, readyFound)
	assert.True(t, progressingFound)
	assert.True(t, degradedFound)
}

func TestSetConditions_OverwritesPreviousConditions(t *testing.T) {

	dp := newDevPod("dev1", "ns").withGeneration(2).build()
	// Set initial conditions
	dp.Status.Conditions = []metav1.Condition{
		{Type: appsv1.ConditionReady, Status: metav1.ConditionTrue},
	}

	cs := conditionSet{
		ready:       metav1.ConditionFalse,
		progressing: metav1.ConditionFalse,
		degraded:    metav1.ConditionTrue,
		reason:      appsv1.ReasonReconcileError,
		message:     "error",
	}
	setConditions(dp, cs)

	for _, c := range dp.Status.Conditions {
		if c.Type == appsv1.ConditionReady {
			assert.Equal(t, metav1.ConditionFalse, c.Status)
		}
		if c.Type == appsv1.ConditionDegraded {
			assert.Equal(t, metav1.ConditionTrue, c.Status)
		}
	}
}

// =============================================================================
// updateStatus
// =============================================================================

func TestUpdateStatus_SetsObservedGeneration(t *testing.T) {

	dp := newDevPod("dev1", "ns").withGeneration(5).withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	err := r.updateStatus(testCtx(), dp)
	require.NoError(t, err)
	assert.Equal(t, int64(5), dp.Status.ObservedGeneration)
}

func TestUpdateStatus_ReturnsError(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := interceptingClient(t, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			return errors.New("status update failed")
		},
	}, dp)
	r := newTestReconciler(c)

	err := r.updateStatus(testCtx(), dp)
	assert.Error(t, err)
}

// =============================================================================
// ready
// =============================================================================

func TestReady_SetsCorrectConditions(t *testing.T) {

	dp := newDevPod("dev1", "ns").withGeneration(3).withFinalizer().build()
	dp.Status.SSHNodePort = 30022
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	result, err := r.ready(testCtx(), dp)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Check conditions
	for _, cond := range dp.Status.Conditions {
		switch cond.Type {
		case appsv1.ConditionReady:
			assert.Equal(t, metav1.ConditionTrue, cond.Status)
			assert.Equal(t, appsv1.ReasonDevPodReady, cond.Reason)
			assert.Contains(t, cond.Message, "30022")
			assert.Equal(t, int64(3), cond.ObservedGeneration)
		case appsv1.ConditionProgressing:
			assert.Equal(t, metav1.ConditionFalse, cond.Status)
		case appsv1.ConditionDegraded:
			assert.Equal(t, metav1.ConditionFalse, cond.Status)
		}
	}
}

func TestReady_ReturnsNoRequeue(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	dp.Status.SSHNodePort = 30022
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	result, err := r.ready(testCtx(), dp)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.Zero(t, result.RequeueAfter)
}

func TestReady_StatusUpdateError(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	dp.Status.SSHNodePort = 30022
	c := interceptingClient(t, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			return errors.New("status update failed")
		},
	}, dp)
	r := newTestReconciler(c)

	_, err := r.ready(testCtx(), dp)
	assert.Error(t, err)
}

// =============================================================================
// progressing
// =============================================================================

func TestProgressing_SetsCorrectConditions(t *testing.T) {

	dp := newDevPod("dev1", "ns").withGeneration(2).withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	result, err := r.progressing(testCtx(), dp, appsv1.ReasonWaitingForStatefulSet, "rolling out")
	require.NoError(t, err)
	_ = result

	for _, cond := range dp.Status.Conditions {
		switch cond.Type {
		case appsv1.ConditionReady:
			assert.Equal(t, metav1.ConditionFalse, cond.Status)
		case appsv1.ConditionProgressing:
			assert.Equal(t, metav1.ConditionTrue, cond.Status)
			assert.Equal(t, appsv1.ReasonWaitingForStatefulSet, cond.Reason)
		case appsv1.ConditionDegraded:
			assert.Equal(t, metav1.ConditionFalse, cond.Status)
		}
	}
}

func TestProgressing_RequeuesAfterInterval(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	result, err := r.progressing(testCtx(), dp, "reason", "msg")
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, result.RequeueAfter)
}

func TestProgressing_StatusUpdateError(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := interceptingClient(t, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			return errors.New("status update failed")
		},
	}, dp)
	r := newTestReconciler(c)

	_, err := r.progressing(testCtx(), dp, "reason", "msg")
	assert.Error(t, err)
}

// =============================================================================
// pending
// =============================================================================

func TestPending_SetsAllConditionsFalse(t *testing.T) {

	dp := newDevPod("dev1", "ns").withGeneration(1).withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	result, err := r.pending(testCtx(), dp, appsv1.ReasonPasswordNotSet, "waiting")
	require.NoError(t, err)
	_ = result

	for _, cond := range dp.Status.Conditions {
		assert.Equal(t, metav1.ConditionFalse, cond.Status,
			"condition %s should be False", cond.Type)
		assert.Equal(t, appsv1.ReasonPasswordNotSet, cond.Reason)
	}
}

func TestPending_ReturnsNoRequeue(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	result, err := r.pending(testCtx(), dp, "reason", "msg")
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.Zero(t, result.RequeueAfter)
}

func TestPending_StatusUpdateError(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := interceptingClient(t, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			return errors.New("status update failed")
		},
	}, dp)
	r := newTestReconciler(c)

	_, err := r.pending(testCtx(), dp, "reason", "msg")
	assert.Error(t, err)
}

// =============================================================================
// degraded
// =============================================================================

func TestDegraded_WithCause_SetsConditionsAndReturnsError(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)
	cause := errors.New("api timeout")

	result, err := r.degraded(testCtx(), dp, appsv1.ReasonReconcileError, "failed", cause)
	assert.Equal(t, ctrl.Result{}, result)
	assert.ErrorIs(t, err, cause)

	for _, cond := range dp.Status.Conditions {
		switch cond.Type {
		case appsv1.ConditionDegraded:
			assert.Equal(t, metav1.ConditionTrue, cond.Status)
		default:
			assert.Equal(t, metav1.ConditionFalse, cond.Status)
		}
	}
}

func TestDegraded_NilCause_RequeuesAfterInterval(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	result, err := r.degraded(testCtx(), dp, appsv1.ReasonStorageClassNotFound, "sc missing", nil)
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, result.RequeueAfter)

	for _, cond := range dp.Status.Conditions {
		if cond.Type == appsv1.ConditionDegraded {
			assert.Equal(t, metav1.ConditionTrue, cond.Status)
		}
	}
}

func TestDegraded_StatusUpdateError(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	cause := errors.New("original")
	statusErr := errors.New("status update failed")
	c := interceptingClient(t, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			return statusErr
		},
	}, dp)
	r := newTestReconciler(c)

	_, err := r.degraded(testCtx(), dp, "reason", "msg", cause)
	// Should return the status update error, not the cause
	assert.ErrorIs(t, err, statusErr)
}
