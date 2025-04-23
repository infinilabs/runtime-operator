// pkg/builders/k8s/volume.go
package k8s

import (
	"path" // Needed for path joining

	// Might be needed if volume builders interact with StatefulSet/Deployment details implicitly
	corev1 "k8s.io/api/core/v1"

	// App types
	"github.com/infinilabs/operator/pkg/apis/common"                 // Common config types
	commonutil "github.com/infinilabs/operator/pkg/apis/common/util" // Common utils
	"github.com/infinilabs/operator/pkg/builders"
	// If builders interact with client
)

// This file contains helpers for building corev1.Volume and corev1.VolumeMount slices.
// App-specific builders will call these after mapping their config to common types.

// BuildVolumesFromConfigMounts builds corev1.Volume slices for ConfigMaps from common.ConfigMapMountSpec slices.
// These volumes need to be added to the Pod Spec Volumes list.
func BuildVolumesFromConfigMounts(configMounts []common.ConfigMountSpec) []corev1.Volume {
	if configMounts == nil || len(configMounts) == 0 {
		return []corev1.Volume{}
	}

	volumes := make([]corev1.Volume, 0, len(configMounts))
	for _, m := range configMounts { // Iterate through value copy of struct
		volumeName := m.VolumeName // Use provided volume name
		if volumeName == "" {      // Default if not provided (e.g. CM name)
			volumeName = m.Name // Default Volume Name to ConfigMap name
			// Potentially check uniqueness or add hashing for safety if names collide
		}

		volumes = append(volumes, corev1.Volume{
			Name: volumeName, // Volume name must match the mount name in PodSpec.Containers[].VolumeMounts
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: m.Name}, // ConfigMap resource name
					Items:                m.Items,                                   // Optional list of key-to-path, can be nil or empty slice. K8s default depends on if items is provided.
					// DefaultMode: // DefaultMode for file permissions, could add to common.ConfigMountSpec
				},
			},
		})
	}
	return volumes
}

// BuildVolumeMountsFromConfigMaps builds corev1.VolumeMount slices for main containers from common.ConfigMapMountSpec slices.
// These volume mounts reference volumes created based on common.ConfigMountSpec.
func BuildVolumeMountsFromConfigMaps(configMounts []common.ConfigMountSpec) []corev1.VolumeMount {
	if configMounts == nil || len(configMounts) == 0 {
		return []corev1.VolumeMount{}
	}

	volumeMounts := make([]corev1.VolumeMount, 0, len(configMounts))
	for _, m := range configMounts { // Iterate through value copy of struct
		readOnly := commonutil.GetBoolPtrValueOrDefault(m.ReadOnly, true) // Use helper for pointer with default
		volumeName := m.VolumeName                                        // Use provided volume name
		if volumeName == "" {
			volumeName = m.Name
		} // Default if not provided

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,                                           // Volume name must match Volume name definition
			MountPath: m.MountPath,                                          // Destination path in container
			SubPath:   commonutil.GetStringPtrValueOrDefault(m.SubPath, ""), // Mount a single file if SubPath provided and Items not
			// Items are handled by Volume definition, SubPath handled by mount definition.
			ReadOnly: readOnly,
		})
	}
	return volumeMounts
}

// BuildVolumesFromSecrets builds corev1.Volume slices for Secrets from common.SecretMountSpec slices.
// These volumes need to be added to the Pod Spec Volumes list.
func BuildVolumesFromSecrets(secretMounts []common.SecretMountSpec) []corev1.Volume {
	if secretMounts == nil || len(secretMounts) == 0 {
		return []corev1.Volume{}
	}

	volumes := make([]corev1.Volume, 0, len(secretMounts))
	for _, m := range secretMounts { // Iterate through value copy of struct
		volumeName := m.VolumeName // Use provided volume name
		if volumeName == "" {
			volumeName = m.SecretName
		} // Default if not provided

		volumes = append(volumes, corev1.Volume{
			Name: volumeName, // Volume name must match the mount name
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: m.SecretName, // Secret resource name
					Items:      m.Items,      // Optional
					// DefaultMode: // DefaultMode
				},
			},
		})
	}
	return volumes
}

