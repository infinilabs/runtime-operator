// pkg/builders/k8s/pod.go
package k8s

import (
	"fmt" // For potential error formatting

	"github.com/infinilabs/operator/pkg/apis/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// Needed for Scheme if passed
	// common "github.com/infinilabs/operator/pkg/apis/common" // Use specific K8s types as input where possible
)

// BuildPodTemplateSpec builds a corev1.PodTemplateSpec from assembled K8s spec parts and metadata.
// This generic builder *assembles* the final PodTemplateSpec. It expects the caller (app-specific builder)
// to provide fully constructed lists of containers, init containers, volumes, and resolved PodSpec fields.
func BuildPodTemplateSpec(
	// Input lists of core Pod/Container specs that are fully built externally.
	containers []corev1.Container, // List of containers (main + potential sidecars)
	initContainers []corev1.Container, // List of init containers
	volumes []corev1.Volume, // List of volumes (ConfigMap, Secret, EmptyDir, HostPath, PVC for Deployment)

	// Input common PodSpec fields (pre-processed from config)
	podSecurityContext *corev1.PodSecurityContext, // Pod Security Context (pointer)
	serviceAccountName string, // Service Account name string
	nodeSelector map[string]string, // Pod Node Selector (map value type)
	tolerations []corev1.Toleration, // Pod Tolerations (slice value type)
	affinity *corev1.Affinity, // Pod Affinity (pointer to K8s struct)
	// Add other common PodSpec fields if needed
	// hostAliases []corev1.HostAlias,
	// dnsPolicy corev1.DNSPolicy,
	// imagePullSecrets []corev1.LocalObjectReference,

	// Input metadata fields
	podLabels map[string]string, // Labels for the Pod template metadata (selector + common)
	podAnnotations map[string]string, // Annotations for the Pod template metadata (optional)

) (*corev1.PodTemplateSpec, error) { // Return pointer and error

	// Basic validation on inputs (optional, caller should ensure validity)
	if len(containers) == 0 {
		return nil, fmt.Errorf("pod template spec requires at least one container")
	}
	// Ensure volume names referenced by container mounts exist in the volumes list (complex check, usually validated by K8s API server).

	// --- Assemble Pod Spec ---
	podSpec := corev1.PodSpec{
		// Assign standard K8s lists/structs/fields directly
		InitContainers: initContainers, // Assign slice (can be nil or empty)
		Containers:     containers,     // Assign slice (must be non-empty)
		Volumes:        volumes,        // Assign slice (can be nil or empty)

		ServiceAccountName: serviceAccountName, // Assign string
		SecurityContext:    podSecurityContext, // Assign pointer directly
		NodeSelector:       nodeSelector,       // Assign map directly
		Tolerations:        tolerations,        // Assign slice directly
		Affinity:           affinity,           // Assign pointer directly

		// Assign other standard fields if passed in
		// HostAliases: hostAliases,
		// DNSPolicy: dnsPolicy,
		// ImagePullSecrets: imagePullSecrets,

		// Use default RestartPolicy for Deployment/StatefulSet Pods
		RestartPolicy: corev1.RestartPolicyAlways,
	}

	// --- Assemble Pod Template Spec ---
	podTemplate := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      podLabels,      // Set Pod labels
			Annotations: podAnnotations, // Set Annotations if provided
		},
		Spec: podSpec, // Assign the built pod spec
	}

	return podTemplate, nil // Return the built Pod Template Spec pointer
}

