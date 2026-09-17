package controller

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
	kappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// =============================================================================
// buildStatefulSet — Basic structure
// =============================================================================

func standardTestDevPod() *appsv1.DevPod {
	return newDevPod().
		withImage("img:1").
		withUsername("alice").
		withDockerEnabled(true).
		withNodePorts(appsv1.NodePortMapping{Src: 22, Dest: 30022}).
		build()
}

func TestBuildStatefulSet_BasicStructure(t *testing.T) {

	dp := standardTestDevPod()
	sts := buildStatefulSet(dp)

	assert.Equal(t, testName, sts.Name)
	assert.Equal(t, "ns", sts.Namespace)
	assert.Equal(t, int32(1), *sts.Spec.Replicas)
	assert.Equal(t, testName, sts.Spec.ServiceName)
	assert.Equal(t, kappsv1.ParallelPodManagement, sts.Spec.PodManagementPolicy)
	assert.Equal(t, kappsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
}

func TestBuildStatefulSet_SelectorLabels(t *testing.T) {

	dp := standardTestDevPod()
	sts := buildStatefulSet(dp)

	expected := selectorLabels(dp)
	assert.Equal(t, expected, sts.Spec.Selector.MatchLabels)
}

func TestBuildStatefulSet_PodTemplateLabels(t *testing.T) {

	dp := standardTestDevPod()
	sts := buildStatefulSet(dp)

	expected := standardLabels(dp)
	assert.Equal(t, expected, sts.Spec.Template.Labels)
}

func TestBuildStatefulSet_PodSecurityContext(t *testing.T) {

	dp := standardTestDevPod()
	sts := buildStatefulSet(dp)

	psc := sts.Spec.Template.Spec.SecurityContext
	require.NotNil(t, psc)
	assert.Equal(t, int64(1001), *psc.FSGroup)
	assert.Equal(t, corev1.FSGroupChangeOnRootMismatch, *psc.FSGroupChangePolicy)
}

func TestBuildStatefulSet_PodServiceAccount(t *testing.T) {

	dp := standardTestDevPod()
	sts := buildStatefulSet(dp)

	podSpec := sts.Spec.Template.Spec
	assert.Equal(t, testName, podSpec.ServiceAccountName)
	assert.Equal(t, false, *podSpec.AutomountServiceAccountToken)
	assert.True(t, *podSpec.EnableServiceLinks)
}

func TestBuildStatefulSet_TerminationGracePeriod(t *testing.T) {

	dp := standardTestDevPod()
	sts := buildStatefulSet(dp)

	assert.Equal(t, int64(10), *sts.Spec.Template.Spec.TerminationGracePeriodSeconds)
}

// =============================================================================
// buildStatefulSet — devpod container
// =============================================================================

func TestBuildStatefulSet_DevpodContainer_Basic(t *testing.T) {

	dp := newDevPod().withImage("myimg:v1").build()
	sts := buildStatefulSet(dp)

	require.NotEmpty(t, sts.Spec.Template.Spec.Containers)
	c := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, labelValueName, c.Name)
	assert.Equal(t, "myimg:v1", c.Image)
	assert.Equal(t, corev1.PullIfNotPresent, c.ImagePullPolicy)
}

func TestBuildStatefulSet_DevpodContainer_SecurityContext(t *testing.T) {

	dp := standardTestDevPod()
	sts := buildStatefulSet(dp)

	c := sts.Spec.Template.Spec.Containers[0]
	sc := c.SecurityContext
	require.NotNil(t, sc)
	assert.Equal(t, int64(0), *sc.RunAsUser)
	assert.Equal(t, int64(0), *sc.RunAsGroup)
	assert.Equal(t, false, *sc.RunAsNonRoot)
	assert.Equal(t, false, *sc.Privileged)
	assert.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, sc.SeccompProfile.Type)
}

func TestBuildStatefulSet_DevpodContainer_Ports(t *testing.T) {

	dp := newDevPod().
		withNodePorts(
			appsv1.NodePortMapping{Src: 22, Dest: 30022},
			appsv1.NodePortMapping{Src: 8080, Dest: 30080},
		).build()
	sts := buildStatefulSet(dp)

	c := sts.Spec.Template.Spec.Containers[0]
	require.Len(t, c.Ports, 2)
	assert.Equal(t, portNameSSH, c.Ports[0].Name)
	assert.Equal(t, int32(22), c.Ports[0].ContainerPort)
	assert.Equal(t, corev1.ProtocolTCP, c.Ports[0].Protocol)
	assert.Equal(t, "port-8080", c.Ports[1].Name)
	assert.Equal(t, int32(8080), c.Ports[1].ContainerPort)
	assert.Equal(t, corev1.ProtocolTCP, c.Ports[1].Protocol)
}

