// pkg/builders/k8s/helpers.go
// Package builders contains functions to construct Kubernetes resources. helpers.go contains general builder utilities.
package k8s

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // Needed for ResourceList copy

	// Needed for GroupKind comparison potentially
	"k8s.io/apimachinery/pkg/util/intstr" // Needed for IntOrString

	// Removed direct import of common.types here to avoid type coupling, use standard K8s types as inputs
	// common "github.com/infinilabs/operator/pkg/apis/common"

	// Using common.util only for pointer helpers if necessary, avoid direct common.types dependency.
	"github.com/infinilabs/operator/pkg/apis/common"
	commonutil "github.com/infinilabs/operator/pkg/apis/common/util" // For GetInt32ValueOrDefault etc.
)

const (
	managedByLabel    = "app.kubernetes.io/managed-by"
	operatorName      = "infini-operator" // Field owner for SSA, ManagedBy label value
	appNameLabel      = "app.infini.cloud/application-name"
	compNameLabel     = "app.infini.cloud/component-name"     // Component Type from CompDef (Name)
	compInstanceLabel = "app.infini.cloud/component-instance" // Component Name from AppDef
)

// BuildCommonLabels creates a map of common labels for Kubernetes resources managed by the operator.
// It requires context about the application, component type (CompDef Name), and component instance name (AppComp Name).
func BuildCommonLabels(appName string, compType string, instanceName string) map[string]string {
	// Standard labels + specific labels
	labels := map[string]string{
		managedByLabel:               operatorName,
		"app.kubernetes.io/name":     compType,     // Component Type as app name
		"app.kubernetes.io/instance": instanceName, // Component Instance as instance name
		appNameLabel:                 appName,      // Full App Name
		compNameLabel:                compType,     // Redundant but specific Component Type label
		compInstanceLabel:            instanceName, // Redundant with io/instance but specific Component Instance label
	}
	return labels
}

// BuildSelectorLabels creates a map of labels commonly used for workload selectors (Deployment, StatefulSet)
// to target the pods belonging to a specific component instance.
// This must match the labels applied to the Pod template metadata for selection to work.
func BuildSelectorLabels(instanceName string) map[string]string {
	// A common selector uses the component instance label for uniqueness.
	// Include app.kubernetes.io/instance as per common convention.
	return map[string]string{
		compInstanceLabel:            instanceName, // Essential for selecting Pods of THIS instance
		"app.kubernetes.io/instance": instanceName, // Standard convention
	}
}

// DeriveResourceName generates a consistent name for primary Kubernetes resources
// managed by a component instance (e.g., Deployment, StatefulSet, main Service, main ConfigMap/Secret).
// K8s resource names must be lowercase DNS subdomain safe.
func DeriveResourceName(instanceName string) string {
	// Use the component instance name directly as it's already constrained and unique within AppDef.
	return strings.ToLower(instanceName)
}

// DeriveContainerName generates a consistent name for the primary container within a Pod template.
// Often uses the component type name or a generic "app". Lowercase and DNS-safe.
func DeriveContainerName(compType string) string {
	name := strings.ToLower(strings.ReplaceAll(compType, " ", "-"))
	if len(name) > 63 { // Enforce K8s max name length for DNS_LABEL
		name = name[:63]
	}
	// Ensure it starts and ends with alphanumeric
	if !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= '0' && name[0] <= '9')) {
		name = "c-" + name // Add a prefix if starts with invalid character (like '-')
	}
	if !((name[len(name)-1] >= 'a' && name[len(name)-1] <= 'z') || (name[len(name)-1] >= '0' && name[len(name)-1] <= '9')) {
		name = name + "-c" // Add a suffix if ends with invalid character
	}
	// Add more robust validation/sanitization if necessary

	return name
}

// BuildObjectMeta builds standard Kubernetes ObjectMeta.
// Returns a K8s ObjectMeta struct.
func BuildObjectMeta(name string, namespace string, labels map[string]string, annotations map[string]string) metav1.ObjectMeta {
	// ownerReference is set AFTER building using controllerutil.SetOwnerReference by the caller (e.g. ApplyTask).
	return metav1.ObjectMeta{
		Name:        name,        // Provided name (derived using DeriveResourceName)
		Namespace:   namespace,   // Provided namespace (from AppDef)
		Labels:      labels,      // Use provided labels (built using BuildCommonLabels and other sources)
		Annotations: annotations, // Use provided annotations (if any)
	}
}

