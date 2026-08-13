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

// secretHashAnnotation is the annotation key for Secret content hash on pod template.
// Mirrors the constant that will be defined in production code.
const secretHashAnnotationKey = "apps.devpod.com/secret-hash"

// =============================================================================
// reconcileStatefulSet — Happy Path
// =============================================================================

func TestReconcileStatefulSet_CreatesWhenNotFound(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("testpass")})
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	require.NoError(t, err)
	require.NotNil(t, sts)

	// Should match buildStatefulSet output (with secret hash injected into pod template)
	desired := buildStatefulSet(dp)
	if desired.Spec.Template.ObjectMeta.Annotations == nil {
		desired.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	desired.Spec.Template.ObjectMeta.Annotations[secretHashAnnotationKey] = secretDataHash(secret.Data)
	assert.Equal(t, desired.Spec.Template, sts.Spec.Template)

	// Verify spec-hash annotation
	assert.Contains(t, sts.Annotations, "devpod.com/spec-hash")

	// Verify owner reference
	require.NotEmpty(t, sts.OwnerReferences)
	assert.Equal(t, "DevPod", sts.OwnerReferences[0].Kind)
	assert.True(t, *sts.OwnerReferences[0].Controller)

	// Verify secret-hash on pod template
	t.Run("secret_hash_on_pod_template", func(t *testing.T) {
		podAnnotations := sts.Spec.Template.ObjectMeta.Annotations
		assert.Contains(t, podAnnotations, secretHashAnnotationKey)
		assert.Equal(t, secretDataHash(secret.Data), podAnnotations[secretHashAnnotationKey])
	})
}

func TestReconcileStatefulSet_NoOpWhenHashMatches(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("testpass")})

	// Build the desired state to compute the expected hash
	desired := buildStatefulSet(dp)
	if desired.Spec.Template.ObjectMeta.Annotations == nil {
		desired.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	desired.Spec.Template.ObjectMeta.Annotations[secretHashAnnotationKey] = secretDataHash(secret.Data)
	expectedHash, _ := specHash(desired.Spec)

	existingSts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev1",
			Namespace: "ns",
			Annotations: map[string]string{
				"devpod.com/spec-hash": expectedHash,
			},
		},
		Spec: kappsv1.StatefulSetSpec{
			Replicas: int32Ptr(1),
		},
	}
	c := fakeClientWith(t, dp, existingSts)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	require.NoError(t, err)
	require.NotNil(t, sts)
	// Should return existing unchanged
}

func TestReconcileStatefulSet_UpdatesMutableFieldsOnHashMismatch(t *testing.T) {

	dp := newDevPod("dev1", "ns").
		withImage("new-image:v2").
		withFinalizer().
		build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("testpass")})

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

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	require.NoError(t, err)
	require.NotNil(t, sts)

	// Mutable fields should be updated
	assert.NotEqual(t, "old-hash", sts.Annotations["devpod.com/spec-hash"])

	// Immutable fields should be preserved
	assert.Equal(t, "dev1", sts.Spec.ServiceName)
	assert.Equal(t, kappsv1.ParallelPodManagement, sts.Spec.PodManagementPolicy)
}

func TestReconcileStatefulSet_DoesNotModifyImmutableFields(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("testpass")})

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

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
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

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("testpass")})
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

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	require.NoError(t, err)
	require.NotNil(t, sts)
	assert.Contains(t, sts.Annotations, "devpod.com/spec-hash")

	// Verify pod template has secret-hash
	t.Run("pod_template_has_secret_hash", func(t *testing.T) {
		podAnnotations := sts.Spec.Template.ObjectMeta.Annotations
		assert.Contains(t, podAnnotations, secretHashAnnotationKey)
	})
}

// =============================================================================
// reconcileStatefulSet — Error Propagation
// =============================================================================

func TestReconcileStatefulSet_GetError(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("testpass")})
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*kappsv1.StatefulSet); ok {
				return errors.New("api error")
			}
			return client.Get(ctx, key, obj, opts...)
		},
	}, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	assert.Nil(t, sts)
	assert.Error(t, err)
}

func TestReconcileStatefulSet_CreateError(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("testpass")})
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

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	assert.Nil(t, sts)
	assert.Error(t, err)
}

func TestReconcileStatefulSet_UpdateError(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("testpass")})
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

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	assert.Nil(t, sts)
	assert.Error(t, err)
}

// =============================================================================
// reconcileStatefulSet — Mock / Dependency Interaction
// =============================================================================

func TestReconcileStatefulSet_SetsOwnerReference(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("testpass")})
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	require.NoError(t, err)
	require.NotNil(t, sts)

	require.NotEmpty(t, sts.OwnerReferences)
	assert.Equal(t, "DevPod", sts.OwnerReferences[0].Kind)
	assert.Equal(t, dp.Name, sts.OwnerReferences[0].Name)
	assert.True(t, *sts.OwnerReferences[0].Controller)
}

