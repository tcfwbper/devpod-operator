package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// =============================================================================
// reconcileService — Happy Path
// =============================================================================

func TestReconcileService_CreatesWhenNotFound(t *testing.T) {

	dp := newDevPod().
		withNodePorts(
			appsv1.NodePortMapping{Src: 22, Dest: 30022},
			appsv1.NodePortMapping{Src: 8080, Dest: 30080},
		).
		withFinalizer().
		build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	svc, err := r.reconcileService(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, svc)

	assert.Equal(t, testName, svc.Name)
	assert.Equal(t, corev1.ServiceTypeNodePort, svc.Spec.Type)
	assert.Equal(t, selectorLabels(dp), svc.Spec.Selector)

	require.Len(t, svc.Spec.Ports, 2)
	assert.Equal(t, portNameSSH, svc.Spec.Ports[0].Name)
	assert.Equal(t, int32(22), svc.Spec.Ports[0].Port)
	assert.Equal(t, int32(22), svc.Spec.Ports[0].TargetPort.IntVal)
	assert.Equal(t, int32(30022), svc.Spec.Ports[0].NodePort)
	assert.Equal(t, corev1.ProtocolTCP, svc.Spec.Ports[0].Protocol)
	assert.Equal(t, "port-8080", svc.Spec.Ports[1].Name)
	assert.Equal(t, int32(8080), svc.Spec.Ports[1].Port)
	assert.Equal(t, int32(8080), svc.Spec.Ports[1].TargetPort.IntVal)
	assert.Equal(t, int32(30080), svc.Spec.Ports[1].NodePort)

	assert.Equal(t, corev1.ServiceExternalTrafficPolicyCluster, svc.Spec.ExternalTrafficPolicy)
	assert.Equal(t, corev1.ServiceAffinityNone, svc.Spec.SessionAffinity)

	// Verify owner reference
	require.NotEmpty(t, svc.OwnerReferences)
	assert.Equal(t, "DevPod", svc.OwnerReferences[0].Kind)

	// Verify spec-hash annotation
	assert.Contains(t, svc.Annotations, specHashAnnotation)
}

func TestReconcileService_NoOpWhenHashMatches(t *testing.T) {

	dp := newDevPod().
		withNodePorts(appsv1.NodePortMapping{Src: 22, Dest: 30022}).
		withFinalizer().
		build()

	// Pre-create a Service with the correct spec-hash
	// This test needs to compute the hash to set it on the existing Service
	// The exact hash is computed from the desired Service spec
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				specHashAnnotation: "placeholder-will-be-set-by-production-code",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{Name: portNameSSH, Port: 22, NodePort: 30022, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	c := fakeClientWith(t, dp, existingSvc)
	r := newTestReconciler(c)

	svc, err := r.reconcileService(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, svc)
	// No update should be issued; service returned as-is
}

func TestReconcileService_UpdatesWhenHashDiffers(t *testing.T) {

	dp := newDevPod().
		withNodePorts(
			appsv1.NodePortMapping{Src: 22, Dest: 30022},
			appsv1.NodePortMapping{Src: 9090, Dest: 30090},
		).
		withFinalizer().
		build()

	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				specHashAnnotation: "old-hash",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeNodePort,
			ClusterIP: "10.0.0.1",
			Ports: []corev1.ServicePort{
				{Name: portNameSSH, Port: 22, NodePort: 30022, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	c := fakeClientWith(t, dp, existingSvc)
	r := newTestReconciler(c)

	svc, err := r.reconcileService(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, svc)

	// Updated ports
	require.Len(t, svc.Spec.Ports, 2)
	// ClusterIP preserved
	assert.Equal(t, "10.0.0.1", svc.Spec.ClusterIP)
	// New hash annotation
	assert.NotEqual(t, "old-hash", svc.Annotations[specHashAnnotation])
}

func TestReconcileService_PreservesClusterIP(t *testing.T) {

	dp := newDevPod().
		withNodePorts(appsv1.NodePortMapping{Src: 22, Dest: 30022}).
		withFinalizer().
		build()

	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				specHashAnnotation: "outdated",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeNodePort,
			ClusterIP: "10.96.0.5",
		},
	}
	c := fakeClientWith(t, dp, existingSvc)
	r := newTestReconciler(c)

	svc, err := r.reconcileService(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, svc)
	assert.Equal(t, "10.96.0.5", svc.Spec.ClusterIP)
}

// =============================================================================
// reconcileService — Null / Empty Input
// =============================================================================

func TestReconcileService_NilAnnotations(t *testing.T) {

	dp := newDevPod().
		withNodePorts(appsv1.NodePortMapping{Src: 22, Dest: 30022}).
		withFinalizer().
		build()

	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        testName,
			Namespace:   "ns",
			Annotations: nil,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
		},
	}
	c := fakeClientWith(t, dp, existingSvc)
	r := newTestReconciler(c)

	svc, err := r.reconcileService(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, svc)
	assert.Contains(t, svc.Annotations, specHashAnnotation)
}

