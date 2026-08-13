package controller

import (
	"context"
	"testing"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// testScheme returns a runtime.Scheme with all required types registered.
func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(appsv1.AddToScheme(s))
	return s
}

// testCtx returns a background context for unit tests.
func testCtx() context.Context {
	return context.Background()
}

// --------------------------------------------------------------------------
// DevPod fixture builder
// --------------------------------------------------------------------------

type devPodBuilder struct {
	dp *appsv1.DevPod
}

func newDevPod(name, namespace string) *devPodBuilder {
	return &devPodBuilder{
		dp: &appsv1.DevPod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps.devpod.com/v1",
				Kind:       "DevPod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
			Spec: appsv1.DevPodSpec{
				Image:              "docker.io/tcfwbper/dev-env:1.0.0",
				InitWorkspaceImage: "docker.io/tcfwbper/dev-env:1.0.0-init-workspace",
				Auth: appsv1.AuthSpec{
					Username: "testuser",
				},
				Persistence: appsv1.PersistenceSpec{
					StorageClass:  "standard",
					Size:          quantityPtr(resource.MustParse("50Gi")),
					ReclaimPolicy: appsv1.PVCReclaimRetain,
				},
				NodePorts: []appsv1.NodePortMapping{
					{Src: 22, Dest: 30022},
				},
				Docker: appsv1.DockerSpec{
					Enabled: boolPtr(true),
					Image:   "docker.io/docker:dind",
					Persistence: appsv1.DockerPersistenceSpec{
						Size: quantityPtr(resource.MustParse("50Gi")),
					},
				},
			},
		},
	}
}

func (b *devPodBuilder) withName(name string) *devPodBuilder {
	b.dp.Name = name
	return b
}

func (b *devPodBuilder) withNamespace(ns string) *devPodBuilder {
	b.dp.Namespace = ns
	return b
}

func (b *devPodBuilder) withUsername(username string) *devPodBuilder {
	b.dp.Spec.Auth.Username = username
	return b
}

func (b *devPodBuilder) withImage(image string) *devPodBuilder {
	b.dp.Spec.Image = image
	return b
}

func (b *devPodBuilder) withInitImage(image string) *devPodBuilder {
	b.dp.Spec.InitWorkspaceImage = image
	return b
}

func (b *devPodBuilder) withNodePorts(ports ...appsv1.NodePortMapping) *devPodBuilder {
	b.dp.Spec.NodePorts = ports
	return b
}

func (b *devPodBuilder) withDockerEnabled(enabled bool) *devPodBuilder {
	b.dp.Spec.Docker.Enabled = boolPtr(enabled)
	return b
}

func (b *devPodBuilder) withDockerImage(image string) *devPodBuilder {
	b.dp.Spec.Docker.Image = image
	return b
}

func (b *devPodBuilder) withStorageClass(sc string) *devPodBuilder {
	b.dp.Spec.Persistence.StorageClass = sc
	return b
}

func (b *devPodBuilder) withStorageSize(size string) *devPodBuilder {
	b.dp.Spec.Persistence.Size = quantityPtr(resource.MustParse(size))
	return b
}

func (b *devPodBuilder) withDockerStorageSize(size string) *devPodBuilder {
	b.dp.Spec.Docker.Persistence.Size = quantityPtr(resource.MustParse(size))
	return b
}

func (b *devPodBuilder) withReclaimPolicy(policy appsv1.PVCReclaimPolicy) *devPodBuilder {
	b.dp.Spec.Persistence.ReclaimPolicy = policy
	return b
}

func (b *devPodBuilder) withFinalizer() *devPodBuilder {
	b.dp.Finalizers = []string{"apps.devpod.com/finalizer"}
	return b
}

func (b *devPodBuilder) withDeletionTimestamp() *devPodBuilder {
	now := metav1.Now()
	b.dp.DeletionTimestamp = &now
	return b
}

func (b *devPodBuilder) withGeneration(gen int64) *devPodBuilder {
	b.dp.Generation = gen
	return b
}

func (b *devPodBuilder) withAptPackages(pkgs ...string) *devPodBuilder {
	b.dp.Spec.Packages.Apt = pkgs
	return b
}

func (b *devPodBuilder) withPipPackages(pkgs ...string) *devPodBuilder {
	b.dp.Spec.Packages.Pip = pkgs
	return b
}

func (b *devPodBuilder) build() *appsv1.DevPod {
	return b.dp.DeepCopy()
}

// --------------------------------------------------------------------------
// Secret fixture builder
// --------------------------------------------------------------------------

func newPasswordSecret(name, namespace, password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ubuntu-password": []byte(password),
		},
	}
}

// --------------------------------------------------------------------------
// Fake client helpers
// --------------------------------------------------------------------------

// fakeClientWith creates a fake client with the given initial objects.
func fakeClientWith(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := testScheme()
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&appsv1.DevPod{}).
		Build()
}

// interceptingClient wraps a fake client with interceptor funcs for simulating errors.
func interceptingClient(t *testing.T, funcs interceptor.Funcs, objs ...client.Object) client.Client {
	t.Helper()
	scheme := testScheme()
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&appsv1.DevPod{}).
		WithInterceptorFuncs(funcs).
		Build()
}

// newTestReconciler creates a DevPodReconciler wired to the given client.
func newTestReconciler(c client.Client) *DevPodReconciler {
	return &DevPodReconciler{
		Client: c,
		Scheme: testScheme(),
	}
}

// --------------------------------------------------------------------------
// Pointer helpers
// --------------------------------------------------------------------------

func boolPtr(b bool) *bool {
	return &b
}

func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func quantityPtr(q resource.Quantity) *resource.Quantity {
	return &q
}
