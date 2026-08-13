/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// =============================================================================
// Condition type constants
// =============================================================================

const (
	// ConditionReady indicates the DevPod is fully functional.
	ConditionReady = "Ready"
	// ConditionProgressing indicates the DevPod is being created or updated.
	ConditionProgressing = "Progressing"
	// ConditionDegraded indicates the DevPod failed to reach or maintain its desired state.
	ConditionDegraded = "Degraded"
)

// =============================================================================
// Reason constants
// =============================================================================

const (
	// ReasonPasswordNotSet indicates the SSH password secret has not been created.
	ReasonPasswordNotSet = "PasswordNotSet"
	// ReasonInvalidPassword indicates the SSH password secret contains invalid data.
	ReasonInvalidPassword = "InvalidPassword"
	// ReasonStorageClassNotFound indicates the specified storage class does not exist.
	ReasonStorageClassNotFound = "StorageClassNotFound"
	// ReasonReconcileError indicates a generic reconciliation error.
	ReasonReconcileError = "ReconcileError"
	// ReasonWaitingForStatefulSet indicates the StatefulSet is not yet ready.
	ReasonWaitingForStatefulSet = "WaitingForStatefulSet"
	// ReasonDevPodReady indicates the DevPod is ready.
	ReasonDevPodReady = "DevPodReady"
)

// =============================================================================
// PVCReclaimPolicy enum
// =============================================================================

// PVCReclaimPolicy describes the reclaim policy for PVCs created by DevPod.
// +kubebuilder:validation:Enum=Retain;Delete
type PVCReclaimPolicy string

const (
	// PVCReclaimRetain retains the PVC when the DevPod is deleted.
	PVCReclaimRetain PVCReclaimPolicy = "Retain"
	// PVCReclaimDelete deletes the PVC when the DevPod is deleted.
	PVCReclaimDelete PVCReclaimPolicy = "Delete"
)

// =============================================================================
// Spec sub-structs
// =============================================================================

// AuthSpec holds authentication configuration for the DevPod.
type AuthSpec struct {
	// username is the Linux account name inside the DevPod.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=31
	// +kubebuilder:validation:Pattern=`^[a-z_][a-z0-9_-]*$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="auth.username is immutable"
	// +kubebuilder:validation:XValidation:rule="!(self in ['root','daemon','bin','sys','sync','games','man','lp','mail','news','uucp','proxy','www-data','backup','list','irc','nobody','sshd','user','users','staff','adm','tty','disk','dialout','cdrom','floppy','tape','sudo','audio','video','plugdev','ssh','docker','nogroup','src','shadow','utmp','crontab','operator','_apt'])",message="auth.username collides with an account or group that already exists in the DevPod image"
	// +required
	Username string `json:"username"`
}