// BuildImageName constructs the full container image string from ImageSpec (Repository:Tag).
// Expects common.ImageSpec as input. Returns a string.
func BuildImageName(imageSpec common.ImageSpec) string { // Accepts value type
	// Repository is the base image name.
	image := imageSpec.Repository
	if image == "" {
		// Log error/warning? Image should typically have repository if this is called.
		return "" // Repository is essential.
	}

	// Append Tag if provided.
	if imageSpec.Tag != "" {
		image = fmt.Sprintf("%s:%s", image, imageSpec.Tag)
	}
	// If no Tag is provided, the image string is "repository", which K8s default tag ':latest'.
	// The image pull policy is crucial in this case.
	return image
}

// GetImagePullPolicyOrDefault returns the ImagePullPolicy from common.ImageSpec or a standard default.
// Use this when configuring the container spec.
// K8s standard default: IfNotPresent unless tag is :latest. This provides a fixed default or allows the K8s default if specified as "".
func GetImagePullPolicyOrDefault(policy corev1.PullPolicy, imageTag string) corev1.PullPolicy {
	if policy != "" {
		return policy
	} // Use provided policy if set
	// Apply K8s default logic based on tag if no policy is specified.
	if strings.ToLower(imageTag) == "latest" { // Case-insensitive compare for tag
		return corev1.PullAlways // Default is Always if tag is "latest"
	}
	return corev1.PullIfNotPresent // Default is IfNotPresent otherwise
}

// GetResourcesSpecOrDefault returns the K8s corev1.ResourceRequirements struct from common.ResourcesSpec pointer.
// Handles the pointer input. Returns an empty struct if pointer is nil.
func GetResourcesSpecOrDefault(resources *common.ResourcesSpec) corev1.ResourceRequirements {
	if resources == nil {
		return corevval.ResourceRequirements{} // Return empty K8s struct if config pointer is nil
	}
	// Return copy of ResourceList maps (they implement DeepCopy())
	return corev1.ResourceRequirements{
		Limits:   resources.Limits.DeepCopy(),
		Requests: resources.Requests.DeepCopy(),
	}
}

