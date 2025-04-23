// pkg/builders/opensearch/builders.go
package opensearch

import (
	corev1 "k8s.io/api/core/v1"

	// App types
	"github.com/infinilabs/operator/pkg/apis/common" // Common types
	// Common utils
	builders "github.com/infinilabs/operator/pkg/builders/k8s" // Import generic K8s builders
	// Needed to call other builders/helpers in this package
)

// OpensearchAppBuilder is an instance of the AppBuilderStrategy for OpenSearch.
// This builder handles translating OpenSearch specific config to K8s objects.
// It doesn't hold state for a specific reconcile.
// Example: Define methods if builders for OS specific parts live here and not in sub-packages.

// We can just put the logic directly into the BuildObjects method itself if it's the only top-level entry point.

// --- Helper functions specific to OpenSearch config and K8s mapping ---
// These are builders specific to the *OpenSearch application* patterns.

// BuildOpenSearchNodePoolStatefulSet builds a StatefulSet for a specific OpenSearch Node Pool.
// A single OpenSearchCluster often maps to multiple StatefulSets (one per node pool).
// This is a conceptual builder - how OS maps config to multiple STS needs definition.
// If Component means the ENTIRE cluster (single STS), this builder is simpler and focuses on *one* STS.
// Let's assume one Component = One primary STS + services for the *entire* cluster.
// Variations in nodes/roles are handled by ConfigMaps, InitContainers, and the OS process itself.

// BuildOpenSearchMainContainerSpec builds the corev1.Container spec for the main OpenSearch container.
// It maps config specific to OpenSearch process (version, roles, paths, probes, JVM).
func buildOpenSearchMainContainerSpec(osConfig *common.OpensearchClusterConfig, nodePool *common.OpensearchNodePoolSpec, defaultImage common.ImageSpec) (corev1.Container, error) {
	// --- Essential Container Fields ---
	// Get image from pool override or cluster default
	imageSpec := defaultImage
	if nodePool.Image != nil {
		imageSpec = *nodePool.Image // Pool override takes precedence (value copy)
	}
	imageName := builders.BuildImageName(&imageSpec) // Use common helper
	imagePullPolicy := builders.GetImagePullPolicyOrDefault(imageSpec.PullPolicy)

	// Get resources from pool config
	resources := builders.GetResourcesSpecOrDefault(nodePool.Resources) // Returns value struct

	// Container name - maybe based on role or generic
	containerName := "opensearch" // Common name

	// Command and Args - highly OS specific, depends on version, setup.
	// Might need scripts or env vars passed in. Entrypoint is often default.
	command := []string{
		// Path to the OpenSearch executable (version/install dir dependent)
		// Use path joining with install path derived earlier or passed in.
		// "/usr/share/opensearch/bin/opensearch" // Standard path
		// Example Args: Role specific, config file location etc.
	}
	args := []string{} // Often configured via config files or ENV vars

	// Ports - specific OS ports (HTTP/Transport)
	// Common.Ports defines application ports. This needs mapping OS internal concepts to those.
	// How does osConfig.Services relate to ports? Maybe Services config provides this mapping?
	// This is application specific.
	// Example: Default OS ports 9200(HTTP), 9300(Transport)
	osPorts := []corev1.ContainerPort{} // Map OS roles/configs to K8s ports list

	// Environment Variables - highly OS specific. JVM options, node discovery settings.
	// Requires extracting info from osConfig and constructing standard EnvVar list.
	// Might use config files from volumes too.
	// Add common Env Vars derived by operator, then specific OS ones, then user defined.
	envVars := []corev1.EnvVar{} // Aggregate base + OS specific + pool specific + user env
	// Base operator env (POD_NAME etc) - probably added by the main pod builder later.
	// Specific OS/Pool envs: e.g. DISCOVERY_SEED_HOSTS, CLUSTER_INITIAL_MASTER_NODES, OPENSEARCH_JAVA_OPTS, NODE_NAME etc.
	// This requires detailed logic pulling values from osConfig and its sub-structs (discovery, network, jvm, roles, etc.).

	// VolumeMounts - These are added to containers *later* by assembler, referencing specific Volumes by Name.

	// Probes - OS specific, /_cluster/health endpoints, transport layer checks.
	// This requires logic based on config/version.
	livenessProbe := osConfig.Probes.Liveness.DeepCopy() // Apply default probe here? or use specific config for OS probes?
	readinessProbe := osConfig.Probes.Readiness.DeepCopy()
	startupProbe := osConfig.Probes.Startup.DeepCopy()

	// Security Context - common type.
	containerSecurityContext := osConfig.ContainerSecurityContext // Pointer

	container := corev1.Container{
		Name:            containerName,
		Image:           imageName,
		ImagePullPolicy: imagePullPolicy,
		Command:         command,
		Args:            args,
		Ports:           containerPorts,           // K8s container ports list
		Env:             envVars,                  // K8s env list
		Resources:       resources,                // K8s resources value struct
		LivenessProbe:   livenessProbe,            // Pointer
		ReadinessProbe:  readinessProbe,           // Pointer
		StartupProbe:    startupProbe,             // Pointer
		SecurityContext: containerSecurityContext, // Pointer
		// WorkingDir...
		// VolumeMounts - ADDED LATER BY ASSEMBLE LOGIC
	}

	// This is a complex mapping! Implement this builder helper.
	return &container, nil // Return pointer to the container spec
}

