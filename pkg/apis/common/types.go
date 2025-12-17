// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Runtime Operator is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

// pkg/apis/common/types.go
// +kubebuilder:object:generate=true
// +groupName=common.infini.cloud
package common

import (
	"fmt"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// WorkloadReference indicates the target Kubernetes workload type.
type WorkloadReference struct {
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// ImageSpec defines the container image configuration.
type ImageSpec struct {
	// +optional
	Repository string `json:"repository,omitempty"`
	// +optional
	Tag string `json:"tag,omitempty"`
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

// ResourcesSpec defines the CPU and Memory requests and limits.
type ResourcesSpec struct {
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`
}

// PortSpec defines a container or service port.
type PortSpec struct {
	// +optional
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Required
	ContainerPort int32 `json:"containerPort"`
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`
	// +optional
	TargetPort *intstr.IntOrString `json:"targetPort,omitempty"`
	// +optional
	NodePort int32 `json:"nodePort,omitempty"`
}

// ProbesConfig groups different types of probes.
type ProbesConfig struct {
	// +optional
	Liveness *corev1.Probe `json:"liveness,omitempty"`
	// +optional
	Readiness *corev1.Probe `json:"readiness,omitempty"`
	// +optional
	Startup *corev1.Probe `json:"startup,omitempty"`
}

// ServiceSpecPart is a helper struct grouping common Service configuration fields.
// Used within specific component configs like RuntimeConfig.
type ServiceSpecPart struct {
	// Type specifies the Kubernetes Service type (ClusterIP, NodePort, LoadBalancer).
	// Defaults to ClusterIP if not specified.
	// +optional
	Type *corev1.ServiceType `json:"type,omitempty"`

	// Ports defines the ports the Service should expose.
	// If not specified, ports from the main component config might be used.
	// +optional
	Ports []PortSpec `json:"ports,omitempty"`

	// Annotations specific to the Service resource.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// SessionAffinity specifies the session affinity for the Service.
	// Supports "ClientIP" and "None".
	// Defaults to "None" if not specified.
	// +optional
	SessionAffinity *corev1.ServiceAffinity `json:"sessionAffinity,omitempty"`

	// SessionAffinityConfig contains the configurations of session affinity.
	// +optional
	SessionAffinityConfig *corev1.SessionAffinityConfig `json:"sessionAffinityConfig,omitempty"`
}

// PersistenceSpec defines configuration for a shared PersistentVolumeClaim (for Deployment).
type PersistenceSpec struct {
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// +optional
	Size *resource.Quantity `json:"size,omitempty"`
	// +optional
	VolumeName string `json:"volumeName,omitempty"`
	// +optional
	MountPath string `json:"mountPath,omitempty"`
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// StorageSpec defines the template for PersistentVolumeClaims created per replica (for StatefulSet).
type StorageSpec struct {
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// +optional
	Size *resource.Quantity `json:"size,omitempty"`
	// +optional
	VolumeClaimTemplateName string `json:"volumeClaimTemplateName,omitempty"`
	// +optional
	MountPath string `json:"mountPath,omitempty"`
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	// +optional
	DataSubpath *string `json:"dataSubpath,omitempty"`
}

// ConfigMountSpec defines how to mount a ConfigMap as a volume.
type ConfigMountSpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +optional
	VolumeName string `json:"volumeName,omitempty"`
	// +kubebuilder:validation:Required
	MountPath string `json:"mountPath"`
	// +optional
	SubPath *string `json:"subPath,omitempty"`
	// +optional
	Items []corev1.KeyToPath `json:"items,omitempty"`
	// +optional
	ReadOnly *bool `json:"readOnly,omitempty"`
}

// SecretMountSpec defines how to mount a Secret as a volume.
type SecretMountSpec struct {
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`
	// +optional
	VolumeName string `json:"volumeName,omitempty"`
	// +kubebuilder:validation:Required
	MountPath string `json:"mountPath"`
	// +optional
	Items []corev1.KeyToPath `json:"items,omitempty"`
	// +optional
	ReadOnly *bool `json:"readOnly,omitempty"`
}

// EnvVarSpec uses corev1.EnvVar directly.
type EnvVarSpec = corev1.EnvVar

// EnvFromSourceSpec uses corev1.EnvFromSource directly.
type EnvFromSourceSpec = corev1.EnvFromSource

// ServiceAccountSpec defines Service Account creation and usage.
type ServiceAccountSpec struct {
	// +optional
	Create *bool `json:"create,omitempty"` // Default: false
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// NodeSelectorSpec uses map directly.
type NodeSelectorSpec map[string]string

// TolerationsSpec uses corev1.Toleration directly.
type TolerationsSpec = corev1.Toleration

// AffinitySpec uses corev1.Affinity directly.
type AffinitySpec = corev1.Affinity

// PodSecurityContextSpec uses corev1.PodSecurityContext directly.
type PodSecurityContextSpec = corev1.PodSecurityContext

// ContainerSecurityContextSpec uses corev1.SecurityContext directly.
type ContainerSecurityContextSpec = corev1.SecurityContext

// DeploymentStrategyPart uses appsv1.DeploymentStrategy directly.
type DeploymentStrategyPart = appsv1.DeploymentStrategy

// StatefulSetUpdateStrategyPart uses appsv1.StatefulSetUpdateStrategy directly.
type StatefulSetUpdateStrategyPart = appsv1.StatefulSetUpdateStrategy

// PodManagementPolicyTypePart uses appsv1.PodManagementPolicyType directly.
type PodManagementPolicyTypePart = appsv1.PodManagementPolicyType

// PodDisruptionBudgetSpecPart uses policyv1.PodDisruptionBudgetSpec directly.
// type PodDisruptionBudgetSpecPart = policyv1.PodDisruptionBudgetSpec

// 双版本类型定义
type (
	PodDisruptionBudgetSpecV1      = policyv1.PodDisruptionBudgetSpec
	PodDisruptionBudgetSpecV1beta1 = policyv1beta1.PodDisruptionBudgetSpec
)

// AppConfigData holds configuration data as key-value strings (filename -> content).
type AppConfigData map[string]string

// --- Application Specific Configuration Structures ---
// Define the STRUCTURE of the config provided in ApplicationComponent.Properties for each type.

// It includes core workload settings and potentially overrides for common components.
type RuntimeConfig struct {
	// --- Core Workload Settings ---

	// Replicas defines the number of desired pods.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"` // Pointer to allow zero value

	// Image specifies the container image details.
	// +kubebuilder:validation:Required
	// +optional
	Image   *ImageSpec `json:"image,omitempty"` // Pointer as it's checked for nil
	Command []string   `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`
	Args    []string   `json:"args,omitempty" protobuf:"bytes,4,rep,name=args"`

	// Ports defines the network ports exposed by the container.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +optional
	Ports []PortSpec `json:"ports,omitempty"` // Slice, builder checks for emptiness

	InitContainer *corev1.Container
	// --- Optional Standard Overrides ---

	// Resources specifies CPU and memory resource requests and limits for the main container.
	// +optional
	Resources *ResourcesSpec `json:"resources,omitempty"`

	// Env defines environment variables for the main container.
	// +optional
	Env []EnvVarSpec `json:"env,omitempty"` // EnvVarSpec is likely corev1.EnvVar

	// EnvFrom defines sources to populate environment variables from (e.g., ConfigMaps, Secrets).
	// +optional
	EnvFrom []EnvFromSourceSpec `json:"envFrom,omitempty"` // EnvFromSourceSpec is likely corev1.EnvFromSource

	// Probes defines liveness, readiness, and startup probe configurations.
	// +optional
	Probes *ProbesConfig `json:"probes,omitempty"`

	// ContainerSecurityContext defines security settings specific to the main container.
	// +optional
	ContainerSecurityContext *ContainerSecurityContextSpec `json:"containerSecurityContext,omitempty"` // Likely *corev1.SecurityContext

	// PodSecurityContext defines security settings for the entire pod.
	// +optional
	PodSecurityContext *PodSecurityContextSpec `json:"podSecurityContext,omitempty"` // Likely *corev1.PodSecurityContext

	// ServiceAccount defines configuration for the Kubernetes Service Account.
	// +optional
	ServiceAccount *ServiceAccountSpec `json:"serviceAccount,omitempty"`

	// --- Scheduling Overrides ---

	// NodeSelector specifies label selectors for node assignment.
	// +optional
	NodeSelector NodeSelectorSpec `json:"nodeSelector,omitempty"` // Likely map[string]string

	// Tolerations specify pod tolerations for scheduling.
	// +optional
	Tolerations []TolerationsSpec `json:"tolerations,omitempty"` // Likely []corev1.Toleration

	// Affinity specifies pod affinity and anti-affinity rules.
	// +optional
	Affinity *AffinitySpec `json:"affinity,omitempty"` // Likely *corev1.Affinity

	// --- Storage ---
	// Use EITHER Persistence (for Deployment-like shared PVC) OR Storage (for StatefulSet VCTs).
	// The builder logic should likely enforce this exclusivity based on the target workload.

	// Persistence defines configuration for a shared PersistentVolumeClaim (typically for Deployment).
	// +optional
	Persistence *PersistenceSpec `json:"persistence,omitempty"`

	// Storage defines the template for PersistentVolumeClaims created per replica (typically for StatefulSet).
	// +optional
	Storage *StorageSpec `json:"storage,omitempty"`

	// --- Service Exposure ---

	// Service defines how to expose the pods via a Kubernetes Service.
	// +optional
	Service *ServiceSpecPart `json:"service,omitempty"` // Use the helper struct defined below

	// --- Volume and Configuration Mounting ---

	// ConfigFiles provides configuration file content as key-value pairs (filename -> content).
	// These will typically be mounted via a ConfigMap generated by the operator.
	// +optional
	ConfigFiles AppConfigData `json:"configFiles,omitempty"` // Likely map[string]string

	// SecretFiles provides sensitive configuration file content as key-value pairs.
	// These will typically be mounted via a Secret generated by the operator.
	// +optional
	// SecretFiles AppConfigData `json:"secretFiles,omitempty"` // Add if needed

	// ConfigMounts specifies how to mount existing ConfigMaps as volumes.
	// +optional
	ConfigMounts []ConfigMountSpec `json:"configMounts,omitempty"`

	// SecretMounts specifies how to mount existing Secrets as volumes.
	// +optional
	SecretMounts []SecretMountSpec `json:"secretMounts,omitempty"`

	// AdditionalVolumes allows specifying custom volumes (e.g., emptyDir, hostPath) directly.
	// Use with caution.
	// +optional
	AdditionalVolumes []corev1.Volume `json:"additionalVolumes,omitempty"`

	// VolumeMounts allows specifying custom volume mounts for the main container.
	// Use with caution, ensure volume names match defined volumes.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// StatefulSetUpdateStrategy defines the update strategy for the StatefulSet.
	// +optional
	StatefulSetUpdateStrategy *StatefulSetUpdateStrategyPart `json:"statefulSetUpdateStrategy,omitempty"` // Likely *appsv1.StatefulSetUpdateStrategy

	// PodManagementPolicy defines the pod management policy for the StatefulSet.
	// +optional
	PodManagementPolicy *PodManagementPolicyTypePart `json:"podManagementPolicy,omitempty"` // Likely *appsv1.PodManagementPolicyType

	// --- Pod Disruption Budget (Optional) ---

	// PodDisruptionBudget defines the PDB settings for the deployment/statefulset.
	// +optional
	PodDisruptionBudgetBeta1 *PodDisruptionBudgetSpecV1beta1 `json:"podDisruptionBudgetBeta1,omitempty"`
	PodDisruptionBudget      *PodDisruptionBudgetSpecV1      `json:"podDisruptionBudget,omitempty"`
}

var Namespace string

func getInClusterNamespace() (string, error) {
	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	if _, err := os.Stat(InClusterNamespacePath); os.IsNotExist(err) {
		return "", fmt.Errorf("not running in-cluster, please check the Namespace")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}

	// Load the namespace file and return its content
	namespace, err := os.ReadFile(InClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(namespace), nil
}

// init function registers schemes or helpers if needed (usually empty here).
func init() {
	// SchemeBuilder is typically used in CRD API group packages (api/v1, api/app/v1).
	Namespace = "default"
	// Get namespace from env in debug mode
	if ns := os.Getenv("NAMESPACE"); ns != "" {
		Namespace = ns
		return
	}
	// Try to get namespace from in-cluster config, but don't panic if not in cluster
	if n, err := getInClusterNamespace(); err == nil {
		Namespace = n
	}
	// If not in cluster (e.g., running tests), keep default namespace
}
