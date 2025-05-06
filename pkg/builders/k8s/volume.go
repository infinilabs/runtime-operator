// pkg/builders/k8s/volume.go
package k8s

import (
	"fmt"
	"path" // Needed for path joining

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Needed for Quantity
	// *** ADD THIS IMPORT ***
	"github.com/infinilabs/operator/pkg/apis/common" // Import common types
	// *** END ADD IMPORT ***

	commonutil "github.com/infinilabs/operator/pkg/apis/common/util" // Common utils
)

// BuildVolumesFromConfigMaps builds corev1.Volume slices for ConfigMaps from common.ConfigMapMountSpec slices.
func BuildVolumesFromConfigMaps(configMounts []common.ConfigMountSpec) []corev1.Volume { // Uses common.ConfigMapMountSpec
	if configMounts == nil || len(configMounts) == 0 {
		return []corev1.Volume{}
	}

	volumes := make([]corev1.Volume, 0, len(configMounts))
	volumeNames := make(map[string]bool)

	for _, m := range configMounts {
		volumeName := m.VolumeName
		if volumeName == "" {
			volumeName = m.Name
		}

		if _, exists := volumeNames[volumeName]; exists {
			continue
		}
		volumeNames[volumeName] = true

		volumes = append(volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: m.Name},
					Items:                m.Items,
				},
			},
		})
	}
	return volumes
}

// BuildVolumeMountsFromConfigMaps builds corev1.VolumeMount slices for containers from common.ConfigMapMountSpec slices.
func BuildVolumeMountsFromConfigMaps(configMounts []common.ConfigMountSpec) []corev1.VolumeMount { // Uses common.ConfigMapMountSpec
	if configMounts == nil || len(configMounts) == 0 {
		return []corev1.VolumeMount{}
	}

	volumeMounts := make([]corev1.VolumeMount, 0, len(configMounts))
	for _, m := range configMounts {
		readOnly := commonutil.GetBoolValueOrDefault(m.ReadOnly, true)
		volumeName := m.VolumeName
		if volumeName == "" {
			volumeName = m.Name
		}

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: m.MountPath,
			SubPath:   commonutil.GetStringValueOrDefault(m.SubPath, ""),
			ReadOnly:  readOnly,
		})
	}
	return volumeMounts
}

// BuildVolumesFromSecrets builds corev1.Volume slices for Secrets from common.SecretMountSpec slices.
func BuildVolumesFromSecrets(secretMounts []common.SecretMountSpec) []corev1.Volume { // Uses common.SecretMountSpec
	if secretMounts == nil || len(secretMounts) == 0 {
		return []corev1.Volume{}
	}

	volumes := make([]corev1.Volume, 0, len(secretMounts))
	volumeNames := make(map[string]bool)

	for _, m := range secretMounts {
		volumeName := m.VolumeName
		if volumeName == "" {
			volumeName = m.SecretName
		}

		if _, exists := volumeNames[volumeName]; exists {
			continue
		}
		volumeNames[volumeName] = true

		volumes = append(volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: m.SecretName,
					Items:      m.Items,
				},
			},
		})
	}
	return volumes
}

// BuildVolumeMountsFromSecrets builds corev1.VolumeMount slices for containers from common.SecretMountSpec slices.
func BuildVolumeMountsFromSecrets(secretMounts []common.SecretMountSpec) []corev1.VolumeMount { // Uses common.SecretMountSpec
	if secretMounts == nil || len(secretMounts) == 0 {
		return []corev1.VolumeMount{}
	}

	volumeMounts := make([]corev1.VolumeMount, 0, len(secretMounts))
	for _, m := range secretMounts {
		readOnly := commonutil.GetBoolValueOrDefault(m.ReadOnly, true)
		volumeName := m.VolumeName
		if volumeName == "" {
			volumeName = m.SecretName
		}

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: m.MountPath,
			ReadOnly:  readOnly,
		})
	}
	return volumeMounts
}

// BuildVolumesFromAdditionalVolumes implementation... (Keep as before)
func BuildVolumesFromAdditionalVolumes(additionalVolumes []corev1.Volume) []corev1.Volume {
	if additionalVolumes == nil || len(additionalVolumes) == 0 {
		return []corev1.Volume{}
	}
	builtVolumes := make([]corev1.Volume, 0, len(additionalVolumes))
	for i := range additionalVolumes {
		builtVolumes = append(builtVolumes, *additionalVolumes[i].DeepCopy())
	}
	return builtVolumes
}

// BuildVolumeMountsFromAdditionalVolumes implementation... (Keep as before)
func BuildVolumeMountsFromAdditionalVolumes(additionalVolumeMounts []corev1.VolumeMount) []corev1.VolumeMount {
	if additionalVolumeMounts == nil || len(additionalVolumeMounts) == 0 {
		return []corev1.VolumeMount{}
	}
	builtMounts := make([]corev1.VolumeMount, 0, len(additionalVolumeMounts))
	for i := range additionalVolumeMounts {
		builtMounts = append(builtMounts, *additionalVolumeMounts[i].DeepCopy())
	}
	return builtMounts
}