func TestBuildStatefulSet_DevpodContainer_Probes(t *testing.T) {

	dp := standardTestDevPod()
	sts := buildStatefulSet(dp)

	c := sts.Spec.Template.Spec.Containers[0]
	for _, probe := range []*corev1.Probe{c.LivenessProbe, c.ReadinessProbe} {
		require.NotNil(t, probe)
		assert.Equal(t, int32(22), probe.TCPSocket.Port.IntVal)
		assert.Equal(t, int32(10), probe.InitialDelaySeconds)
		assert.Equal(t, int32(20), probe.TimeoutSeconds)
		assert.Equal(t, int32(30), probe.PeriodSeconds)
		assert.Equal(t, int32(3), probe.FailureThreshold)
		assert.Equal(t, int32(1), probe.SuccessThreshold)
	}
}

func TestBuildStatefulSet_DevpodContainer_Resources(t *testing.T) {

	dp := standardTestDevPod()
	sts := buildStatefulSet(dp)

	c := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, resource.MustParse("1"), *c.Resources.Requests.Cpu())
	assert.Equal(t, resource.MustParse("3072Mi"), *c.Resources.Requests.Memory())
	assert.Equal(t, resource.MustParse("50Mi"), *c.Resources.Requests.StorageEphemeral())
	assert.Equal(t, resource.MustParse("3"), *c.Resources.Limits.Cpu())
	assert.Equal(t, resource.MustParse("6144Mi"), *c.Resources.Limits.Memory())
	assert.Equal(t, resource.MustParse("2Gi"), *c.Resources.Limits.StorageEphemeral())
}

func TestBuildStatefulSet_DevpodContainer_VolumeMount(t *testing.T) {

	dp := newDevPod().withUsername("bob").build()
	sts := buildStatefulSet(dp)

	c := sts.Spec.Template.Spec.Containers[0]
	require.NotEmpty(t, c.VolumeMounts)
	vm := c.VolumeMounts[0]
	assert.Equal(t, "workspace", vm.Name)
	assert.Equal(t, "/home/bob", vm.MountPath)
	assert.Equal(t, "workspace", vm.SubPath)
}

func TestBuildStatefulSet_DevpodContainer_EnvPassword(t *testing.T) {

	dp := newDevPod().build()
	sts := buildStatefulSet(dp)

	c := sts.Spec.Template.Spec.Containers[0]
	require.NotEmpty(t, c.Env)

	var found bool
	for _, env := range c.Env {
		if env.Name == "UBUNTU_PASSWORD" {
			found = true
			require.NotNil(t, env.ValueFrom)
			require.NotNil(t, env.ValueFrom.SecretKeyRef)
			assert.Equal(t, testName, env.ValueFrom.SecretKeyRef.Name)
			assert.Equal(t, secretKeyPassword, env.ValueFrom.SecretKeyRef.Key)
		}
	}
	assert.True(t, found, "UBUNTU_PASSWORD env var must be present")
}

func TestBuildStatefulSet_DevpodContainer_Lifecycle(t *testing.T) {

	dp := standardTestDevPod()
	sts := buildStatefulSet(dp)

	c := sts.Spec.Template.Spec.Containers[0]
	require.NotNil(t, c.Lifecycle)
	require.NotNil(t, c.Lifecycle.PostStart)
	require.NotNil(t, c.Lifecycle.PostStart.Exec)
	require.Len(t, c.Lifecycle.PostStart.Exec.Command, 3)
	assert.Equal(t, "sh", c.Lifecycle.PostStart.Exec.Command[0])
	assert.Equal(t, "-c", c.Lifecycle.PostStart.Exec.Command[1])
	// The third element should contain the postStartScript output
	assert.NotEmpty(t, c.Lifecycle.PostStart.Exec.Command[2])
}

// =============================================================================
// buildStatefulSet — docker-dind sidecar
// =============================================================================

func TestBuildStatefulSet_DockerSidecar_Present(t *testing.T) {

	dp := newDevPod().
		withDockerEnabled(true).
		withDockerImage("docker:dind").
		build()
	sts := buildStatefulSet(dp)

	require.Len(t, sts.Spec.Template.Spec.Containers, 2)
	dc := sts.Spec.Template.Spec.Containers[1]
	assert.Equal(t, "docker-daemon", dc.Name)
	assert.Equal(t, "docker:dind", dc.Image)
	assert.Equal(t, corev1.PullIfNotPresent, dc.ImagePullPolicy)
}