// GetBoolPtrValueOrDefault returns the value of a bool pointer or a default value.
func GetBoolPtrValueOrDefault(ptr *bool, defaultValue bool) bool {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// GetStringValueOrDefault returns the value of a string pointer or a default value.
func GetStringValueOrDefault(ptr *string, defaultValue string) string {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// GetInt32ValueOrDefault returns the value of an int32 pointer or a default value.
func GetInt32ValueOrDefault(ptr *int32, defaultValue int32) int32 {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// GetServiceAccountName derives a Service Account name based on component instance name and optional common config.
// Returns the resolved SA name string (lowercase DNS-safe).
func DeriveServiceAccountName(instanceName string, config *common.ServiceAccountSpec) string {
	// If user provides a name in config, use it. Otherwise, use a default convention.
	if config != nil && config.Name != "" {
		return strings.ToLower(config.Name) // Use user-provided name if exists
	}
	// Default convention: component instance name + "-sa" (lowercase)
	return strings.ToLower(instanceName) + "-sa"
}

// GetServiceAccountAnnotations gets SA annotations from common config if provided.
func GetServiceAccountAnnotations(config *common.ServiceAccountSpec) map[string]string {
	if config != nil {
		return config.Annotations
	} // Return annotations map directly
	return nil // Return nil map if no config or no annotations
}

// BuildServicePort constructs a K8s corev1.ServicePort from common.PortSpec.
// Maps common.PortSpec fields to K8s corev1.ServicePort fields.
// Called by BuildServicePorts.
func BuildServicePort(portSpec common.PortSpec) corev1.ServicePort { // Accepts value type
	sp := corev1.ServicePort{
		Name:       portSpec.Name,                            // Use name from common.PortSpec (might be empty)
		Port:       portSpec.ContainerPort,                   // Often ServicePort is same as ContainerPort by default
		TargetPort: intstr.FromInt32(portSpec.ContainerPort), // Default target by number by default
		Protocol:   corev1.ProtocolTCP,                       // Default TCP
	}
	// Override default protocol if specified
	if portSpec.Protocol != "" {
		sp.Protocol = portSpec.Protocol
	}

	// Override TargetPort if explicitly specified in common.PortSpec
	// If TargetPort in common.PortSpec is nil, default to targeting ContainerPort by number (already done).
	// If TargetPort is set as string or IntOrString, use it.
	if portSpec.TargetPort != nil {
		sp.TargetPort = *portSpec.TargetPort
	}

	// K8s requires unique Names for service ports if more than one.
	// If common.PortSpec Name is empty, assign a default based on port number.
	// This helps K8s validation and allows targeting by generated name if no name provided.
	if sp.Name == "" {
		sp.Name = fmt.Sprintf("port-%d-%s", sp.Port, strings.ToLower(string(sp.Protocol))) // Convention: port-NNNN-protocol
		// Add more robustness if name is not DNS_LABEL safe after this format.
	}

	return sp
}

// BuildServicePorts builds corev1.ServicePort slices from common.PortSpec slices.
// Maps common.PortSpec fields to K8s corev1.ServicePort fields.
func BuildServicePorts(portSpecs []common.PortSpec) []corev1.ServicePort { // Accepts slice of value types
	if portSpecs == nil || len(portSpecs) == 0 {
		return []corev1.ServicePort{}
	}

	k8sPorts := make([]corev1.ServicePort, 0, len(portSpecs))
	for _, ps := range portSpecs { // Iterate through value copy
		k8sPorts = append(k8sPorts, BuildServicePort(ps)) // Call BuildServicePort helper
	}
	return k8sPorts
}

// BuildContainerPort constructs a K8s corev1.ContainerPort from common.PortSpec.
// Called by BuildContainerPorts.
func BuildContainerPort(portSpec common.PortSpec) corev1.ContainerPort { // Accepts value type
	return corev1.ContainerPort{
		Name:          portSpec.Name,
		ContainerPort: portSpec.ContainerPort,
		Protocol:      portSpec.Protocol, // Uses K8s built-in type default if empty ("TCP")
		// Other optional fields like HostIP, HostPort
	}
}

// BuildContainerPorts builds corev1.ContainerPort slices from common.PortSpec slices.
// Used for main container spec.
func BuildContainerPorts(portSpecs []common.PortSpec) []corev1.ContainerPort { // Accepts slice of value types
	if portSpecs == nil || len(portSpecs) == 0 {
		return []corev1.ContainerPort{}
	}

	k8sPorts := make([]corev1.ContainerPort, 0, len(portSpecs))
	for _, ps := range portSpecs { // Iterate through value copy
		k8sPorts = append(k8sPorts, BuildContainerPort(ps)) // Call BuildContainerPort helper
	}
	return k8sPorts
}

// BuildProbe constructs a K8s corev1.Probe from corev1.Probe struct pointer.
// This is trivial if common.ProbesConfig uses *corev1.Probe directly.
func BuildProbe(probeSpec *corev1.Probe) *corev1.Probe { // Accepts pointer
	if probeSpec == nil {
		return nil
	}
	// If custom defaulting on *fields* inside probeSpec is needed, do it here.
	// e.g., if probeSpec.HTTPGet != nil { if probeSpec.HTTPGet.Scheme == "" { probeSpec.HTTPGet.Scheme = corev1.URISchemeHTTP }}
	// But simpler is often to just return the pointer if it's valid K8s spec.
	return probeSpec // Return the pointer received
}

// BuildVolumesFromConfigMaps builds corev1.Volume slices for ConfigMaps from common.ConfigMountSpec slices.
// Used for Pod Spec Volumes list.
func BuildVolumesFromConfigMaps(configMounts []common.ConfigMountSpec) []corev1.Volume { // Accepts slice of value types
	if configMounts == nil || len(configMounts) == 0 {
		return []corev1.Volume{}
	}

	volumes := make([]corev1.Volume, 0, len(configMounts))
	for _, m := range configMounts { // Iterate through value copy of struct
		volumeName := m.VolumeName // Use provided volume name
		if volumeName == "" {      // Default if not provided (e.g. CM name)
			volumeName = m.Name // Default Volume Name to ConfigMap name
			// Need to ensure name is unique across ALL volumes for the Pod.
			// Add logic here or rely on higher-level volume aggregator to resolve uniqueness/collisions.
		}

		volumes = append(volumes, corev1.Volume{
			Name: volumeName, // Volume name must match the mount name
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: m.Name}, // ConfigMap resource name
					Items:                m.Items,                                   // Optional list of key-to-path, can be nil or empty slice.
					// DefaultMode: // DefaultMode for file permissions (need config for this)
				},
			},
		})
	}
	return volumes
}