// BuildVolumeClaimTemplates builds PersistentVolumeClaim templates for StatefulSet.
func BuildVolumeClaimTemplates(storageSpec *common.StorageSpec, commonLabels map[string]string) ([]corev1.PersistentVolumeClaim, error) { // Uses common.StorageSpec
	if storageSpec == nil || !storageSpec.Enabled {
		return []corev1.PersistentVolumeClaim{}, nil
	}
	if storageSpec.Size == nil {
		return nil, fmt.Errorf("storage is enabled but required field 'size' is missing")
	}
	vctName := storageSpec.VolumeClaimTemplateName
	if vctName == "" {
		vctName = "data"
	}
	if storageSpec.MountPath == "" {
		return nil, fmt.Errorf("storage is enabled but required field 'mountPath' is missing")
	}

	pvcTemplateSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: storageSpec.AccessModes, // Value slice
		Resources: corev1.VolumeResourceRequirements{ // Use VolumeResourceRequirements struct
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: *storageSpec.Size, // Assign storage size to ResourceStorage key
			},
			// Limits can also be set if needed:
			// Limits: corev1.ResourceList{ corev1.ResourceStorage: *storageSpec.Size },
		},
		StorageClassName: storageSpec.StorageClassName, // Pointer
		VolumeMode:       nil,                          // Default Filesystem (can be made configurable)
	}
	if len(pvcTemplateSpec.AccessModes) == 0 {
		pvcTemplateSpec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}

	pvcTemplate := corev1.PersistentVolumeClaim{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "PersistentVolumeClaim"},
		ObjectMeta: metav1.ObjectMeta{Name: vctName, Labels: commonLabels},
		Spec:       pvcTemplateSpec,
	}
	return []corev1.PersistentVolumeClaim{pvcTemplate}, nil
}

// BuildSharedPVCPVC builds a single corev1.PersistentVolumeClaim object for a shared PVC (Deployment).
func BuildSharedPVCPVC(persistenceConfig *common.PersistenceSpec, instanceName string, namespace string, commonLabels map[string]string) (*corev1.PersistentVolumeClaim, error) { // Uses common.PersistenceSpec
	if persistenceConfig == nil || !persistenceConfig.Enabled {
		return nil, nil
	}
	if persistenceConfig.Size == nil {
		return nil, fmt.Errorf("persistence is enabled but required field 'size' is missing for instance %s/%s", namespace, instanceName)
	}
	if persistenceConfig.MountPath == "" {
		return nil, fmt.Errorf("persistence is enabled but required field 'mountPath' is missing for instance %s/%s", namespace, instanceName)
	} // MountPath needed conceptually
	if persistenceConfig.VolumeName == "" {
		persistenceConfig.VolumeName = DeriveResourceName(instanceName) + "-pvc-vol"
	} // Set default if empty

	pvcResourceName := DeriveResourceName(instanceName) + "-pvc" // Convention

	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: persistenceConfig.AccessModes,
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: *persistenceConfig.Size,
			},
		},
		StorageClassName: persistenceConfig.StorageClassName,
	}

	if len(pvcSpec.AccessModes) == 0 {
		pvcSpec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "PersistentVolumeClaim"},
		ObjectMeta: BuildObjectMeta(pvcResourceName, namespace, commonLabels, nil), // Use generic helper
		Spec:       pvcSpec,
	}
	return pvc, nil
}

// BuildPersistentVolumeMounts builds VolumeMounts derived from PersistenceSpec.
func BuildPersistentVolumeMounts(persistenceConfig *common.PersistenceSpec, volumeName string) []corev1.VolumeMount { // Uses common.PersistenceSpec
	if persistenceConfig == nil || !persistenceConfig.Enabled || persistenceConfig.MountPath == "" || volumeName == "" {
		return []corev1.VolumeMount{} // Return empty if disabled or required fields missing
	}
	return []corev1.VolumeMount{{
		Name:      volumeName, // Use the provided Volume Name
		MountPath: persistenceConfig.MountPath,
		ReadOnly:  false, // Default R/W
	}}
}

// BuildVolumeMountsFromStorage builds VolumeMounts derived from StorageSpec (for StatefulSet VCT).
func BuildVolumeMountsFromStorage(storageSpec *common.StorageSpec) []corev1.VolumeMount { // Uses common.StorageSpec
	if storageSpec == nil || !storageSpec.Enabled || storageSpec.MountPath == "" || storageSpec.VolumeClaimTemplateName == "" {
		return []corev1.VolumeMount{} // Return empty if disabled or required fields missing
	}

	mounts := []corev1.VolumeMount{{
		Name:      storageSpec.VolumeClaimTemplateName, // Mount the VCT volume by its name
		MountPath: storageSpec.MountPath,
		ReadOnly:  false, // Default R/W
	}}

	// Add subpath mount if specified
	if storageSpec.DataSubpath != nil && *storageSpec.DataSubpath != "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      storageSpec.VolumeClaimTemplateName,                        // Reference same volume
			MountPath: path.Join(storageSpec.MountPath, *storageSpec.DataSubpath), // Specific path
			SubPath:   *storageSpec.DataSubpath,                                   // Specify the subpath within the volume
			ReadOnly:  false,
		})
	}

	return mounts
}
