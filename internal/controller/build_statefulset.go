package controller

import (
	"fmt"
	"strings"

	appsv1 "github.com/tcfwbper/devpod-operator/api/v1"
	kappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const defaultVolumeSize = "50Gi"

// buildStatefulSet renders a complete StatefulSet manifest from a DevPod spec.
// It does not set owner references or spec-hash annotations — the caller handles those.
func buildStatefulSet(dp *appsv1.DevPod) *kappsv1.StatefulSet {
	username := dp.Spec.Auth.Username
	dockerEnabled := dp.Spec.DockerEnabled()

	// Container ports
	ports := make([]corev1.ContainerPort, 0, len(dp.Spec.NodePorts))
	for _, np := range dp.Spec.NodePorts {
		ports = append(ports, corev1.ContainerPort{
			Name:          portName(np.Src),
			ContainerPort: np.Src,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	// Workspace volume mount for main container
	workspaceMount := corev1.VolumeMount{
		Name:      "workspace",
		MountPath: fmt.Sprintf("/home/%s", username),
		SubPath:   "workspace",
	}

	// Main devpod container
	replicas := int32(1)
	probe := sshProbe()
	devpodContainer := corev1.Container{
		Name:            "devpod",
		Image:           dp.Spec.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		SecurityContext: containerSecurityContext(false),
		Ports:           ports,
		LivenessProbe:   probe,
		ReadinessProbe:  probe,
		Resources:       resourcePreset("1", "3072Mi", "3", "6144Mi"),
		VolumeMounts:    []corev1.VolumeMount{workspaceMount},
		Env: []corev1.EnvVar{
			{
				Name: "UBUNTU_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: dp.Name,
						},
						Key: "ubuntu-password",
					},
				},
			},
		},
		Lifecycle: &corev1.Lifecycle{
			PostStart: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"sh", "-c", postStartScript(dp)},
				},
			},
		},
	}

	// Containers
	containers := []corev1.Container{devpodContainer}

	// Docker sidecar
	if dockerEnabled {
		dockerContainer := corev1.Container{
			Name:            "docker-daemon",
			Image:           dp.Spec.Docker.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: containerSecurityContext(true),
			Command:         []string{"dockerd", "-H", "tcp://0.0.0.0:2375"},
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: 2375,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			Resources: resourcePreset("250m", "256Mi", "375m", "384Mi"),
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "docker-storage",
					MountPath: "/var/lib/docker",
				},
			},
		}
		containers = append(containers, dockerContainer)
	}

	// Init container
	initContainer := corev1.Container{
		Name:            "init-workspace",
		Image:           dp.Spec.InitWorkspaceImage,
		SecurityContext: containerSecurityContext(false),
		Resources:       resourcePreset("100m", "128Mi", "150m", "192Mi"),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/tmp/workspace",
				SubPath:   "workspace",
			},
		},
	}

	// Volume claim templates
	volumeClaimTemplates := []corev1.PersistentVolumeClaim{
		volumeClaim(dp, "workspace", dp.Spec.Persistence.Size),
	}
	if dockerEnabled {
		volumeClaimTemplates = append(volumeClaimTemplates,
			volumeClaim(dp, "docker-storage", dp.Spec.Docker.Persistence.Size))
	}

	// Pod security context
	fsGroup := int64(1001)
	fsGroupChangePolicy := corev1.FSGroupChangeOnRootMismatch
	terminationGrace := int64(10)
	automount := false
	enableServiceLinks := true

	sts := &kappsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dp.Name,
			Namespace: dp.Namespace,
		},
		Spec: kappsv1.StatefulSetSpec{
			Replicas:            &replicas,
			ServiceName:         dp.Name,
			PodManagementPolicy: kappsv1.ParallelPodManagement,
			UpdateStrategy: kappsv1.StatefulSetUpdateStrategy{
				Type: kappsv1.RollingUpdateStatefulSetStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels(dp),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: standardLabels(dp),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            dp.Name,
					AutomountServiceAccountToken:  &automount,
					EnableServiceLinks:            &enableServiceLinks,
					TerminationGracePeriodSeconds: &terminationGrace,
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup:             &fsGroup,
						FSGroupChangePolicy: &fsGroupChangePolicy,
					},
					InitContainers: []corev1.Container{initContainer},
					Containers:     containers,
				},
			},
			VolumeClaimTemplates: volumeClaimTemplates,
		},
	}

	return sts
}