// PersistenceSpec holds persistent storage configuration for the DevPod.
type PersistenceSpec struct {
	// storageClass is the name of the StorageClass to use.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="persistence.storageClass is immutable"
	// +required
	StorageClass string `json:"storageClass"`

	// size is the requested storage size.
	// +optional
	// +kubebuilder:default="50Gi"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="persistence.size is immutable"
	Size *resource.Quantity `json:"size,omitempty"`

	// reclaimPolicy controls PVC lifecycle on DevPod deletion.
	// +optional
	// +kubebuilder:default="Retain"
	ReclaimPolicy PVCReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// DockerPersistenceSpec holds persistent storage configuration for the Docker sidecar.
type DockerPersistenceSpec struct {
	// size is the requested storage size for Docker data.
	// +optional
	// +kubebuilder:default="50Gi"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="docker.persistence.size is immutable"
	Size *resource.Quantity `json:"size,omitempty"`
}

// DockerSpec holds configuration for the Docker-in-Docker sidecar.
type DockerSpec struct {
	// enabled controls whether the Docker sidecar is deployed.
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// image is the Docker-in-Docker container image.
	// +optional
	// +kubebuilder:default="docker.io/docker:dind"
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image,omitempty"`

	// persistence holds storage configuration for Docker data.
	// +optional
	Persistence DockerPersistenceSpec `json:"persistence,omitempty"`
}

// NodePortMapping maps a container port to a node port.
type NodePortMapping struct {
	// src is the container port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +required
	Src int32 `json:"src"`

	// dest is the node port.
	// +kubebuilder:validation:Minimum=30000
	// +kubebuilder:validation:Maximum=32767
	// +required
	Dest int32 `json:"dest"`
}

// PackagesSpec holds package installation configuration.
type PackagesSpec struct {
	// apt is a list of apt packages to install.
	// +optional
	// +listType=atomic
	Apt []string `json:"apt,omitempty"`

	// pip is a list of pip packages to install.
	// +optional
	// +listType=atomic
	Pip []string `json:"pip,omitempty"`
}

// =============================================================================
// DevPodSpec
// =============================================================================

// DevPodSpec defines the desired state of DevPod.
type DevPodSpec struct {
	// image is the main DevPod container image.
	// +optional
	// +kubebuilder:default="docker.io/tcfwbper/dev-env:1.0.0"
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image,omitempty"`

	// initWorkspaceImage is the init container image for workspace setup.
	// +optional
	// +kubebuilder:default="docker.io/tcfwbper/dev-env:1.0.0-init-workspace"
	// +kubebuilder:validation:MinLength=1
	InitWorkspaceImage string `json:"initWorkspaceImage,omitempty"`

	// auth holds authentication configuration.
	// +required
	Auth AuthSpec `json:"auth"`

	// persistence holds persistent storage configuration.
	// +required
	Persistence PersistenceSpec `json:"persistence"`

	// nodePorts maps container ports to node ports.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:rule="self.exists(e, e.src == 22)",message="nodePorts must contain an entry with src 22 (ssh)"
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.all(y, x.src == y.src || x.dest != y.dest))",message="nodePorts dest values must be unique"
	// +listType=map
	// +listMapKey=src
	// +required
	NodePorts []NodePortMapping `json:"nodePorts"`

	// packages holds package installation configuration.
	// +optional
	Packages PackagesSpec `json:"packages,omitempty"`

	// docker holds Docker-in-Docker sidecar configuration.
	// +optional
	Docker DockerSpec `json:"docker,omitempty"`
}

// DockerEnabled returns true when Docker sidecar should be deployed.
// It returns true when Docker.Enabled is nil (default) or explicitly true.
// It returns false only when Docker.Enabled is explicitly set to false.
func (s *DevPodSpec) DockerEnabled() bool {
	if s.Docker.Enabled == nil {
		return true
	}
	return *s.Docker.Enabled
}

// =============================================================================
// DevPodStatus
// =============================================================================

// DevPodStatus defines the observed state of DevPod.
type DevPodStatus struct {
	// conditions represent the current state of the DevPod resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the generation this status was computed from.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// readyReplicas mirrors the StatefulSet readyReplicas.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// passwordSecret is the name of the owned Secret holding the ssh password.
	// +optional
	PasswordSecret string `json:"passwordSecret,omitempty"`

	// sshNodePort is the node port mapped to container port 22.
	// +optional
	SSHNodePort int32 `json:"sshNodePort,omitempty"`
}

// =============================================================================
// Resource types
// =============================================================================

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=dp
// +kubebuilder:printcolumn:name="Username",type=string,JSONPath=`.spec.auth.username`
// +kubebuilder:printcolumn:name="SSH",type=integer,JSONPath=`.status.sshNodePort`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DevPod is the Schema for the devpods API.
type DevPod struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DevPod.
	// +required
	Spec DevPodSpec `json:"spec"`

	// status defines the observed state of DevPod.
	// +optional
	Status DevPodStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DevPodList contains a list of DevPod.
type DevPodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DevPod `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &DevPod{}, &DevPodList{})
		return nil
	})
}