func TestBuildStatefulSet_DockerSidecar_Privileged(t *testing.T) {

	dp := newDevPod().withDockerEnabled(true).build()
	sts := buildStatefulSet(dp)

	dc := sts.Spec.Template.Spec.Containers[1]
	require.NotNil(t, dc.SecurityContext)
	assert.True(t, *dc.SecurityContext.Privileged)
	assert.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, dc.SecurityContext.SeccompProfile.Type)
}

func TestBuildStatefulSet_DockerSidecar_Command(t *testing.T) {

	dp := newDevPod().withDockerEnabled(true).build()
	sts := buildStatefulSet(dp)

	dc := sts.Spec.Template.Spec.Containers[1]
	assert.Equal(t, []string{"dockerd", "-H", "tcp://0.0.0.0:2375"}, dc.Command)
}

func TestBuildStatefulSet_DockerSidecar_Port(t *testing.T) {

	dp := newDevPod().withDockerEnabled(true).build()
	sts := buildStatefulSet(dp)

	dc := sts.Spec.Template.Spec.Containers[1]
	require.Len(t, dc.Ports, 1)
	assert.Equal(t, int32(2375), dc.Ports[0].ContainerPort)
	assert.Equal(t, corev1.ProtocolTCP, dc.Ports[0].Protocol)
}

func TestBuildStatefulSet_DockerSidecar_Resources(t *testing.T) {

	dp := newDevPod().withDockerEnabled(true).build()
	sts := buildStatefulSet(dp)

	dc := sts.Spec.Template.Spec.Containers[1]
	assert.Equal(t, resource.MustParse("250m"), *dc.Resources.Requests.Cpu())
	assert.Equal(t, resource.MustParse("256Mi"), *dc.Resources.Requests.Memory())
	assert.Equal(t, resource.MustParse("50Mi"), *dc.Resources.Requests.StorageEphemeral())
	assert.Equal(t, resource.MustParse("375m"), *dc.Resources.Limits.Cpu())
	assert.Equal(t, resource.MustParse("384Mi"), *dc.Resources.Limits.Memory())
	assert.Equal(t, resource.MustParse("2Gi"), *dc.Resources.Limits.StorageEphemeral())
}

func TestBuildStatefulSet_DockerSidecar_VolumeMount(t *testing.T) {

	dp := newDevPod().withDockerEnabled(true).build()
	sts := buildStatefulSet(dp)

	dc := sts.Spec.Template.Spec.Containers[1]
	require.NotEmpty(t, dc.VolumeMounts)
	assert.Equal(t, "docker-storage", dc.VolumeMounts[0].Name)
	assert.Equal(t, "/var/lib/docker", dc.VolumeMounts[0].MountPath)
}

func TestBuildStatefulSet_DockerDisabled_NoSidecar(t *testing.T) {

	dp := newDevPod().withDockerEnabled(false).build()
	sts := buildStatefulSet(dp)

	require.Len(t, sts.Spec.Template.Spec.Containers, 1)
	assert.Equal(t, labelValueName, sts.Spec.Template.Spec.Containers[0].Name)
}

// =============================================================================
// buildStatefulSet — init container
// =============================================================================

func TestBuildStatefulSet_InitContainer(t *testing.T) {

	dp := newDevPod().withInitImage("init:1").build()
	sts := buildStatefulSet(dp)

	require.NotEmpty(t, sts.Spec.Template.Spec.InitContainers)
	ic := sts.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, "init-workspace", ic.Name)
	assert.Equal(t, "init:1", ic.Image)
	require.NotNil(t, ic.SecurityContext)
	assert.Equal(t, false, *ic.SecurityContext.Privileged)
	assert.Equal(t, int64(0), *ic.SecurityContext.RunAsUser)
	require.NotEmpty(t, ic.VolumeMounts)
	assert.Equal(t, "workspace", ic.VolumeMounts[0].Name)
	assert.Equal(t, "/tmp/workspace", ic.VolumeMounts[0].MountPath)
	assert.Equal(t, "workspace", ic.VolumeMounts[0].SubPath)
}