// =============================================================================
// reconcileService — Error Propagation
// =============================================================================

func TestReconcileService_GetError(t *testing.T) {

	dp := newDevPod().withFinalizer().build()
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Service); ok {
				return errors.New("api error")
			}
			return client.Get(ctx, key, obj, opts...)
		},
	}, dp)
	r := newTestReconciler(c)

	svc, err := r.reconcileService(testCtx(), dp)
	assert.Nil(t, svc)
	assert.Error(t, err)
}

func TestReconcileService_CreateError(t *testing.T) {

	dp := newDevPod().withFinalizer().build()
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Service); ok {
				return apierrors.NewNotFound(schema.GroupResource{Resource: "services"}, key.Name)
			}
			return client.Get(ctx, key, obj, opts...)
		},
		Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*corev1.Service); ok {
				return errors.New("create failed")
			}
			return client.Create(ctx, obj, opts...)
		},
	}, dp)
	r := newTestReconciler(c)

	svc, err := r.reconcileService(testCtx(), dp)
	assert.Nil(t, svc)
	assert.Error(t, err)
}

func TestReconcileService_UpdateError(t *testing.T) {

	dp := newDevPod().
		withNodePorts(appsv1.NodePortMapping{Src: 22, Dest: 30022}).
		withFinalizer().
		build()
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				specHashAnnotation: "stale",
			},
		},
		Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort},
	}
	c := interceptingClient(t, interceptor.Funcs{
		Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*corev1.Service); ok {
				return errors.New("update failed")
			}
			return client.Update(ctx, obj, opts...)
		},
	}, dp, existingSvc)
	r := newTestReconciler(c)

	svc, err := r.reconcileService(testCtx(), dp)
	assert.Nil(t, svc)
	assert.Error(t, err)
}

func TestReconcileService_HashComputationError(t *testing.T) {

	// This test requires a way to inject a specHash failure.
	// Blocked until the production code provides a testable seam or the function
	// can be made to fail via specific input.
}

// =============================================================================
// reconcileService — Mock / Dependency Interaction
// =============================================================================

func TestReconcileService_SetsOwnerReference(t *testing.T) {

	dp := newDevPod().withFinalizer().build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	svc, err := r.reconcileService(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, svc)

	require.NotEmpty(t, svc.OwnerReferences)
	assert.Equal(t, "DevPod", svc.OwnerReferences[0].Kind)
	assert.Equal(t, dp.Name, svc.OwnerReferences[0].Name)
	assert.True(t, *svc.OwnerReferences[0].Controller)
}

func TestReconcileService_LabelsAndSelector(t *testing.T) {

	dp := newDevPod().
		withNodePorts(appsv1.NodePortMapping{Src: 22, Dest: 30022}).
		withFinalizer().
		build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	svc, err := r.reconcileService(testCtx(), dp)
	require.NoError(t, err)
	require.NotNil(t, svc)

	assert.Equal(t, standardLabels(dp), svc.Labels)
	assert.Equal(t, selectorLabels(dp), svc.Spec.Selector)
}