// BuildVolumeMountsFromSecrets builds corev1.VolumeMount slices for main containers from common.SecretMountSpec slices.
// Returns a slice of K8s VolumeMounts.
func BuildVolumeMountsFromSecrets(secretMounts []common.SecretMountSpec) []corev1.VolumeMount {
	if secretMounts == nil || len(secretMounts) == 0 {
		return []corev1.VolumeMount{}
	}

	volumeMounts := make([]corev1.VolumeMount, 0, len(secretMounts))
	for _, m := range secretMounts { // Iterate through value copy of struct
		readOnly := commonutil.GetBoolPtrValueOrDefault(m.ReadOnly, true)
		volumeName := m.VolumeName // Use provided volume name
		if volumeName == "" {
			volumeName = m.SecretName
		} // Default if not provided

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName, // Volume name must match Volume name definition
			MountPath: m.MountPath,
			// SubPath is not typical for SecretMounts but possible via items[].path (handled in Volume definition)
			ReadOnly: readOnly,
		})
	}
	return volumeMounts
}

// BuildVolumeMountsFromPersistence builds corev1.VolumeMount slices from common.PersistenceSpec (for Deployment shared PVC).
func BuildVolumeMountsFromPersistence(persistenceConfig *common.PersistenceSpec) []corev1.VolumeMount {
	if persistenceConfig == nil || !persistenceConfig.Enabled {
		return []corev1.VolumeMount{}
	} // Only build if enabled

	// Ensure essential fields are present if enabled (Size handled during PVC object building validation)
	volumeName := persistenceConfig.VolumeName
	if volumeName == "" { // Default volume name
		volumeName = builders.DeriveResourceName("todo") + "-pvc-vol" // Requires base instance name
		// Builder of Deployment should pass the correct instance name or base name.
		// Adjust this builder helper signature or rely on caller providing resolved VolumeName
	}
	mountPath := persistenceConfig.MountPath
	if mountPath == "" { // MountPath is required if enabled
		// Error or use a default? Validation should happen earlier.
		return []corev1.VolumeMount{} // Return empty if essential field missing (or better: error)
	}

	mounts := []corev1.VolumeMount{
		{
			Name:      volumeName, // Use derived/provided Volume Name
			MountPath: mountPath,
			ReadOnly:  false, // Default read-write for persistence
			// Optional SubPath logic if supported by PersistenceSpec
			// SubPath: GetStringPtrValueOrDefault(persistenceConfig.Subpath, ""),
		},
	}
	return mounts
}