func TestReconcileStatefulSet_CallsBuildStatefulSet(t *testing.T) {

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("testpass")})
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	require.NoError(t, err)
	require.NotNil(t, sts)

	// The actual template should match buildStatefulSet output with the addition of the secret hash annotation
	desired := buildStatefulSet(dp)
	if desired.Spec.Template.ObjectMeta.Annotations == nil {
		desired.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	desired.Spec.Template.ObjectMeta.Annotations[secretHashAnnotationKey] = secretDataHash(secret.Data)
	assert.Equal(t, desired.Spec.Template, sts.Spec.Template)
}

// =============================================================================
// reconcileStatefulSet — Secret hash (new tests, scaffolded)
// =============================================================================

func TestReconcileStatefulSet_SecretHashOnPodTemplate(t *testing.T) {
	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"key": []byte("val")})
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	require.NoError(t, err)
	require.NotNil(t, sts)

	// Pod template should have secret-hash annotation
	podAnnotations := sts.Spec.Template.ObjectMeta.Annotations
	assert.Contains(t, podAnnotations, secretHashAnnotationKey)
	assert.Equal(t, secretDataHash(secret.Data), podAnnotations[secretHashAnnotationKey])

	// StatefulSet metadata should NOT have secret-hash annotation
	assert.NotContains(t, sts.ObjectMeta.Annotations, secretHashAnnotationKey)
}

func TestReconcileStatefulSet_SecretHashInjectedBeforeSpecHash(t *testing.T) {
	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("testpass123")})
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	require.NoError(t, err)
	require.NotNil(t, sts)

	// Verify the spec-hash captures the secret hash:
	// Build the desired manually to verify
	desired := buildStatefulSet(dp)
	if desired.Spec.Template.ObjectMeta.Annotations == nil {
		desired.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	desired.Spec.Template.ObjectMeta.Annotations[secretHashAnnotationKey] = secretDataHash(secret.Data)
	expectedHash, err := specHash(desired.Spec)
	require.NoError(t, err)
	assert.Equal(t, expectedHash, sts.Annotations["devpod.com/spec-hash"])
}

func TestReconcileStatefulSet_SecretChangeTriggersUpdate(t *testing.T) {
	dp := newDevPod("dev1", "ns").withFinalizer().build()

	// Simulate existing StatefulSet created with old secret data
	oldSecretData := map[string][]byte{"pw": []byte("old")}
	oldSecretHash := secretDataHash(oldSecretData)

	// Build existing StatefulSet with old secret hash in pod template
	existingSts := buildStatefulSet(dp)
	if existingSts.Spec.Template.ObjectMeta.Annotations == nil {
		existingSts.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	existingSts.Spec.Template.ObjectMeta.Annotations[secretHashAnnotationKey] = oldSecretHash
	oldSpecHash, _ := specHash(existingSts.Spec)
	setAnnotation(existingSts, "devpod.com/spec-hash", oldSpecHash)

	// New secret with changed data
	newSecret := newSecretWithData("dev1", "ns", map[string][]byte{"pw": []byte("new")})

	c := fakeClientWith(t, dp, existingSts)
	r := newTestReconciler(c)
	sts, err := r.reconcileStatefulSet(testCtx(), dp, newSecret)
	require.NoError(t, err)
	require.NotNil(t, sts)

	// Pod template annotation should reflect new secret data
	assert.Equal(t, secretDataHash(newSecret.Data), sts.Spec.Template.ObjectMeta.Annotations[secretHashAnnotationKey])
	// Spec-hash should be updated
	assert.NotEqual(t, oldSpecHash, sts.Annotations["devpod.com/spec-hash"])
}

func TestReconcileStatefulSet_LegacyStatefulSetWithoutSecretHash(t *testing.T) {
	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("validpass1")})

	// Simulate a legacy StatefulSet that was created before the secret-hash feature.
	// It has a spec-hash but no secret-hash in pod template annotations.
	legacySts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev1",
			Namespace: "ns",
			Annotations: map[string]string{
				"devpod.com/spec-hash": "legacy-hash-without-secret",
			},
		},
		Spec: kappsv1.StatefulSetSpec{
			Replicas: int32Ptr(1),
		},
	}

	c := fakeClientWith(t, dp, legacySts)
	r := newTestReconciler(c)
	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	require.NoError(t, err)
	require.NotNil(t, sts)

	// Update should be issued; pod template now contains secret-hash
	podAnnotations := sts.Spec.Template.ObjectMeta.Annotations
	assert.Contains(t, podAnnotations, secretHashAnnotationKey)
	// Spec-hash should be refreshed
	assert.NotEqual(t, "legacy-hash-without-secret", sts.Annotations["devpod.com/spec-hash"])
}

func TestReconcileStatefulSet_CallsSecretDataHash(t *testing.T) {
	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newSecretWithData("dev1", "ns", map[string][]byte{"ubuntu-password": []byte("secret123")})
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	sts, err := r.reconcileStatefulSet(testCtx(), dp, secret)
	require.NoError(t, err)
	require.NotNil(t, sts)

	// The pod template annotation should equal the output of secretDataHash(secret.Data)
	expectedHash := secretDataHash(secret.Data)
	actualHash := sts.Spec.Template.ObjectMeta.Annotations[secretHashAnnotationKey]
	assert.Equal(t, expectedHash, actualHash)
}
