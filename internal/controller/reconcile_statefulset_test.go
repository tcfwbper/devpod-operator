package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kappsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// =============================================================================
// reconcileStatefulSet — Happy Path
// =============================================================================

// scaffolded: awaiting reconcileStatefulSet from internal/controller/reconcile_statefulset.go
// scaffolded: awaiting buildStatefulSet from internal/controller/build_statefulset.go

func TestReconcileStatefulSet_CreatesWhenNotFound(t *testing.T) {
	t.Skip("scaffolded: awaiting reconcileStatefulSet from reconcile_statefulset.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, sts)

	// Should match buildStatefulSet output
	desired := buildStatefulSet(dp)
	assert.Equal(t, desired.Spec.Template, sts.Spec.Template)

	// Verify spec-hash annotation
	assert.Contains(t, sts.Annotations, "devpod.com/spec-hash")

	// Verify owner reference
	require.NotEmpty(t, sts.OwnerReferences)
	assert.Equal(t, "DevPod", sts.OwnerReferences[0].Kind)
	assert.True(t, *sts.OwnerReferences[0].Controller)
}

func TestReconcileStatefulSet_NoOpWhenHashMatches(t *testing.T) {
	t.Skip("scaffolded: awaiting reconcileStatefulSet, specHash from reconcile_statefulset.go, helpers.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()

	existingSts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev1",
			Namespace: "ns",
			Annotations: map[string]string{
				"devpod.com/spec-hash": "placeholder-computed-by-production",
			},
		},
		Spec: kappsv1.StatefulSetSpec{
			Replicas: int32Ptr(1),
		},
	}
	c := fakeClientWith(t, dp, existingSts)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, sts)
	// Should return existing unchanged
}

func TestReconcileStatefulSet_UpdatesMutableFieldsOnHashMismatch(t *testing.T) {
	t.Skip("scaffolded: awaiting reconcileStatefulSet, buildStatefulSet from reconcile_statefulset.go, build_statefulset.go")

	dp := newDevPod("dev1", "ns").
		withImage("new-image:v2").
		withFinalizer().
		build()

	existingSts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev1",
			Namespace: "ns",
			Annotations: map[string]string{
				"devpod.com/spec-hash": "old-hash",
			},
		},
		Spec: kappsv1.StatefulSetSpec{
			Replicas:            int32Ptr(1),
			ServiceName:         "dev1",
			PodManagementPolicy: kappsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app.kubernetes.io/instance": "dev1"},
			},
		},
	}
	c := fakeClientWith(t, dp, existingSts)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, sts)

	// Mutable fields should be updated
	assert.NotEqual(t, "old-hash", sts.Annotations["devpod.com/spec-hash"])

	// Immutable fields should be preserved
	assert.Equal(t, "dev1", sts.Spec.ServiceName)
	assert.Equal(t, kappsv1.ParallelPodManagement, sts.Spec.PodManagementPolicy)
}

func TestReconcileStatefulSet_DoesNotModifyImmutableFields(t *testing.T) {
	t.Skip("scaffolded: awaiting reconcileStatefulSet from reconcile_statefulset.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()

	originalSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"original": "selector"},
	}
	existingSts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev1",
			Namespace: "ns",
			Annotations: map[string]string{
				"devpod.com/spec-hash": "different",
			},
		},
		Spec: kappsv1.StatefulSetSpec{
			Replicas:            int32Ptr(1),
			ServiceName:         "original-svc",
			PodManagementPolicy: kappsv1.OrderedReadyPodManagement,
			Selector:            originalSelector,
		},
	}
	c := fakeClientWith(t, dp, existingSts)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, sts)

	// Immutable fields must remain as they were
	assert.Equal(t, "original-svc", sts.Spec.ServiceName)
	assert.Equal(t, kappsv1.OrderedReadyPodManagement, sts.Spec.PodManagementPolicy)
	assert.Equal(t, originalSelector, sts.Spec.Selector)
}

// =============================================================================
// reconcileStatefulSet — Null / Empty Input
// =============================================================================

func TestReconcileStatefulSet_NilAnnotations(t *testing.T) {
	t.Skip("scaffolded: awaiting reconcileStatefulSet from reconcile_statefulset.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	existingSts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "dev1",
			Namespace:   "ns",
			Annotations: nil,
		},
		Spec: kappsv1.StatefulSetSpec{
			Replicas: int32Ptr(1),
		},
	}
	c := fakeClientWith(t, dp, existingSts)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, sts)
	assert.Contains(t, sts.Annotations, "devpod.com/spec-hash")
}

// =============================================================================
// reconcileStatefulSet — Error Propagation
// =============================================================================

func TestReconcileStatefulSet_GetError(t *testing.T) {
	t.Skip("scaffolded: awaiting reconcileStatefulSet from reconcile_statefulset.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*kappsv1.StatefulSet); ok {
				return errors.New("api error")
			}
			return client.Get(ctx, key, obj, opts...)
		},
	}, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp)
	assert.Nil(t, sts)
	assert.Error(t, err)
}

func TestReconcileStatefulSet_CreateError(t *testing.T) {
	t.Skip("scaffolded: awaiting reconcileStatefulSet from reconcile_statefulset.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*kappsv1.StatefulSet); ok {
				return apierrors.NewNotFound(schema.GroupResource{Resource: "statefulsets"}, key.Name)
			}
			return client.Get(ctx, key, obj, opts...)
		},
		Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*kappsv1.StatefulSet); ok {
				return errors.New("create failed")
			}
			return client.Create(ctx, obj, opts...)
		},
	}, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp)
	assert.Nil(t, sts)
	assert.Error(t, err)
}

func TestReconcileStatefulSet_UpdateError(t *testing.T) {
	t.Skip("scaffolded: awaiting reconcileStatefulSet from reconcile_statefulset.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	existingSts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev1",
			Namespace: "ns",
			Annotations: map[string]string{
				"devpod.com/spec-hash": "stale",
			},
		},
		Spec: kappsv1.StatefulSetSpec{Replicas: int32Ptr(1)},
	}
	c := interceptingClient(t, interceptor.Funcs{
		Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*kappsv1.StatefulSet); ok {
				return errors.New("update failed")
			}
			return client.Update(ctx, obj, opts...)
		},
	}, dp, existingSts)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp)
	assert.Nil(t, sts)
	assert.Error(t, err)
}

// =============================================================================
// reconcileStatefulSet — Mock / Dependency Interaction
// =============================================================================

func TestReconcileStatefulSet_SetsOwnerReference(t *testing.T) {
	t.Skip("scaffolded: awaiting reconcileStatefulSet from reconcile_statefulset.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, sts)

	require.NotEmpty(t, sts.OwnerReferences)
	assert.Equal(t, "DevPod", sts.OwnerReferences[0].Kind)
	assert.Equal(t, dp.Name, sts.OwnerReferences[0].Name)
	assert.True(t, *sts.OwnerReferences[0].Controller)
}

func TestReconcileStatefulSet_CallsBuildStatefulSet(t *testing.T) {
	t.Skip("scaffolded: awaiting reconcileStatefulSet, buildStatefulSet from reconcile_statefulset.go, build_statefulset.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, sts)

	desired := buildStatefulSet(dp)
	assert.Equal(t, desired.Spec.Template, sts.Spec.Template)
}