// postStartScript generates the shell script executed as a postStart lifecycle hook.
func postStartScript(dp *appsv1.DevPod) string {
	username := dp.Spec.Auth.Username
	var sb strings.Builder

	// Account rename
	sb.WriteString(fmt.Sprintf("usermod -l %s user\n", username))
	sb.WriteString(fmt.Sprintf("usermod -d /home/%s -m %s\n", username, username))
	sb.WriteString(fmt.Sprintf("groupmod -n %s user\n", username))
	sb.WriteString(fmt.Sprintf("usermod -g %s %s\n", username, username))
	sb.WriteString(fmt.Sprintf("chown -R %s /home/%s\n", username, username))
	sb.WriteString(fmt.Sprintf("chgrp -R %s /home/%s\n", username, username))

	// Password application
	sb.WriteString(fmt.Sprintf("echo '%s:'\"$UBUNTU_PASSWORD\" | chpasswd\n", username))

	// Apt packages
	if len(dp.Spec.Packages.Apt) > 0 {
		sb.WriteString("export DEBIAN_FRONTEND=noninteractive\n")
		sb.WriteString("apt-get update\n")
		sb.WriteString(fmt.Sprintf("apt-get install -y%s || true\n", shellQuoteAll(dp.Spec.Packages.Apt)))
		sb.WriteString("apt-get clean\n")
		sb.WriteString("unset DEBIAN_FRONTEND\n")
	}

	// Pip packages
	if len(dp.Spec.Packages.Pip) > 0 {
		sb.WriteString(fmt.Sprintf("pip install --no-cache-dir%s || true\n", shellQuoteAll(dp.Spec.Packages.Pip)))
	}

	return sb.String()
}

// volumeClaim renders a PersistentVolumeClaim for use as a VolumeClaimTemplate entry.
func volumeClaim(dp *appsv1.DevPod, name string, size *resource.Quantity) corev1.PersistentVolumeClaim {
	storageClass := dp.Spec.Persistence.StorageClass
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: selectorLabels(dp),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: volumeSize(size),
				},
			},
		},
	}
}

// volumeSize returns the provided quantity, or defaultVolumeSize (50Gi) if nil or zero.
func volumeSize(size *resource.Quantity) resource.Quantity {
	if size == nil || size.IsZero() {
		return resource.MustParse(defaultVolumeSize)
	}
	return *size
}

// containerSecurityContext returns a SecurityContext appropriate for DevPod containers.
func containerSecurityContext(privileged bool) *corev1.SecurityContext {
	uid := int64(0)
	gid := int64(0)
	nonRoot := false
	allowEscalation := true
	readOnly := false
	return &corev1.SecurityContext{
		RunAsUser:                &uid,
		RunAsGroup:               &gid,
		RunAsNonRoot:             &nonRoot,
		AllowPrivilegeEscalation: &allowEscalation,
		ReadOnlyRootFilesystem:   &readOnly,
		Privileged:               &privileged,
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// resourcePreset returns ResourceRequirements with the given request/limit values
// plus hardcoded ephemeral-storage: request=50Mi, limit=2Gi.
func resourcePreset(reqCPU, reqMem, limCPU, limMem string) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse(reqCPU),
			corev1.ResourceMemory:           resource.MustParse(reqMem),
			corev1.ResourceEphemeralStorage: resource.MustParse("50Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse(limCPU),
			corev1.ResourceMemory:           resource.MustParse(limMem),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		},
	}
}

// sshProbe returns a Probe that checks TCP connectivity on port 22.
func sshProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(22),
			},
		},
		InitialDelaySeconds: 10,
		TimeoutSeconds:      20,
		PeriodSeconds:       30,
		FailureThreshold:    3,
		SuccessThreshold:    1,
	}
}

// shellQuoteAll returns a space-prefixed string of shell-quoted package names.
// E.g., ["curl", "git"] -> ` "curl" "git"`.
func shellQuoteAll(pkgs []string) string {
	if len(pkgs) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, pkg := range pkgs {
		sb.WriteString(fmt.Sprintf(" %q", pkg))
	}
	return sb.String()
}