// BuildOpenSearchInitContainers builds the list of init containers for OpenSearch pods.
// Includes ensure data dirs, copy certs, plugins install, security bootstrap etc.
// Logic is specific to OS paths, versions, config.
func buildOpenSearchInitContainers(osConfig *common.OpensearchClusterConfig, dataMountPath string) ([]corev1.Container, error) {
	initContainers := []corev1.Container{}
	// Ensure base OS paths and data dirs exist (very common init step).
	// Requires Image, command to create dirs (like busybox/init image or OS init image), volume mounts for target path.
	// Need default owner UID/GID or derived from config.
	// Needs to know data Mount Path (from StorageSpec) and potentially subpath.
	// Installation Path derivation logic needed.
	// Check specific OS version/paths convention: /usr/share/opensearch or /usr/share/elasticsearch
	// Call builders.BuildEnsureDirectoryContainer or specialized OS version builder helper.
	/*
		installPath := builders.InstallPath(osConfig.Version) // Needs helper
		dataDirPath := path.Join(dataMountPath, commonutil.GetStringPtrValueOrDefault(osConfig.Storage.DataSubpath, "data")) // Needs helper commonutil.GetStringPtrValueOrDefault

		ensureDataDir := builders.BuildEnsureDirectoryContainer("init-data-dir", dataDirPath, 1000, 1000) // Need common uid/gid or config
		// Need to add volume mounts to this init container for the data path!
		ensureDataDir.VolumeMounts = []corev1.VolumeMount{{Name: osConfig.Storage.VolumeClaimTemplateName, MountPath: dataMountPath}}
		initContainers = append(initContainers, ensureDataDir)
	*/

	// Copy certificates if managed via Secrets (need specific paths and volumes).
	// Specific builder helpers needed here (e.g. BuildCopyCertificatesInitContainer).
	// Needs Secret Mount configuration details from OSConfig.

	// Install Plugins init container (if plugins managed) - often a Job or separate init container run once.
	// Requires specific OS config for plugins list. Image needed. Mounts for plugins dir.

	// Security bootstrap init container - init security state after first pod starts.
	// Requires logic specific to OS security setup and init passwords/certs/users.

	// TODO: Add more OS specific init container building logic.

	return initContainers, nil // Return built init containers list
}

