// pkg/builders/k8s/pod.go
package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Needed for Scheme (optional)
	// Need to access common types for building main container etc.
	"github.com/infinilabs/operator/pkg/apis/common" // Import common types if main container build logic is here
)

// BuildPodTemplateSpec builds a corev1.PodTemplateSpec from assembled K8s spec parts and common labels.
// This builder is called by higher-level builders (like BuildDeploymentResources/BuildStatefulSetResources)
// after they have prepared all the necessary lists and fields.
func BuildPodTemplateSpec(
	// Input lists of core Pod/Container specs that are fully built externally.
	mainContainer corev1.Container, // The primary application container spec (should have VolumeMounts attached)
	initContainers []corev1.Container, // List of init containers specs
	volumes []corev1.Volume, // List of Volume specs (including CM, Secret, EmptyDir, HostPath)

	// Input explicit PodSpec fields from config (already processed and default-applied by caller)
	podSecurityContext *corev1.PodSecurityContext, // Pointer
	serviceAccountName string, // Service Account name string
	nodeSelector map[string]string, // Node Selector map
	tolerations []corev1.Toleration, // Tolerations slice
	affinity *corev1.Affinity, // Affinity pointer

	// Input metadata fields
	podLabels map[string]string, // Labels for the Pod metadata (selector and common labels)
	podAnnotations map[string]string, // Annotations for the Pod metadata

) (*corev1.PodTemplateSpec, error) {
	// Assemble the core Pod Spec using the provided lists and fields.
	podSpec := corev1.PodSpec{
		InitContainers: initContainers,                    // Assign list
		Containers:     []corev1.Container{mainContainer}, // Create list with the main container
		Volumes:        volumes,                           // Assign list

		ServiceAccountName: serviceAccountName, // Assign name string
		SecurityContext:    podSecurityContext, // Assign pointer

		NodeSelector: nodeSelector, // Assign map
		Tolerations:  tolerations,  // Assign slice
		Affinity:     affinity,     // Assign pointer

		// Add other standard fields from K8s PodSpec if they are supported in common.types
		// and passed as inputs to this builder.
		// HostAliases: hostAliases, // If added as input
		// DNSPolicy: dnsPolicy,   // If added as input

		// Restart Policy is usually determined by the workload type (Deployment vs StatefulSet vs Job).
		// It should NOT be set in the PodTemplateSpec itself usually, but by the Deployment/StatefulSet Spec.
		// If RestartPolicy needs to be set at the Pod level (rare), add as input and set here.
		// RestartPolicy:
	}

	// Assemble the final Pod Template Spec structure.
	podTemplate := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      podLabels,      // Set Pod labels
			Annotations: podAnnotations, // Set Annotations if provided
		},
		Spec: podSpec, // Set the built pod spec
	}

	// Return the built Pod Template Spec pointer.
	return podTemplate, nil
}

// BuildMainContainerSpec builds the specification for the primary application container (corev1.Container).
// It maps standard common config fields to K8s corev1.Container spec fields.
// Application-specific builders should call this helper or implement similar logic directly.
// VolumeMounts list for the container is built by the caller AFTER aggregating mounts from all sources.
func BuildMainContainerSpec(
	// --- Standard Container Fields ---
	containerName string,
	image common.ImageSpec, // common type, not pointer (builder ensures exists)
	resources *common.ResourcesSpec, // pointer, use helper to get default/value
	ports []common.PortSpec, // slice, use helper to build k8s ports
	env []corev1.EnvVar, // K8s slice (raw copy assumed or use builder to append/merge)
	envFrom []corev1.EnvFromSource, // K8s slice
	probesConfig *common.ProbesConfig, // pointer, use helpers to build K8s probes
	securityContext *corev1.SecurityContext, // K8s pointer

	// --- Optional Container Fields (less common) ---
	command []string, // Command string slice
	args []string, // Args string slice
	workingDir string, // WorkingDir string

	// --- App-Specific Container Logic Indicators ---
	// These are NOT fields to copy, but indicators used by app-specific builder
	// E.g. if OpenSearch role=data, configure X. If role=master, configure Y.
	// Pass app-specific config struct itself or specific flags if builder needs it.
	// appSpecificConfig interface{}, // Could pass the raw config if needed internally

) corev1.Container { // Return value type for a single container struct
	// No error returned by this builder assuming inputs are correct structure.
	// Validation/Error handling should be done BEFORE calling this.

	// Get defaulted values for optional fields using helpers
	resourcesSpec := GetResourcesSpecOrDefault(resources) // common helper returns value struct

	// Build K8s core types based on common types slices
	k8sContainerPorts := BuildContainerPorts(ports) // helpers.go function

	// Build K8s Probe structs from ProbesConfig (assuming common.ProbesConfig uses *corev1.Probe)
	// Call BuildProbe for each probe if necessary (if defaults/conventions need applying)
	// Or if *corev1.Probe in common is directly passed as input.
	// Based on common.types.go, ProbesConfig HAS pointers to corev1.Probe.
	livenessProbe := BuildProbe(probesConfig.Liveness) // Uses builder helper for corev1.Probe
	readinessProbe := BuildProbe(probesConfig.Readiness)
	startupProbe := BuildProbe(probesConfig.Startup)

	// Build the corev1.Container struct
	container := corev1.Container{
		Name:            containerName,                                 // Name passed in
		Image:           BuildImageName(&image),                        // Use common helper, pass pointer to value
		ImagePullPolicy: GetImagePullPolicyOrDefault(image.PullPolicy), // Use common helper

		Command:    command,    // Use provided command slice
		Args:       args,       // Use provided args slice
		WorkingDir: workingDir, // Use provided working dir

		Ports: k8sContainerPorts, // Use built K8s ports list

		Env:     env,     // Use K8s EnvVar slice provided
		EnvFrom: envFrom, // Use K8s EnvFromSource slice provided

		Resources:       resourcesSpec,   // Use K8s Resources value struct (with defaults)
		SecurityContext: securityContext, // Pointer passed in

		LivenessProbe:  livenessProbe,  // Pointer to K8s Probe
		ReadinessProbe: readinessProbe, // Pointer
		StartupProbe:   startupProbe,   // Pointer

		// VolumeMounts - Are added to the container spec *later* by the assembler logic
		// that iterates through ALL sources of mounts.
		// This builder DOES NOT set VolumeMounts.
	}

	return container // Return value type
}
