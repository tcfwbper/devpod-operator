package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
	kappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// =============================================================================
// Reconcile — Happy Path
// =============================================================================

// scaffolded: awaiting full Reconcile implementation from internal/controller/devpod_controller.go
// scaffolded: awaiting reconcileSecret, reconcileServiceAccount, reconcileService, reconcileStatefulSet
// scaffolded: awaiting ready, progressing, pending, degraded from status.go

func TestReconcile_FullHappyPath(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withStorageClass("standard").
		build()
	secret := newPasswordSecret("dev1", "ns", "validpass1")
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "standard"},
	}
	// Pre-create the StatefulSet as ready
	sts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev1",
			Namespace: "ns",
		},
		Status: kappsv1.StatefulSetStatus{
			ReadyReplicas:      1,
			ObservedGeneration: 1,
		},
	}
	sts.Generation = 1

	c := fakeClientWith(t, dp, secret, storageClass, sts)
	r := newTestReconciler(c)

	result, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify status is Ready
	var updated appsv1.DevPod
	require.NoError(t, c.Get(testCtx(), types.NamespacedName{Name: "dev1", Namespace: "ns"}, &updated))
	for _, cond := range updated.Status.Conditions {
		if cond.Type == appsv1.ConditionReady {
			assert.Equal(t, metav1.ConditionTrue, cond.Status)
		}
	}
}

func TestReconcile_AddsFinalizer(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").build() // No finalizer
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	_, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	require.NoError(t, err)

	// Verify finalizer was added
	var updated appsv1.DevPod
	require.NoError(t, c.Get(testCtx(), types.NamespacedName{Name: "dev1", Namespace: "ns"}, &updated))
	assert.Contains(t, updated.Finalizers, "apps.devpod.com/finalizer")
}

func TestReconcile_ProgressingWhenStatefulSetNotReady(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withStorageClass("standard").
		build()
	secret := newPasswordSecret("dev1", "ns", "validpass1")
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "standard"},
	}
	sts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev1",
			Namespace: "ns",
		},
		Status: kappsv1.StatefulSetStatus{
			ReadyReplicas:      0,
			ObservedGeneration: 1,
		},
	}
	sts.Generation = 1

	c := fakeClientWith(t, dp, secret, storageClass, sts)
	r := newTestReconciler(c)

	result, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, result.RequeueAfter)
}

func TestReconcile_ProgressingWhenObservedGenerationLags(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withStorageClass("standard").
		build()
	secret := newPasswordSecret("dev1", "ns", "validpass1")
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "standard"},
	}
	sts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "dev1",
			Namespace:  "ns",
			Generation: 2,
		},
		Status: kappsv1.StatefulSetStatus{
			ReadyReplicas:      1,
			ObservedGeneration: 1,
		},
	}

	c := fakeClientWith(t, dp, secret, storageClass, sts)
	r := newTestReconciler(c)

	result, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, result.RequeueAfter)
}

// =============================================================================
// Reconcile — Status fields
// =============================================================================

func TestReconcile_WritesPasswordSecret(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").withFinalizer().withStorageClass("standard").build()
	secret := newPasswordSecret("dev1", "ns", "validpass1")
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "standard"},
	}

	c := fakeClientWith(t, dp, secret, storageClass)
	r := newTestReconciler(c)

	_, _ = r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})

	var updated appsv1.DevPod
	require.NoError(t, c.Get(testCtx(), types.NamespacedName{Name: "dev1", Namespace: "ns"}, &updated))
	assert.Equal(t, "dev1", updated.Status.PasswordSecret)
}

func TestReconcile_WritesSSHNodePort(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withStorageClass("standard").
		withNodePorts(appsv1.NodePortMapping{Src: 22, Dest: 30022}).
		build()
	secret := newPasswordSecret("dev1", "ns", "validpass1")
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "standard"},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev1",
			Namespace: "ns",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 22, NodePort: 30022},
			},
		},
	}

	c := fakeClientWith(t, dp, secret, storageClass, svc)
	r := newTestReconciler(c)

	_, _ = r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})

	var updated appsv1.DevPod
	require.NoError(t, c.Get(testCtx(), types.NamespacedName{Name: "dev1", Namespace: "ns"}, &updated))
	assert.Equal(t, int32(30022), updated.Status.SSHNodePort)
}

func TestReconcile_WritesReadyReplicas(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withStorageClass("standard").
		build()
	secret := newPasswordSecret("dev1", "ns", "validpass1")
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "standard"},
	}
	sts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev1",
			Namespace: "ns",
		},
		Status: kappsv1.StatefulSetStatus{
			ReadyReplicas:      1,
			ObservedGeneration: 1,
		},
	}
	sts.Generation = 1

	c := fakeClientWith(t, dp, secret, storageClass, sts)
	r := newTestReconciler(c)

	_, _ = r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})

	var updated appsv1.DevPod
	require.NoError(t, c.Get(testCtx(), types.NamespacedName{Name: "dev1", Namespace: "ns"}, &updated))
	assert.Equal(t, int32(1), updated.Status.ReadyReplicas)
}

