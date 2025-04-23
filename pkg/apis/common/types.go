// pkg/apis/common/types.go
// +kubebuilder:object:generate=true // Generate deepcopy code for types defined here
// +groupName=common.infini.cloud   // Optional: assign a group for common types themselves
package common

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"        // Needed for PDBSpecPart
	"k8s.io/apimachinery/pkg/api/resource" // Import metav1 if needed
	"k8s.io/apimachinery/pkg/runtime"      // Needed for Runtime.RawExtension if used (removed from here, used in CRDs)
	"k8s.io/apimachinery/pkg/util/intstr"
)

// WorkloadReference indicates the target Kubernetes workload type. Used in ComponentDefinition.
// This type specifies which K8s built-in controller manages the primary resource for a component type.
type WorkloadReference struct {
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// ImageSpec defines the container image configuration.
type ImageSpec struct {
	// +optional Repository string `json:"repository,omitempty"` // e.g. "nginx", "infinilabs/gateway"
	// +optional Tag string `json:"tag,omitempty"`
	// +optional PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
	Repository string            `json:"repository,omitempty"`
	Tag        string            `json:"tag,omitempty"`
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

// ResourcesSpec defines the CPU and Memory requests and limits for a container.
type ResourcesSpec struct {
	// +optional Limits corev1.ResourceList `json:"limits,omitempty"`
	// +optional Requests corev1.ResourceList `json:"requests,omitempty"`
	Limits   corev1.ResourceList `json:"limits,omitempty"`
	Requests corev1.ResourceList `json:"requests,omitempty"`
}

// PortSpec defines a container or service port.
type PortSpec struct {
	// +optional Name string `json:"name,omitempty"` // Port name (e.g., "http", "api")
	// +kubebuilder:validation:Required ContainerPort int32 `json:"containerPort"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ContainerPort int32 `json:"containerPort"` // Port number inside the container
	// +optional Protocol corev1.Protocol `json:"protocol,omitempty"` // Default: TCP
	// +optional TargetPort *intstr.IntOrString `json:"targetPort,omitempty"` // For Service, target port on Pod
	Name       string              `json:"name",omitempty"` // Make Name required by Convention if multiple ports
	Protocol   corev1.Protocol     `json:"protocol,omitempty"`
	TargetPort *intstr.IntOrString `json:"targetPort,omitempty"`
}

// ProbesConfig groups different types of probes. Uses K8s corev1.Probe directly.
type ProbesConfig struct {
	Liveness  *corev1.Probe `json:"liveness,omitempty"`  // Pointer to standard K8s Probe
	Readiness *corev1.Probe `json:"readiness,omitempty"` // Pointer to standard K8s Probe
	Startup   *corev1.Probe `json:"startup,omitempty"`   // Pointer to standard K8s Probe
}

// PersistenceSpec defines configuration for a shared PersistentVolumeClaim (typically for Deployment).
type PersistenceSpec struct {
	// +optional Enabled bool `json:"enabled,omitempty"` // Default: false. If enabled, Size, MountPath, etc should be provided.
	Enabled bool `json:"enabled,omitempty"` // Explicit bool value
	// +kubebuilder:validation:Required Size *resource.Quantity `json:"size"` // Required WHEN Enabled=true (Validation logic needed)
	Size *resource.Quantity `json:"size,omitempty"` // Pointer to Resource Quantity

	// Optional Naming/Mounting overrides
	// +optional VolumeName string `json:"volumeName,omitempty"` // Defaults to instanceName + "-pvc" or similar.
	// +optional MountPath string `json:"mountPath,omitempty"` // Mount path inside the container.
	VolumeName string `json:"volumeName",omitempty"` // Keep as value types for simpler defaulting logic
	MountPath  string `json:"mountPath",omitempty"`

	// Standard PVC fields
	// +optional StorageClassName *string `json:"storageClassName,omitempty"` // Optional StorageClass name. Pointer.
	// +optional AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"` // Default: ReadWriteOnce.
	StorageClassName *string `json:"storageClassName,omitempty"`
	// +kubebuilder:default={"ReadWriteOnce"}
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes",omitempty"`
}

// StorageSpec defines the template for PersistentVolumeClaims created per replica (typically for StatefulSet).
type StorageSpec struct {
	// +optional Enabled bool `json:"enabled,omitempty"` // Default: false. If enabled, Size, MountPath, etc should be provided.
	Enabled bool `json:"enabled",omitempty"`
	// +kubebuilder:validation:Required Size *resource.Quantity `json:"size"` // Required WHEN Enabled=true
	Size *resource.Quantity `json:"size",omitempty"` // Pointer

	// Optional Naming/Mounting overrides
	// +optional VolumeClaimTemplateName string `json:"volumeClaimTemplateName,omitempty"` // Default to "data" or instanceName + "-data".
	// +optional MountPath string `json:"mountPath,omitempty"` // Mount path inside the container.
	VolumeClaimTemplateName string `json:"volumeClaimTemplateName",omitempty"`
	MountPath               string `json:"mountPath",omitempty"`

	// Standard PVC fields for template
	// +optional StorageClassName *string `json:"storageClassName,omitempty"` // Pointer.
	// +optional AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes",omitempty"` // Default: ReadWriteOnce.
	StorageClassName *string `json:"storageClassName",omitempty"`
	// +kubebuilder:default={"ReadWriteOnce"}
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes",omitempty"`

	// Application Specific Storage fields (e.g. Elasticsearch/Opensearch data subdirectory within mount)
	// +optional DataSubpath *string `json:"dataSubpath,omitempty"` // Example: "data" if actual app data goes into /mount/data. Pointer.
	DataSubpath *string `json:"dataSubpath",omitempty"`
}

// ConfigMountSpec defines how to mount a ConfigMap as a volume.
type ConfigMountSpec struct {
	// +kubebuilder:validation:Required Name string `json:"name"` // Name of the ConfigMap resource.
	Name string `json:"name"`

	// +optional VolumeName string `json:"volumeName,omitempty"` // Defaults to ConfigMap name if empty.
	VolumeName string `json:"volumeName",omitempty"`

	// +kubebuilder:validation:Required MountPath string `json:"mountPath"` // Mount path inside the container.
	MountPath string `json:"mountPath"`

	// +optional SubPath *string `json:"subPath,omitempty"` // Pointer. Mount specific key as subpath.
	// +optional Items []corev1.KeyToPath `json:"items,omitempty"` // Mount specific keys as files. SubPath and Items are mutually exclusive in K8s.
	// +optional ReadOnly *bool `json:"readOnly,omitempty"` // Default: true. Pointer.
	SubPath  *string            `json:"subPath",omitempty"`
	Items    []corev1.KeyToPath `json:"items",omitempty"`
	ReadOnly *bool              `json:"readOnly",omitempty"`
}

// SecretMountSpec defines how to mount a Secret as a volume.
type SecretMountSpec struct {
	// +kubebuilder:validation:Required SecretName string `json:"secretName"` // Name of the Secret resource.
	SecretName string `json:"secretName"`

	// +optional VolumeName string `json:"volumeName,omitempty"` // Defaults to Secret name if empty.
	VolumeName string `json:"volumeName",omitempty"`

	// +kubebuilder:validation:Required MountPath string `json:"mountPath"` // Mount path inside the container.
	MountPath string `json:"mountPath"`

	// +optional Items []corev1.KeyToPath `json:"items,omitempty"` // Optional: Specify specific keys to mount.
	// +optional ReadOnly *bool `json:"readOnly,omitempty"` // Default: true. Pointer.
	Items    []corev1.KeyToPath `json:"items",omitempty"`
	ReadOnly *bool              `json:"readOnly",omitempty"`
}

// EnvVarSpec uses corev1.EnvVar directly.
type EnvVarSpec = corev1.EnvVar

// EnvFromSourceSpec uses corev1.EnvFromSource directly.
type EnvFromSourceSpec = corev1.EnvFromSource

// ServiceAccountSpec defines Service Account creation and usage.
type ServiceAccountSpec struct {
	// +optional Create *bool `json:"create,omitempty"` // Default: true. Pointer.
	Create *bool `json:"create",omitempty"`

	// +optional Name string `json:"name,omitempty"` // Defaults to derived name (component name + "-sa")
	Name string `json:"name",omitempty"`

	// +optional Annotations map[string]string `json:"annotations,omitempty"`
	Annotations map[string]string `json:"annotations",omitempty"`
}

// NodeSelectorSpec uses map directly.
type NodeSelectorSpec map[string]string

// TolerationsSpec uses corev1.Toleration directly.
type TolerationsSpec = corev1.Toleration

// AffinitySpec uses corev1.Affinity directly. Pointer.
type AffinitySpec = corev1.Affinity

// PodSecurityContextSpec uses corev1.PodSecurityContext directly. Pointer.
type PodSecurityContextSpec = corev1.PodSecurityContext

// ContainerSecurityContextSpec uses corev1.SecurityContext directly. Pointer.
type ContainerSecurityContextSpec = corev1.SecurityContext

// PersistentVolumeClaimSpecPart uses corev1.PersistentVolumeClaimSpec directly.
type PersistentVolumeClaimSpecPart = corev1.PersistentVolumeClaimSpec

// DeploymentStrategyPart uses appsv1.DeploymentStrategy directly. Pointer.
type DeploymentStrategyPart = appsv1.DeploymentStrategy

// StatefulSetUpdateStrategyPart uses appsv1.StatefulSetUpdateStrategy directly. Pointer.
type StatefulSetUpdateStrategyPart = appsv1.StatefulSetUpdateStrategy

// PodManagementPolicyTypePart uses appsv1.PodManagementPolicyType directly. Pointer.
type PodManagementPolicyTypePart *appsv1.PodManagementPolicyType

// PodDisruptionBudgetSpecPart uses policyv1.PodDisruptionBudgetSpec directly. Pointer.
type PodDisruptionBudgetSpecPart = policyv1.PodDisruptionBudgetSpec

// AppConfigData holds configuration data that is marshalled into files (e.g., for ConfigMap data).
// Key = filename (e.g., "gateway.yml", "opensearch.yml"), Value = file content string
// This is often part of an application-specific config structure.
type AppConfigData map[string]string

// --- Application Specific Configuration Structures ---
// These are the TOP-LEVEL structures users will define in ApplicationComponent.Properties (RawExtension).
// Add a struct for EACH application type your operator supports (e.g., "opensearch", "elasticsearch", "gateway").
// They should *contain* the configuration specific to that application.
// The controller's strategy dispatcher will identify and unmarshal RawExtension into the correct one.

// Example 1: OpensearchClusterConfig (For ApplicationComponent.Type="opensearch")
// Defines all parameters specific to deploying and managing an OpenSearch cluster.
type OpensearchClusterConfig struct {
	// K8s Standard fields configured specifically for OS/ES clusters
	// +optional Replicas *int32 `json:"replicas,omitempty"`
	// +optional Image ImageSpec `json:"image,omitempty"`
	// +optional Resources *ResourcesSpec `json:"resources,omitempty"` // Defaults per pool usually

	// +kubebuilder:validation:Required Version *string `json:"version"` // OpenSearch Version is essential
	Version *string `json:"version",omitempty"`

	// +kubebuilder:validation:Required NodePools []OpensearchNodePoolSpec `json:"nodePools"` // OpenSearch Node Pools are fundamental
	NodePools []OpensearchNodePoolSpec `json:"nodePools",omitempty"`

	// Service Configuration for OS - specific structure reflecting OS endpoints (Client/Transport/REST)
	// +optional Services OpensearchServiceConfig `json:"services,omitempty"` // Structure for Client/Transport/Rest Services

	// Security Configuration - Certs, Security Plugin (very complex and OS version dependent)
	// +optional Security OpensearchSecurityConfig `json:"security,omitempty"`

	// OpenSearch Configuration file data and options
	// unstructured/raw sections from opensearch.yml, jvm.options, etc.
	// +optional NodeConfig *runtime.RawExtension `json:"nodeConfig,omitempty"`
	// +optional ClusterConfig *runtime.RawExtension `json:"clusterConfig,omitempty"`

	// Add more OS specific features config: Index management, Shard allocation, JVM, Plugins, Snapshot/Restore, CCR etc.
	// These often map to Task Runner jobs or specialized controllers/tasks.

	// You will likely define dedicated structures for OpensearchServiceConfig, OpensearchSecurityConfig, OpenSearchNodePoolSpec etc.
}

// Example for OpensearchNodePoolSpec (used within OpensearchClusterConfig)
type OpensearchNodePoolSpec struct {
	// +kubebuilder:validation:Required Name string `json:"name"` // Data, Master, Ingest, Client etc.
	// +kubebuilder:validation:Required Replicas *int32 `json:"replicas"`
	// +optional Roles []string `json:"roles,omitempty"` // e.g. ["data", "master"]

	Name     string   `json:"name"`
	Replicas *int32   `json:"replicas",omitempty"`
	Roles    []string `json:"roles",omitempty"`

	// Resources per node in pool
	// +kubebuilder:validation:Required Resources ResourcesSpec `json:"resources"`
	Resources ResourcesSpec `json:"resources",omitempty"`

	// Persistent storage for data/logs/config per node in pool (usually applies to data, master, ingest)
	// +kubebuilder:validation:Required Storage StorageSpec `json:"storage"`
	Storage StorageSpec `json:"storage",omitempty"` // Note: StorageSpec from common

	// Optional overrides for standard K8s/common config parts at NodePool level
	// These override corresponding fields in OpensearchClusterConfig if specified here.
	// +optional Image *ImageSpec `json:"image,omitempty"` // Override cluster image for this pool
	// +optional Env []EnvVarSpec `json:"env,omitempty"` // Additional env vars per pool
	// +optional Probes *ProbesConfig `json:"probes,omitempty"` // Probes specific to roles/pool

	// +optional NodeSelector NodeSelectorSpec `json:"nodeSelector,omitempty"` // Scheduling specific to pool
	// +optional Tolerations []TolerationsSpec `json:"tolerations,omitempty"`
	// +optional Affinity *AffinitySpec `json:"affinity,omitempty"`
	// +optional PodSecurityContext *PodSecurityContextSpec `json:"podSecurityContext,omitempty"`
	// +optional ContainerSecurityContext *ContainerSecurityContextSpec `json:"containerSecurityContext,omitempty"`

	// JVM Options specific to nodes in this pool (e.g. heap size, other flags)
	// +optional JvmOptions []string `json:"jvmOptions,omitempty"`

	// Raw configuration file content specific to this pool (merged with cluster-wide and higher levels)
	// Key = filename, Value = content. Builders handle merge.
	// +optional Config map[string]string `json:"config,omitempty"` // Raw key-value config per file per node in pool
}

// Define structures for other sections in OpensearchClusterConfig as needed.
// type OpensearchServiceConfig struct { /* ... Client/Transport/REST service port mapping ... */ }
// type OpensearchSecurityConfig struct { /* ... Security config... */ }
// ...

// Example 2: ElasticsearchClusterConfig (For ApplicationComponent.Type="elasticsearch")
// Define similar structure as OpensearchClusterConfig, using ES terminology.
// type ElasticsearchClusterConfig { /* ... Structure mirroring ES specifics (nodeSets, terminology, config) ... */ }

// Example 3: GatewayConfig (For ApplicationComponent.Type="gateway")
// Defines all parameters specific to the Gateway application.
// This IS the concrete structure used for ApplicationComponent.Properties when type is "gateway"
type GatewayConfig struct {
	// Basic K8s deployment specs required for a Gateway instance
	// +kubebuilder:validation:Required Replicas *int32 `json:"replicas"` // Replicas for Gateway's workload
	Replicas *int32 `json:"replicas",omitempty"`

	// +kubebuilder:validation:Required Image ImageSpec `json:"image"` // Image for the Gateway container
	Image ImageSpec `json:"image",omitempty"` // Note: ImageSpec is from common

	// Container Ports for the main Gateway container
	// +kubebuilder:validation:Required Ports []PortSpec `json:"ports"` // Needs common.PortSpec definition
	Ports []PortSpec `json:"ports",omitempty"`

	// Other frequently customized standard K8s configs for Gateway Pods/Container
	// +optional Resources *ResourcesSpec `json:"resources,omitempty"`
	// +optional Env []EnvVarSpec `json:"env,omitempty"`
	// +optional EnvFrom []EnvFromSourceSpec `json:"envFrom,omitempty"`
	// +optional Probes *ProbesConfig `json:"probes,omitempty"`
	// +optional ContainerSecurityContext *ContainerSecurityContextSpec `json:"containerSecurityContext,omitempty"`

	// Service config to expose the Gateway
	// +optional
	Service *ServiceSpecPart `json:"service,omitempty"` // Common Service config part

	// Persistence/Storage depending on the Gateway's workload type (Deployment vs StatefulSet)
	// Determined by ComponentDefinition, configured here based on *usage* pattern
	// +optional Persistence *PersistenceSpec `json:"persistence,omitempty"` // If Gateway uses shared PVC (Deployment)
	// +optional Storage *StorageSpec `json:"storage,omitempty"` // If Gateway uses per-replica PVCs (StatefulSet)

	// --- Configuration File Data (Gateway) ---
	// Raw key-value content for gateway config files (e.g., gateway.yml)
	// +optional ConfigFiles map[string]string `json:"configFiles,omitempty"` // Map of filename to content

	// Other common K8s Pod Spec overrides for Gateway
	// +optional PodSecurityContext *PodSecurityContextSpec `json:"podSecurityContext,omitempty"`
	// +optional ServiceAccount *ServiceAccountSpec `json:"serviceAccount,omitempty"`
	// +optional NodeSelector NodeSelectorSpec `json:"nodeSelector,omitempty"`
	// +optional Tolerations []TolerationsSpec `json:"tolerations,omitempty"`
	// +optional Affinity *AffinitySpec `json:"affinity,omitempty"`

	// Optional Init Containers for Gateway specific setup (raw K8s spec)
	// +optional InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Optional Volumes/Mounts (beyond Persistence/Storage/ConfigMaps/Secrets)
	// +optional VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"` // User explicit raw mounts to main container
	// +optional ConfigMounts []ConfigMapMountSpec `json:"configMounts,omitempty"` // User explicit CM mounts using simplified spec
	// +optional SecretMounts []SecretMountSpec `json:"secretMounts,omitempty"` // User explicit Secret mounts
	// +optional AdditionalVolumes []corev1.Volume `json:"additionalVolumes,omitempty"` // EmptyDir, HostPath etc. as raw K8s Volumes

	// This config is specific to the Gateway application logic/features
	// +optional SomeGatewayFeatureFlag *bool `json:"someGatewayFeatureFlag,omitempty"`

}

// AddScheme adds the types in this package to the given scheme.
// This is called from init() functions in specific CRD packages (core/v1 and app/v1).
// This function itself does NOT register common types as root types.
// Its purpose is to be called BY the init() funcs in CRD packages that import this.
func AddScheme(scheme *runtime.Scheme) error {
	// Nothing to do here in the common package itself usually, as types are nested.
	// Registration is done by the CRD init() functions where types are used.
	// Unless common types are registered as root level API themselves (rare).
	return nil
}

// init() function is not strictly needed here if AddScheme is used by importing packages' init funcs.
// func init() {
// 	// Nothing to register if only nested types are defined.
// }
