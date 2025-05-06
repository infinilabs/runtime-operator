// pkg/apis/common/types.go
// +kubebuilder:object:generate=true
// +groupName=common.infini.cloud
package common

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime" // Needed for RawExtension in app-specific types if used
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
// Used within specific component configs like GatewayConfig.
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

	// HeadlessServiceName explicitly sets the name for the Headless Service (if different from default convention).
	// Only relevant for StatefulSet workloads.
	// +optional
	// HeadlessServiceName *string `json:"headlessServiceName,omitempty"` // Example if needed

	// Add other common Service Spec fields if needed for configuration:
	// +optional
	// ClusterIP *string `json:"clusterIP,omitempty"`
	// +optional
	// SessionAffinity *corev1.ServiceAffinity `json:"sessionAffinity,omitempty"`
	// +optional
	// LoadBalancerIP *string `json:"loadBalancerIP,omitempty"`
	// etc.
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
	Create *bool `json:"create,omitempty"` // Default: true
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
type PodDisruptionBudgetSpecPart = policyv1.PodDisruptionBudgetSpec

// AppConfigData holds configuration data as key-value strings (filename -> content).
type AppConfigData map[string]string

// --- Application Specific Configuration Structures ---
// Define the STRUCTURE of the config provided in ApplicationComponent.Properties for each type.

// GatewayConfig defines the expected structure within ApplicationComponent.Properties when Type is "gateway".
// GatewayConfig defines the expected structure within ApplicationComponent.Properties when Type is "gateway".
// It includes core workload settings and potentially overrides for common components.
type GatewayConfig struct {
	// --- Core Workload Settings ---

	// Replicas defines the number of desired pods.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"` // Pointer to allow zero value

	// Image specifies the container image details.
	// +kubebuilder:validation:Required
	// +optional
	Image *ImageSpec `json:"image,omitempty"` // Pointer as it's checked for nil

	// Ports defines the network ports exposed by the gateway container.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +optional
	Ports []PortSpec `json:"ports,omitempty"` // Slice, builder checks for emptiness

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

	// Service defines how to expose the Gateway pods via a Kubernetes Service.
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

	// --- StatefulSet Specific Overrides (if Gateway is a StatefulSet) ---

	// StatefulSetUpdateStrategy defines the update strategy for the StatefulSet.
	// +optional
	StatefulSetUpdateStrategy *StatefulSetUpdateStrategyPart `json:"statefulSetUpdateStrategy,omitempty"` // Likely *appsv1.StatefulSetUpdateStrategy

	// PodManagementPolicy defines the pod management policy for the StatefulSet.
	// +optional
	PodManagementPolicy *PodManagementPolicyTypePart `json:"podManagementPolicy,omitempty"` // Likely *appsv1.PodManagementPolicyType

	// --- Pod Disruption Budget (Optional) ---

	// PodDisruptionBudget defines the PDB settings for the gateway deployment/statefulset.
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpecPart `json:"podDisruptionBudget,omitempty"` // Likely *policyv1.PodDisruptionBudgetSpec

	// --- Other Gateway specific parameters ---
	// Add any other flags or configurations specific to your Gateway application here.
	// Example:
	// +optional
	// SomeGatewayFeatureFlag *bool `json:"someGatewayFeatureFlag,omitempty"`
	// +optional
	// LogLevel *string `json:"logLevel,omitempty"`
}

// OpensearchClusterConfig defines parameters specific to OpenSearch clusters.
// (Structure defined earlier, keep it here for reference/use by other components)
type OpensearchClusterConfig struct {
	// +kubebuilder:validation:Required
	Version *string `json:"version"`
	// +kubebuilder:validation:Required
	Image ImageSpec `json:"image"`
	// +kubebuilder:validation:Required
	NodePools []OpensearchNodePoolSpec `json:"nodePools"`
	// ... other fields as defined before ...
}

// OpensearchNodePoolSpec defines configurations for a specific pool of nodes in an OpenSearch cluster.
type OpensearchNodePoolSpec struct {
	Name      string        `json:"name"`
	Replicas  *int32        `json:"replicas"`
	Roles     []string      `json:"roles"`
	Resources ResourcesSpec `json:"resources"`
	Storage   StorageSpec   `json:"storage"`
}

// ElasticsearchClusterConfig defines parameters specific to Elasticsearch clusters.
// (Structure defined earlier, keep it here for reference/use by other components)
type ElasticsearchClusterConfig struct {
	// +kubebuilder:validation:Required
	Version *string `json:"version"`
	// +kubebuilder:validation:Required
	Image ImageSpec `json:"image"`
	// +kubebuilder:validation:Required
	NodePools []ElasticsearchNodePoolSpec `json:"nodePools"`
}

// OpensearchNodePoolSpec defines configurations for a specific pool of nodes in an OpenSearch cluster.
type ElasticsearchNodePoolSpec struct {
	Name      string        `json:"name"`
	Replicas  *int32        `json:"replicas"`
	Roles     []string      `json:"roles"`
	Resources ResourcesSpec `json:"resources"`
	Storage   StorageSpec   `json:"storage"`
}

// AddScheme adds the types in this package to the given scheme.
func AddScheme(scheme *runtime.Scheme) error {
	// Register application-specific config types if they need to be handled by Scheme directly (e.g., for conversion).
	// Generally not needed if unmarshalling directly from RawExtension via json.Unmarshal.
	// err := scheme.AddKnownTypes(SchemeGroupVersion, &OpensearchClusterConfig{})
	return nil
}

// init function registers schemes or helpers if needed (usually empty here).
func init() {
	// SchemeBuilder is typically used in CRD API group packages (api/v1, api/app/v1).
}