// BuildMainContainerSpec builds the specification for the primary application container (corev1.Container).
// This helper is called by application-specific builders.
// It maps standard common config fields OR direct K8s types to corev1.Container spec fields.
// VolumeMounts list is PASSED IN, aggregated by the caller.
func BuildMainContainerSpec(
	// --- Basic Container Info ---
	containerName string, // Derived name (e.g., "gateway", "opensearch")
	image common.ImageSpec, // Common ImageSpec struct (value type)
	command []string, // Optional Command override slice
	args []string, // Optional Args slice
	workingDir string, // Optional Working Directory string

	// --- Container Configuration ---
	ports []common.PortSpec, // Slice of common PortSpec
	env []corev1.EnvVar, // Slice of K8s EnvVar (value type)
	envFrom []corev1.EnvFromSource, // Slice of K8s EnvFromSource (value type)
	resources *common.ResourcesSpec, // Pointer to common ResourcesSpec
	volumeMounts []corev1.VolumeMount, // FULL list of volume mounts for THIS container

	// --- Probes & Security (Pointers to K8s types) ---
	probes *common.ProbesConfig, // Pointer to common ProbesConfig (containing *corev1.Probe)
	securityContext *corev1.SecurityContext, // Pointer to K8s SecurityContext

) (corev1.Container, error) { // Return value type and error

	// Basic validation on required inputs
	if image.Repository == "" && image.Tag == "" {
		return corev1.Container{}, fmt.Errorf("image repository and tag cannot both be empty for main container %s", containerName)
	}
	if containerName == "" {
		return corev1.Container{}, fmt.Errorf("main container name cannot be empty")
	}

	// --- Build K8s structs from common inputs using helpers ---
	imageName := BuildImageName(image.Repository, image.Tag)           // Use common helper, pass value type
	imagePullPolicy := GetImagePullPolicy(image.PullPolicy, image.Tag) // Use common helper
	resourcesSpec := BuildK8sResourceRequirements(resources)           // Use common helper, returns value struct

	// *** FIX: Call helper to build K8s ports and assign to variable ***
	k8sContainerPorts := BuildContainerPorts(ports) // Call the helper

	// Handle probes (directly use K8s pointers from common.ProbesConfig if non-nil)
	var livenessProbe, readinessProbe, startupProbe *corev1.Probe
	if probes != nil {
		// Use the BuildProbe helper which handles nil and potentially defaults/copies
		livenessProbe = BuildProbe(probes.Liveness)
		readinessProbe = BuildProbe(probes.Readiness)
		startupProbe = BuildProbe(probes.Startup)
	}

	// Build the corev1.Container struct
	container := corev1.Container{
		Name:            containerName,
		Image:           imageName,
		ImagePullPolicy: imagePullPolicy,

		Command:    command,    // Assign provided slice (can be nil)
		Args:       args,       // Assign provided slice (can be nil)
		WorkingDir: workingDir, // Assign provided string (can be empty)

		Ports: k8sContainerPorts, // *** FIX: Use the built K8s ports list ***

		Env:     env,     // Use K8s EnvVar slice directly
		EnvFrom: envFrom, // Use K8s EnvFromSource slice directly

		Resources: resourcesSpec, // Use K8s Resources value struct

		VolumeMounts: volumeMounts, // Assign the pre-built list of ALL mounts for this container

		LivenessProbe:  livenessProbe,  // Pointer to K8s Probe
		ReadinessProbe: readinessProbe, // Pointer
		StartupProbe:   startupProbe,   // Pointer

		SecurityContext: securityContext, // Pointer passed in
	}

	return container, nil // Return the built container spec (value type)
}

// BuildEnsureDirectoryContainer builds a standard init container spec to ensure a directory exists with correct permissions.
// name: name of the init container.
// image: image to use for the init container (e.g., "busybox:latest").
// path: the path to ensure exists within the container.
// ownerUID, ownerGID: numeric owner ID/GID for the directory.
// Returns a corev1.Container spec. Volume mounts for the target path must be added separately by the caller.
func BuildEnsureDirectoryContainer(name string, image string, path string, ownerUID int64, ownerGID int64) corev1.Container {
	// Use sh -c for the command. Ensure path is quoted correctly for shell.
	// Use chown to set ownership. Use mkdir -p to create parents if needed.
	// Use exit 0 to ensure success unless command fails.
	command := fmt.Sprintf("echo 'Ensuring directory exists: %s'; mkdir -p '%s'; chown %d:%d '%s'; echo 'Directory setup complete.'; exit 0", path, path, ownerUID, ownerGID, path)

	// Default image if not provided
	initImage := image
	if initImage == "" {
		initImage = "busybox:latest" // Common default
	}

	return corev1.Container{
		Name:            name, // e.g., "init-data-dir"
		Image:           initImage,
		ImagePullPolicy: corev1.PullIfNotPresent, // Good default for utility images
		Command:         []string{"sh", "-c"},
		Args:            []string{command},
		// VolumeMounts: Must be added by the caller to mount the volume where 'path' resides.
		Resources: corev1.ResourceRequirements{ // Minimal resources for init container
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("20Mi"),
			},
			Limits: corev1.ResourceList{ // Optional limits
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
		// SecurityContext: // Optional: Set RunAsUser/Group if needed to match chown or permissions.
	}
}