// BuildOpenSearchVolumes builds the list of corev1.Volume objects for the OpenSearch Pod spec.
// Aggregates volumes needed for config files, data, logs, temp, certs etc.
// Handles mapping AppConfigData or SecurityConfig or other structures to K8s Volumes.
// DOES NOT build PVC volumes (handled by VCT) BUT needs ConfigMap/Secret/EmptyDir/HostPath etc.
func buildOpenSearchVolumes(osConfig *common.OpensearchClusterConfig, commonConfig *common.ComponentConfig) ([]corev1.Volume, error) {
	volumes := []corev1.Volume{}

	// Standard volumes derived from common config specs (if OS uses them)
	// Example: Volumes for explicit ConfigMap/Secret mounts
	volumes = append(volumes, builders.BuildVolumesFromConfigMounts(osConfig.ConfigMounts)...) // Needs common builder
	volumes = append(volumes, builders.BuildVolumesFromSecrets(osConfig.SecretMounts)...)      // Needs common builder

	// Volumes from AppConfigData (config file data in CM/Secret)
	// Need builder like BuildConfigMapsFromAppData
	// Need to decide name/convention for CM/Secret holding config file data.
	// Builder should create the CM/Secret objects themselves and also add Volume/Mount here.

	// Add common EmptyDir for logs or temporary files (optional based on config)
	// Use builders.BuildEmptyDirVolume helper if available or implement directly.
	// E.g., LogDirMountPath convention, TempDirMountPath convention.
	// If OS config supports these directly, add them to osConfig.

	// Specific OpenSearch volume needs (e.g. certificates from secrets mounted as files)
	// This requires accessing osConfig.Security structure and creating volumes based on that.
	// E.g., Create Secret Volume from Cert SecretRef in OS config.

	// Add user-provided AdditionalVolumes from common.AdditionalVolumes if supported and present.
	// if osConfig.AdditionalVolumes != nil && len(osConfig.AdditionalVolumes) > 0 {
	//     volumes = append(volumes, builders.BuildVolumesFromAdditionalVolumes(osConfig.AdditionalVolumes)...) // Needs common helper
	// }

	return volumes, nil // Return the list of volumes
}

// buildOpenSearchVolumeMounts builds the list of corev1.VolumeMount objects for OpenSearch containers.
// Aggregates mounts from various sources. Needs to match volumes by name.
// Main container needs mounts for data, logs, config, plugins, certs.
func buildOpenSearchVolumeMounts(osConfig *common.OpensearchClusterConfig) ([]corev1.VolumeMount, error) {
	allVolumeMounts := []corev1.VolumeMount{}
	// Common mounts from config (if applicable)
	allVolumeMounts = append(allVolumeMounts, builders.BuildVolumeMountsFromConfigMaps(osConfig.ConfigMounts)...)
	allVolumeMounts = append(allVolumeMounts, builders.BuildVolumeMountsFromSecrets(osConfig.SecretMounts)...)
	if osConfig.VolumeMounts != nil && len(osConfig.VolumeMounts) > 0 { // Explicit raw mounts
		allVolumeMounts = append(allVolumeMounts, osConfig.VolumeMounts...) // Assuming slice copy is enough or needs DeepCopy
	}

	// Mounts for Storage (per-replica data volume)
	if osConfig.Storage != nil && osConfig.Storage.Enabled {
		allVolumeMounts = append(allVolumeMounts, builders.BuildVolumeMountsFromStorage(osConfig.Storage)...)
	}

	// Specific OS mounts based on config (data paths, logs paths, config files, plugins, certs).
	// These use standard K8s mount structures but with OS-specific paths and volume names derived from builders/config.
	// Example: Data Path mount derived from Storage mount. Logs path might be different. Config files at a specific location.
	// certificates at another.
	// Use paths derived based on Install Path and roles.
	// Add specific corev1.VolumeMount entries here for standard OS directories and files.

	// This is complex and highly dependent on OpenSearch image conventions and config structure mapping.
	return allVolumeMounts, nil // Return aggregated mounts
}