// =============================================================================
// Reconcile — State Transitions
// =============================================================================

func TestReconcile_PendingWhenPasswordEmpty(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newPasswordSecret("dev1", "ns", "") // empty password
	c := fakeClientWith(t, dp, secret)
	r := newTestReconciler(c)

	result, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result) // no requeue

	var updated appsv1.DevPod
	require.NoError(t, c.Get(testCtx(), types.NamespacedName{Name: "dev1", Namespace: "ns"}, &updated))
	for _, cond := range updated.Status.Conditions {
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, appsv1.ReasonPasswordNotSet, cond.Reason)
	}
}

func TestReconcile_DegradedWhenPasswordInvalid(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	secret := newPasswordSecret("dev1", "ns", "short") // 5 chars < 8
	c := fakeClientWith(t, dp, secret)
	r := newTestReconciler(c)

	result, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, result.RequeueAfter)

	var updated appsv1.DevPod
	require.NoError(t, c.Get(testCtx(), types.NamespacedName{Name: "dev1", Namespace: "ns"}, &updated))
	for _, cond := range updated.Status.Conditions {
		if cond.Type == appsv1.ConditionDegraded {
			assert.Equal(t, metav1.ConditionTrue, cond.Status)
			assert.Equal(t, appsv1.ReasonInvalidPassword, cond.Reason)
		}
	}
}

func TestReconcile_DegradedWhenStorageClassNotFound(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withStorageClass("nonexistent").
		build()
	secret := newPasswordSecret("dev1", "ns", "validpass1")
	// No StorageClass created
	c := fakeClientWith(t, dp, secret)
	r := newTestReconciler(c)

	result, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, result.RequeueAfter)

	var updated appsv1.DevPod
	require.NoError(t, c.Get(testCtx(), types.NamespacedName{Name: "dev1", Namespace: "ns"}, &updated))
	for _, cond := range updated.Status.Conditions {
		if cond.Type == appsv1.ConditionDegraded {
			assert.Equal(t, metav1.ConditionTrue, cond.Status)
			assert.Equal(t, appsv1.ReasonStorageClassNotFound, cond.Reason)
		}
	}
}

// =============================================================================
// Reconcile — Error Propagation
// =============================================================================

func TestReconcile_NotFound(t *testing.T) {
	// This test can run since Reconcile already exists (stub returns success for not-found)
	// but the production behavior (checking for NotFound) is not yet implemented
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	c := fakeClientWith(t) // no DevPod
	r := newTestReconciler(c)

	result, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: "ns"},
	})
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestReconcile_SecretReconcileError(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").withFinalizer().build()
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Secret); ok {
				return errors.New("transient error")
			}
			return client.Get(ctx, key, obj, opts...)
		},
	}, dp)
	r := newTestReconciler(c)

	_, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	assert.Error(t, err)
}

func TestReconcile_ServiceAccountReconcileError(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withStorageClass("standard").
		build()
	secret := newPasswordSecret("dev1", "ns", "validpass1")
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "standard"},
	}
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.ServiceAccount); ok {
				return errors.New("sa error")
			}
			return client.Get(ctx, key, obj, opts...)
		},
	}, dp, secret, storageClass)
	r := newTestReconciler(c)

	_, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	assert.Error(t, err)
}

func TestReconcile_ServiceReconcileError(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withStorageClass("standard").
		build()
	secret := newPasswordSecret("dev1", "ns", "validpass1")
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "standard"},
	}
	sa := &corev1.ServiceAccount{
		ObjectMeta:                   metav1.ObjectMeta{Name: "dev1", Namespace: "ns"},
		AutomountServiceAccountToken: boolPtr(false),
	}
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Service); ok {
				return errors.New("svc error")
			}
			return client.Get(ctx, key, obj, opts...)
		},
	}, dp, secret, storageClass, sa)
	r := newTestReconciler(c)

	_, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	assert.Error(t, err)
}

func TestReconcile_StatefulSetReconcileError(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withStorageClass("standard").
		build()
	secret := newPasswordSecret("dev1", "ns", "validpass1")
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "standard"},
	}
	sa := &corev1.ServiceAccount{
		ObjectMeta:                   metav1.ObjectMeta{Name: "dev1", Namespace: "ns"},
		AutomountServiceAccountToken: boolPtr(false),
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "dev1", Namespace: "ns"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 22, NodePort: 30022}},
		},
	}
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*kappsv1.StatefulSet); ok {
				return errors.New("sts error")
			}
			return client.Get(ctx, key, obj, opts...)
		},
	}, dp, secret, storageClass, sa, svc)
	r := newTestReconciler(c)

	_, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	assert.Error(t, err)
}

func TestReconcile_StorageClassGetError(t *testing.T) {
	t.Skip("scaffolded: awaiting full Reconcile implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withStorageClass("standard").
		build()
	secret := newPasswordSecret("dev1", "ns", "validpass1")
	c := interceptingClient(t, interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*storagev1.StorageClass); ok {
				return apierrors.NewInternalError(errors.New("internal server error"))
			}
			return client.Get(ctx, key, obj, opts...)
		},
	}, dp, secret)
	r := newTestReconciler(c)

	_, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	assert.Error(t, err)
}