// BuildVolumeMountsFromStorage builds corev1.VolumeMount slices from common.StorageSpec (for StatefulSet per-replica PVC).
// This only builds the *mount* definition for the main container, referencing the Volume defined by the VolumeClaimTemplate.
func BuildVolumeMountsFromStorage(storageConfig *common.StorageSpec) []corev1.VolumeMount {
	if storageConfig == nil || !storageConfig.Enabled {
		return []corev1.VolumeMount{}
	} // Only build if storage is enabled

	// Ensure essential fields are present if enabled
	// volumeClaimTemplateName and MountPath are required if storage is enabled (validate earlier)
	volumeName := storageConfig.VolumeClaimTemplateName // Name in Pod Spec VolumeMounts must match VCT name
	if volumeName == "" {
		// Validation/defaulting should happen earlier. Return empty if essential field missing.
		// Or builder receiving storageConfig handles default and validates.
		return []corev1.VolumeMount{} // Return empty if essential field missing (or error if better)
	}
	mountPath := storageConfig.MountPath
	if mountPath == "" {
		// MountPath is required if enabled.
		return []corev1.VolumeMount{} // Return empty if essential field missing (or error)
	}

	mounts := []corev1.VolumeMount{
		{
			Name:      volumeName, // Use the VolumeClaimTemplate name as the mount name
			MountPath: mountPath,  // Destination path in container
			ReadOnly:  false,      // Data volume is usually read-write
			// Optional DataSubpath logic: Add an additional volume mount using subpath
			// If common.StorageSpec has DataSubpath *string
			// SubPath field in VolumeMount applies a subpath *within* the volume.
			// K8s VolumeMount also has a SubPath field.
			// Let's apply subpath directly to the VolumeMount Path if the config combines them conceptually.
			// OR add another VolumeMount with a subpath referencing the *same volume name*.
			// Option A (Combined conceptually): The builder expects common.StorageSpec.MountPath to be the final mount path.
			// Option B (Separate concept): storage.MountPath is base, storage.DataSubpath string is optional subpath.
			// Let's follow option B as it reflects K8s API better. Add a second mount for the subpath.
		},
	}

	// Optional DataSubpath mount: Add an *additional* volume mount using subpath if specified in config
	if storageConfig.DataSubpath != nil && *storageConfig.DataSubpath != "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      storageName,                                                    // Reference the *same* Volume Name
			MountPath: path.Join(storageConfig.MountPath, *storageConfig.DataSubpath), // Path to subpath within base mount path
			ReadOnly:  false,                                                          // Usually needs writes
			// Set SubPath field in VolumeMount for subpath functionality
			SubPath: *storageConfig.DataSubpath,
		})
		// Note: If using subpath, ensure the directory itself exists. Often requires an init container.
	}

	return mounts
}

// BuildVolumesFromAdditionalVolumes builds corev1.Volume slices from a slice of raw corev1.Volume spec.
// Used for volume types like EmptyDir, HostPath etc. that don't have dedicated specs in common types.
// Assumes the input slice already contains valid K8s corev1.Volume specs provided by the user or other helpers.
// Builders need to add VolumeMounts for these volumes as well.
func BuildVolumesFromAdditionalVolumes(additionalVolumes []corev1.Volume) []corev1.Volume {
	if additionalVolumes == nil || len(additionalVolumes) == 0 {
		return []corev1.Volume{}
	}

	// Return a copy if needed to prevent external modification.
	// Since these are structs containing potentially complex pointers,
	// a simple shallow copy is not enough if inner pointers/maps/slices are mutated.
	// Using a loop + DeepCopy() for each element is safest.
	builtVolumes := make([]corev1.Volume, 0, len(additionalVolumes))
	for i := range additionalVolumes {
		builtVolumes = append(builtVolumes, *additionalVolumes[i].DeepCopy()) // Assuming corev1.Volume has DeepCopy()
	}

	return builtVolumes
}

// BuildPersistentVolumeMounts builds VolumeMounts derived from PersistenceSpec.
// This is the VolumeMount *within* a Pod that uses the *shared* PersistentVolumeClaim (for Deployment).
// Needs the resolved volume name.
func BuildPersistentVolumeMounts(persistenceConfig *common.PersistenceSpec, volumeName string) []corev1.VolumeMount {
	if persistenceConfig == nil || !persistenceConfig.Enabled {
		return []corev1.VolumeMount{}
	}

	// Ensure essential fields are present if enabled
	// MountPath is required if enabled.
	mountPath := persistenceConfig.MountPath
	if mountPath == "" {
		// MountPath is required if enabled.
		return []corev1.VolumeMount{} // Return empty if essential field missing (or error)
	}
	if volumeName == "" {
		// Volume name is required to build the mount.
		return []corev1.VolumeMount{} // Return empty if essential field missing (or error)
	}

	mounts := []corev1.VolumeMount{
		{
			Name:      volumeName, // Use the Volume Name referencing the shared PVC
			MountPath: mountPath,  // Destination path in container
			ReadOnly:  false,      // Default read-write for persistence
			// Optional SubPath logic if supported by PersistenceSpec (less common for shared PVC)
			// SubPath: GetStringPtrValueOrDefault(persistenceConfig.Subpath, ""), // If added to PersistenceSpec
		},
	}
	return mounts
}