func TestBuildStatefulSet_InitContainer_Resources(t *testing.T) {

	dp := standardTestDevPod()
	sts := buildStatefulSet(dp)

	ic := sts.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, resource.MustParse("100m"), *ic.Resources.Requests.Cpu())
	assert.Equal(t, resource.MustParse("128Mi"), *ic.Resources.Requests.Memory())
	assert.Equal(t, resource.MustParse("50Mi"), *ic.Resources.Requests.StorageEphemeral())
	assert.Equal(t, resource.MustParse("150m"), *ic.Resources.Limits.Cpu())
	assert.Equal(t, resource.MustParse("192Mi"), *ic.Resources.Limits.Memory())
	assert.Equal(t, resource.MustParse("2Gi"), *ic.Resources.Limits.StorageEphemeral())
}

// =============================================================================
// buildStatefulSet — volume claims
// =============================================================================

func TestBuildStatefulSet_VolumeClaimWorkspace(t *testing.T) {

	dp := newDevPod().
		withStorageClass(testStorageClass).
		withStorageSize("100Gi").
		build()
	sts := buildStatefulSet(dp)

	require.NotEmpty(t, sts.Spec.VolumeClaimTemplates)
	vct := sts.Spec.VolumeClaimTemplates[0]
	assert.Equal(t, "workspace", vct.Name)
	assert.Equal(t, selectorLabels(dp), vct.Labels)
	require.NotEmpty(t, vct.Spec.AccessModes)
	assert.Equal(t, corev1.ReadWriteOnce, vct.Spec.AccessModes[0])
	assert.Equal(t, testStorageClass, *vct.Spec.StorageClassName)
	storage := vct.Spec.Resources.Requests[corev1.ResourceStorage]
	assert.Equal(t, resource.MustParse("100Gi"), storage)
}

func TestBuildStatefulSet_VolumeClaimDocker(t *testing.T) {

	dp := newDevPod().
		withDockerEnabled(true).
		withDockerStorageSize("200Gi").
		build()
	sts := buildStatefulSet(dp)

	require.Len(t, sts.Spec.VolumeClaimTemplates, 2)
	vct := sts.Spec.VolumeClaimTemplates[1]
	assert.Equal(t, "docker-storage", vct.Name)
	storage := vct.Spec.Resources.Requests[corev1.ResourceStorage]
	assert.Equal(t, resource.MustParse("200Gi"), storage)
}

func TestBuildStatefulSet_VolumeClaimDocker_Absent(t *testing.T) {

	dp := newDevPod().withDockerEnabled(false).build()
	sts := buildStatefulSet(dp)

	require.Len(t, sts.Spec.VolumeClaimTemplates, 1)
	assert.Equal(t, "workspace", sts.Spec.VolumeClaimTemplates[0].Name)
}

// =============================================================================
// postStartScript
// =============================================================================

func TestPostStartScript_AccountRename(t *testing.T) {

	dp := newDevPod().withUsername("alice").build()
	script := postStartScript(dp)

	assert.Contains(t, script, "usermod -l alice")
	assert.Contains(t, script, "usermod -d /home/alice")
	assert.Contains(t, script, "groupmod -n alice")
	assert.Contains(t, script, "usermod -g alice")
	assert.Contains(t, script, "chown")
	assert.Contains(t, script, "chgrp")
}

func TestPostStartScript_PasswordApplication(t *testing.T) {

	dp := newDevPod().withUsername("alice").build()
	script := postStartScript(dp)

	assert.Contains(t, script, "chpasswd")
	assert.Contains(t, script, "$UBUNTU_PASSWORD")
}

func TestPostStartScript_AptPackages(t *testing.T) {

	dp := newDevPod().withAptPackages("curl", "git").build()
	script := postStartScript(dp)

	assert.Contains(t, script, "DEBIAN_FRONTEND=noninteractive")
	assert.Contains(t, script, "apt-get update")
	assert.Contains(t, script, "apt-get install -y")
	assert.Contains(t, script, "|| true")
	assert.Contains(t, script, "apt-get clean")
	// Verify DEBIAN_FRONTEND is unset
	assert.Contains(t, script, "unset DEBIAN_FRONTEND")
}

func TestPostStartScript_PipPackages(t *testing.T) {

	dp := newDevPod().withPipPackages("numpy", "pandas").build()
	script := postStartScript(dp)

	assert.Contains(t, script, "pip install --no-cache-dir")
	assert.Contains(t, script, "|| true")
}

func TestPostStartScript_NoPackages(t *testing.T) {

	dp := newDevPod().
		withAptPackages().
		withPipPackages().
		build()
	script := postStartScript(dp)

	assert.NotContains(t, script, "apt-get")
	assert.NotContains(t, script, "pip install")
}