// BuildVolumeMountsFromConfigMaps builds corev1.VolumeMount slices for containers from common.ConfigMountSpec slices.
// These volume mounts reference volumes created based on common.ConfigMountSpec.
func BuildVolumeMountsFromConfigMaps(configMounts []common.ConfigMountSpec) []corev1.VolumeMount { // Accepts slice of value types
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
			// Path in VolumeMount is SubPath *relative to* the volume's root path
			ReadOnly: readOnly,
		})
	}
	return volumeMounts
}

// BuildVolumesFromSecrets builds corev1.Volume slices for Secrets from common.SecretMountSpec slices.
// Used for Pod Spec Volumes list.
func BuildVolumesFromSecrets(secretMounts []common.SecretMountSpec) []corev1.Volume { // Accepts slice of value types
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

// BuildVolumeMountsFromSecrets builds corev1.VolumeMount slices for containers from common.SecretMountSpec slices.
// Used for main container spec VolumeMounts list.
func BuildVolumeMountsFromSecrets(secretMounts []common.SecretMountSpec) []corev1.VolumeMount { // Accepts slice of value types
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
			ReadOnly:  readOnly,
			// SubPath if needed via Items logic (handled in Volume definition)
		})
	}
	return volumeMounts
}

// BuildVolumesFromAdditionalVolumes builds corev1.Volume slices from common.AdditionalVolumes slices.
// These volumes need to be added to the Pod Spec Volumes list.
// Assumes AdditionalVolumes field in common.types is defined as []corev1.Volume.
func BuildVolumesFromAdditionalVolumes(additionalVolumes []corev1.Volume) []corev1.Volume { // Accepts slice of value types
	if additionalVolumes == nil || len(additionalVolumes) == 0 {
		return []corev1.Volume{}
	}

	// Return a DeepCopy of the provided K8s volume specs.
	builtVolumes := make([]corev1.Volume, 0, len(additionalVolumes))
	for i := range additionalVolumes { // Iterate through value copy of structs
		builtVolumes = append(builtVolumes, *additionalVolumes[i].DeepCopy()) // Use K8s DeepCopy()
	}
	return builtVolumes
}

// BuildVolumeMountsFromAdditionalVolumes builds corev1.VolumeMount slices for main containers from
// a slice of raw corev1.VolumeMount spec provided in configuration.
// This is for user provided explicit volume mounts, referencing volumes built by other builders.
func BuildVolumeMountsFromAdditionalVolumes(additionalVolumeMounts []corev1.VolumeMount) []corev1.VolumeMount { // Accepts slice of value types
	if additionalVolumeMounts == nil || len(additionalVolumeMounts) == 0 {
		return []corev1.VolumeMount{}
	}

	// Return a DeepCopy of the provided K8s volume mount specs.
	builtMounts := make([]corev1.VolumeMount, 0, len(additionalVolumeMounts))
	for i := range additionalVolumeMounts { // Iterate through value copy of structs
		builtMounts = append(builtMounts, *additionalVolumeMounts[i].DeepCopy()) // Use K8s DeepCopy()
	}
	return builtMounts
}

// BuildVolumeClaimTemplates builds PersistentVolumeClaim templates for StatefulSet.
// Returns a slice of K8s PVC templates. Requires StorageSpec list if multiple template definitions are possible.
// For simplicity, assume one StorageSpec = one VCT definition here.
func BuildVolumeClaimTemplates(storageSpec *common.StorageSpec, commonLabels map[string]string) ([]corev1.PersistentVolumeClaim, error) {
	// Ensure StorageSpec pointer is non-nil and storage is enabled.
	if storageSpec == nil || !storageSpec.Enabled {
		return []corev1.PersistentVolumeClaim{}, nil // Nothing to build
	}

	// Ensure essential fields are present if enabled (Size, VolumeClaimTemplateName, MountPath).
	// Size validation *must* be handled as it's required for K8s PVC requests.
	if storageSpec.Size == nil {
		// Error as Size is required when enabled
		return nil, fmt.Errorf("storage is enabled but required field 'size' is missing")
	}
	if storageSpec.VolumeClaimTemplateName == "" {
		// Default name if empty. Rely on a helper for deriving unique VCT names if needed.
		// Using a default convention if name is empty.
		storageSpec.VolumeClaimTemplateName = "data" // Common K8s convention if name is empty. Or derive from instance name.
	}
	if storageSpec.MountPath == "" {
		// Mount path is also required if enabled.
		return nil, fmt.Errorf("storage is enabled but required field 'mountPath' is missing")
	}

	// Build the PersistentVolumeClaim template spec.
	pvcTemplateSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: storageSpec.AccessModes, // Value slice
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: *storageSpec.Size, // Use quantity value
			},
		},
		StorageClassName: storageSpec.StorageClassName, // Pointer
		VolumeMode:       nil,                          // Default Block or Filesystem
	}
	// Apply default access modes if not provided (common.types default helps here)
	if len(pvcTemplateSpec.AccessModes) == 0 {
		pvcTemplateSpec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}

	// Build the PersistentVolumeClaim object that acts as the template.
	pvcTemplate := corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "PersistentVolumeClaim"},
		ObjectMeta: metav1.ObjectMeta{
			Name:   storageSpec.VolumeClaimTemplateName, // VCT Name (used as Volume name in Pod Spec)
			Labels: commonLabels,                        // Apply common labels
		},
		Spec: pvcTemplateSpec, // Set the spec
	}

	// Return a slice containing the single VCT definition.
	return []corev1.PersistentVolumeClaim{pvcTemplate}, nil
}

