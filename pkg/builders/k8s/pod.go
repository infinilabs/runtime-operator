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

// pkg/builders/k8s/pod.go
package k8s

import (
	"fmt" // For potential error formatting

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