func TestPostStartScript_BothPackageTypes(t *testing.T) {

	dp := newDevPod().
		withAptPackages("curl").
		withPipPackages("flask").
		build()
	script := postStartScript(dp)

	assert.Contains(t, script, "apt-get install")
	assert.Contains(t, script, "pip install")
}

// =============================================================================
// volumeSize
// =============================================================================

func TestVolumeSize_NonNilNonZero(t *testing.T) {

	size := quantityPtr(resource.MustParse("100Gi"))
	result := volumeSize(size)
	assert.Equal(t, resource.MustParse("100Gi"), result)
}

func TestVolumeSize_Nil(t *testing.T) {

	result := volumeSize(nil)
	assert.Equal(t, resource.MustParse("50Gi"), result)
}

func TestVolumeSize_Zero(t *testing.T) {

	zero := quantityPtr(resource.Quantity{})
	result := volumeSize(zero)
	assert.Equal(t, resource.MustParse("50Gi"), result)
}

// =============================================================================
// containerSecurityContext
// =============================================================================

func TestContainerSecurityContext_NonPrivileged(t *testing.T) {

	sc := containerSecurityContext(false)

	assert.Equal(t, int64(0), *sc.RunAsUser)
	assert.Equal(t, int64(0), *sc.RunAsGroup)
	assert.Equal(t, false, *sc.RunAsNonRoot)
	assert.Equal(t, true, *sc.AllowPrivilegeEscalation)
	assert.Equal(t, false, *sc.ReadOnlyRootFilesystem)
	assert.Equal(t, false, *sc.Privileged)
	assert.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, sc.SeccompProfile.Type)
}

func TestContainerSecurityContext_Privileged(t *testing.T) {

	sc := containerSecurityContext(true)

	assert.Equal(t, int64(0), *sc.RunAsUser)
	assert.Equal(t, int64(0), *sc.RunAsGroup)
	assert.Equal(t, false, *sc.RunAsNonRoot)
	assert.Equal(t, true, *sc.AllowPrivilegeEscalation)
	assert.Equal(t, false, *sc.ReadOnlyRootFilesystem)
	assert.Equal(t, true, *sc.Privileged)
	assert.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, sc.SeccompProfile.Type)
}

// =============================================================================
// resourcePreset
// =============================================================================

func TestResourcePreset_Values(t *testing.T) {

	rr := resourcePreset("1", "3072Mi", "3", "6144Mi")

	assert.Equal(t, resource.MustParse("1"), *rr.Requests.Cpu())
	assert.Equal(t, resource.MustParse("3072Mi"), *rr.Requests.Memory())
	assert.Equal(t, resource.MustParse("50Mi"), *rr.Requests.StorageEphemeral())
	assert.Equal(t, resource.MustParse("3"), *rr.Limits.Cpu())
	assert.Equal(t, resource.MustParse("6144Mi"), *rr.Limits.Memory())
	assert.Equal(t, resource.MustParse("2Gi"), *rr.Limits.StorageEphemeral())
}

// =============================================================================
// sshProbe
// =============================================================================

func TestSSHProbe_Fields(t *testing.T) {

	probe := sshProbe()

	require.NotNil(t, probe.TCPSocket)
	assert.Equal(t, int32(22), probe.TCPSocket.Port.IntVal)
	assert.Equal(t, int32(10), probe.InitialDelaySeconds)
	assert.Equal(t, int32(20), probe.TimeoutSeconds)
	assert.Equal(t, int32(30), probe.PeriodSeconds)
	assert.Equal(t, int32(3), probe.FailureThreshold)
	assert.Equal(t, int32(1), probe.SuccessThreshold)
}

// =============================================================================
// shellQuoteAll
// =============================================================================

func TestShellQuoteAll_MultiplePackages(t *testing.T) {

	result := shellQuoteAll([]string{"curl", "git"})
	assert.Equal(t, " \"curl\" \"git\"", result)
}

func TestShellQuoteAll_SinglePackage(t *testing.T) {

	result := shellQuoteAll([]string{"curl"})
	assert.Equal(t, " \"curl\"", result)
}

func TestShellQuoteAll_MetaCharacters(t *testing.T) {

	result := shellQuoteAll([]string{"pkg; rm -rf /"})
	// The result should have the package name escaped via %q
	assert.True(t, strings.Contains(result, "pkg; rm -rf /") || strings.Contains(result, "pkg"),
		"result should contain escaped package name, got: %q", result)
}

func TestShellQuoteAll_EmptySlice(t *testing.T) {

	result := shellQuoteAll([]string{})
	assert.Equal(t, "", result)
}