// BuildSharedPVCPVC builds a single corev1.PersistentVolumeClaim object for a shared PVC (Deployment).
// Needs PersistenceSpec and context for name, namespace, labels.
// This creates the *resource*, not the template.
func BuildSharedPVCPVC(persistenceConfig *common.PersistenceSpec, instanceName string, namespace string, commonLabels map[string]string) (*corev1.PersistentVolumeClaim, error) {
	if persistenceConfig == nil || !persistenceConfig.Enabled {
		return nil, nil
	} // Nothing to build

	// Ensure essential fields are present if enabled (Size, VolumeName, MountPath).
	// VolumeName is often the name in PodSpec, derived PVC name might be different or derived from this.
	// Let's ensure Size and MountPath are present and Size is non-nil.
	if persistenceConfig.Size == nil {
		return nil, fmt.Errorf("persistence is enabled but required field 'size' is missing")
	}
	// MountPath and VolumeName validation: Add if needed, or assume presence due to config structure design or validation elsewhere.

	// Determine the name for the PVC *resource* in Kubernetes API.
	// Use the resource name derivation convention + a suffix like "-pvc".
	pvcResourceName := DeriveResourceName(instanceName) + "-pvc" // Convention: instanceName-pvc
	// Or derive based on persistenceConfig.VolumeName if needed.

	// Build the PersistentVolumeClaim Spec.
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: persistenceConfig.AccessModes, // Value slice
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: *persistenceConfig.Size, // Use quantity value
			},
		},
		StorageClassName: persistenceConfig.StorageClassName, // Pointer
		VolumeMode:       nil,                                // Default Block or Filesystem
	}
	// Apply default access modes if not provided (common.types default helps here)
	if len(pvcSpec.AccessModes) == 0 {
		pvcSpec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}

	// Build the PersistentVolumeClaim object.
	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{ // Start of TypeMeta composite literal
			APIVersion: corev1.SchemeGroupVersion.Version, // Correct field name is APIVersion, not A<ctrl62>
			Kind:       "PersistentVolumeClaim",           // Correct field name is Kind
		}, // End of TypeMeta composite literal. Needs comma IF more fields follow.
		ObjectMeta: metav1.ObjectMeta{ // Start of ObjectMeta composite literal
			Name:      pvcResourceName,
			Namespace: namespace,
			Labels:    commonLabels,
			// OwnerReference is set later
		}, // End of ObjectMeta composite literal. Needs comma IF more fields follow.
		Spec: pvcSpec, // Spec field is assigned a corev1.PersistentVolumeClaimSpec value
		// Status field is omitted when building new objects.
	}

	return pvc, nil // Return built PVC object pointer
}

// BuildObjectMeta builds standard Kubernetes ObjectMeta.
// It requires name, namespace, and label/annotation maps.
// Returns a metav1.ObjectMeta struct.
func BuildObjectMeta(name string, namespace string, labels map[string]string, annotations map[string]string) metav1.ObjectMeta {
	// ownerReference is set AFTER building by the caller (e.g. ApplyTask).
	return metav1.ObjectMeta{
		Name:        name,        // Provided name (derived using DeriveResourceName)
		Namespace:   namespace,   // Provided namespace (from AppDef)
		Labels:      labels,      // Use provided labels
		Annotations: annotations, // Use provided annotations (if any)
	}
}
