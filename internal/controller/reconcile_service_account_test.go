package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// =============================================================================
// reconcileServiceAccount — Happy Path
// =============================================================================

func TestReconcileServiceAccount_CreatesWhenNotFound(t *testing.T) {

	dp := newDevPod().withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	err := r.reconcileServiceAccount(testCtx(), dp)
	require.NoError(t, err)

	// Verify created ServiceAccount
	var sa corev1.ServiceAccount
	require.NoError(t, c.Get(testCtx(), client.ObjectKeyFromObject(dp), &sa))
	assert.Equal(t, testName, sa.Name)
	assert.Equal(t, "ns", sa.Namespace)
	assert.NotNil(t, sa.AutomountServiceAccountToken)
	assert.False(t, *sa.AutomountServiceAccountToken)
	assert.Equal(t, standardLabels(dp), sa.Labels)

	// Verify owner reference
	require.NotEmpty(t, sa.OwnerReferences)
	assert.Equal(t, "DevPod", sa.OwnerReferences[0].Kind)
}

func TestReconcileServiceAccount_NoOpWhenCorrect(t *testing.T) {

	dp := newDevPod().withFinalizer().build()
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		AutomountServiceAccountToken: boolPtr(false),
	}
	c := fakeClientWith(t, dp, existingSA)
	r := newTestReconciler(c)

	err := r.reconcileServiceAccount(testCtx(), dp)
	assert.NoError(t, err)
}

func TestReconcileServiceAccount_FixesNilAutomount(t *testing.T) {

	dp := newDevPod().withFinalizer().build()
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		AutomountServiceAccountToken: nil,
	}
	c := fakeClientWith(t, dp, existingSA)
	r := newTestReconciler(c)

	err := r.reconcileServiceAccount(testCtx(), dp)
	require.NoError(t, err)

	// Verify update
	var sa corev1.ServiceAccount
	require.NoError(t, c.Get(testCtx(), client.ObjectKeyFromObject(dp), &sa))
	require.NotNil(t, sa.AutomountServiceAccountToken)
	assert.False(t, *sa.AutomountServiceAccountToken)
}

func TestReconcileServiceAccount_FixesTrueAutomount(t *testing.T) {

	dp := newDevPod().withFinalizer().build()
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		AutomountServiceAccountToken: boolPtr(true),
	}
	c := fakeClientWith(t, dp, existingSA)
	r := newTestReconciler(c)

	err := r.reconcileServiceAccount(testCtx(), dp)
	require.NoError(t, err)

	var sa corev1.ServiceAccount
	require.NoError(t, c.Get(testCtx(), client.ObjectKeyFromObject(dp), &sa))
	require.NotNil(t, sa.AutomountServiceAccountToken)
	assert.False(t, *sa.AutomountServiceAccountToken)
}

// =============================================================================
// reconcileServiceAccount — Error Propagation
// =============================================================================

func TestReconcileServiceAccount_GetError(t *testing.T) {

	dp := newDevPod().withFinalizer().build()
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.ServiceAccount); ok {
				return errors.New("api error")
			}
			return client.Get(ctx, key, obj, opts...)
		},
	}, dp)
	r := newTestReconciler(c)

	err := r.reconcileServiceAccount(testCtx(), dp)
	assert.Error(t, err)
}

func TestReconcileServiceAccount_CreateError(t *testing.T) {

	dp := newDevPod().withFinalizer().build()
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.ServiceAccount); ok {
				return apierrors.NewNotFound(schema.GroupResource{Resource: "serviceaccounts"}, key.Name)
			}
			return client.Get(ctx, key, obj, opts...)
		},
		Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*corev1.ServiceAccount); ok {
				return errors.New("create failed")
			}
			return client.Create(ctx, obj, opts...)
		},
	}, dp)
	r := newTestReconciler(c)

	err := r.reconcileServiceAccount(testCtx(), dp)
	assert.Error(t, err)
}

func TestReconcileServiceAccount_UpdateError(t *testing.T) {

	dp := newDevPod().withFinalizer().build()
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		AutomountServiceAccountToken: nil, // triggers update
	}
	c := interceptingClient(t, interceptor.Funcs{
		Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*corev1.ServiceAccount); ok {
				return errors.New("update failed")
			}
			return client.Update(ctx, obj, opts...)
		},
	}, dp, existingSA)
	r := newTestReconciler(c)

	err := r.reconcileServiceAccount(testCtx(), dp)
	assert.Error(t, err)
}

// =============================================================================
// reconcileServiceAccount — Mock / Dependency Interaction
// =============================================================================

func TestReconcileServiceAccount_SetsOwnerReference(t *testing.T) {

	dp := newDevPod().withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	err := r.reconcileServiceAccount(testCtx(), dp)
	require.NoError(t, err)

	var sa corev1.ServiceAccount
	require.NoError(t, c.Get(testCtx(), client.ObjectKeyFromObject(dp), &sa))
	require.NotEmpty(t, sa.OwnerReferences)
	assert.Equal(t, "DevPod", sa.OwnerReferences[0].Kind)
	assert.Equal(t, dp.Name, sa.OwnerReferences[0].Name)
	assert.True(t, *sa.OwnerReferences[0].Controller)
}