// =============================================================================
// Finalize
// =============================================================================

// scaffolded: awaiting finalize implementation from internal/controller/devpod_controller.go

func TestFinalize_DeletesPVCsWhenReclaimDelete(t *testing.T) {
	t.Skip("scaffolded: awaiting finalize implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withDeletionTimestamp().
		withReclaimPolicy(appsv1.PVCReclaimDelete).
		build()

	pvc1 := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-dev1-0",
			Namespace: "ns",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "devpod",
				"app.kubernetes.io/instance": "dev1",
			},
		},
	}
	pvc2 := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "docker-storage-dev1-0",
			Namespace: "ns",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "devpod",
				"app.kubernetes.io/instance": "dev1",
			},
		},
	}
	c := fakeClientWith(t, dp, pvc1, pvc2)
	r := newTestReconciler(c)

	result, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify PVCs are deleted
	var pvcList corev1.PersistentVolumeClaimList
	require.NoError(t, c.List(testCtx(), &pvcList, client.InNamespace("ns")))
	assert.Empty(t, pvcList.Items)

	// Verify finalizer removed
	var updated appsv1.DevPod
	err = c.Get(testCtx(), types.NamespacedName{Name: "dev1", Namespace: "ns"}, &updated)
	// Object may be deleted or finalizer removed
	if err == nil {
		assert.NotContains(t, updated.Finalizers, "apps.devpod.com/finalizer")
	}
}

func TestFinalize_RetainsPVCsWhenReclaimRetain(t *testing.T) {
	t.Skip("scaffolded: awaiting finalize implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withDeletionTimestamp().
		withReclaimPolicy(appsv1.PVCReclaimRetain).
		build()

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-dev1-0",
			Namespace: "ns",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "devpod",
				"app.kubernetes.io/instance": "dev1",
			},
		},
	}
	c := fakeClientWith(t, dp, pvc)
	r := newTestReconciler(c)

	_, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	require.NoError(t, err)

	// PVCs should still exist
	var pvcList corev1.PersistentVolumeClaimList
	require.NoError(t, c.List(testCtx(), &pvcList, client.InNamespace("ns")))
	assert.Len(t, pvcList.Items, 1)
}

func TestFinalize_RemovesFinalizer(t *testing.T) {
	t.Skip("scaffolded: awaiting finalize implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withDeletionTimestamp().
		withReclaimPolicy(appsv1.PVCReclaimRetain).
		build()
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	_, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	require.NoError(t, err)

	var updated appsv1.DevPod
	err = c.Get(testCtx(), types.NamespacedName{Name: "dev1", Namespace: "ns"}, &updated)
	if err == nil {
		assert.NotContains(t, updated.Finalizers, "apps.devpod.com/finalizer")
	}
}

func TestFinalize_NoFinalizerPresent(t *testing.T) {
	t.Skip("scaffolded: awaiting finalize implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withDeletionTimestamp().
		build() // no finalizer
	c := fakeClientWith(t, dp)
	r := newTestReconciler(c)

	result, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestFinalize_PVCDeleteNotFoundIgnored(t *testing.T) {
	t.Skip("scaffolded: awaiting finalize implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withDeletionTimestamp().
		withReclaimPolicy(appsv1.PVCReclaimDelete).
		build()

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-dev1-0",
			Namespace: "ns",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "devpod",
				"app.kubernetes.io/instance": "dev1",
			},
		},
	}
	c := interceptingClient(t, interceptor.Funcs{
		Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if _, ok := obj.(*corev1.PersistentVolumeClaim); ok {
				return apierrors.NewNotFound(schema.GroupResource{Resource: "persistentvolumeclaims"}, obj.GetName())
			}
			return client.Delete(ctx, obj, opts...)
		},
	}, dp, pvc)
	r := newTestReconciler(c)

	_, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	assert.NoError(t, err)
}

func TestFinalize_PVCListError(t *testing.T) {
	t.Skip("scaffolded: awaiting finalize implementation from devpod_controller.go")

	dp := newDevPod("dev1", "ns").
		withFinalizer().
		withDeletionTimestamp().
		withReclaimPolicy(appsv1.PVCReclaimDelete).
		build()
	c := interceptingClient(t, interceptor.Funcs{
		List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(*corev1.PersistentVolumeClaimList); ok {
				return errors.New("list error")
			}
			return client.List(ctx, list, opts...)
		},
	}, dp)
	r := newTestReconciler(c)

	_, err := r.Reconcile(testCtx(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev1", Namespace: "ns"},
	})
	assert.Error(t, err)
}

// =============================================================================
// SetupWithManager
// =============================================================================

func TestSetupWithManager_Registers(t *testing.T) {
	t.Skip("scaffolded: awaiting envtest or manager mock — integration-level test")

	// This test requires a real or fake Manager and is typically verified
	// in integration tests. The controller should be named "devpod".
}
