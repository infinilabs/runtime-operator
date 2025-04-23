// pkg/builders/k8s/secret.go
package k8s

import (
	"github.com/infinilabs/operator/pkg/apis/common"
	commonutil "github.com/infinilabs/operator/pkg/apis/common/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// Needed for Scheme
	// Use base64 for encoding binary data if needed.
	// No appv1 or common dependency directly here
)

// BuildSecretsFromAppData builds a list of corev1.Secret objects from AppConfigData if values represent secrets.
// This might require a more complex rule than map[string]string. Perhaps map[string]common.SecretFile or map[string]common.SecretValue?
// For simplicity, assume AppConfigData map[string]string can contain keys indicating sensitive data (e.g., "*.key", "*password").
// Or assume *another* map in specific config holds files that MUST be secrets.
// Let's provide a basic builder that assumes you give it the *data* and the desired *secret name*.
// The decision on WHICH data goes into secrets is application-specific and should be done in app builders.

// BuildSecret builds a standard corev1.Secret resource.
// data: map[string]string where keys are filenames and values are base64-encoded strings (BinaryData) or string data (Data).
// Assumes you want to build ONE Secret from the provided data.
func BuildSecret(
	secretName string, // Name of the secret resource
	namespace string, // Namespace
	labels map[string]string, // Labels
	annotations map[string]string, // Annotations (Optional)
	stringData map[string]string, // Data as strings (K8s converts to base64)
	binaryData map[string][]byte, // Binary data (base64 encoded or raw bytes depending on K8s version/conversion)
	secretType corev1.SecretType, // Opaque, kubernetes.io/tls, etc.
) *corev1.Secret {
	// Ensure required fields are present (name, namespace)

	secret := &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "Secret"},
		ObjectMeta: BuildObjectMeta(secretName, namespace, labels, annotations),
		Data:       stringData, // Use stringData field
		BinaryData: binaryData, // Use binaryData field (exclusive with Data based on K8s validation logic, depends on type)
		Type:       secretType, // Specify Secret Type
	}
	// Note: For common use cases, you might use `Data` field with string values, and K8s handles base64.

	// Ensure Data and BinaryData are mutually exclusive depending on Secret Type validation rules.
	// K8s usually validates this on the API server. Builders create valid structures.
	if secretType == corev1.SecretTypeTLS {
		// Validation for TLS Secret: Requires "tls.crt", "tls.key" keys in Data/BinaryData
	}

	// A more common use case: Builder receiving a map of filename->string, decides if CM or Secret based on convention/flags.
	// Or, receiving map[string]string specifically for SECRET data.
	// Let's use a builder specific for Secret data map:

	return secret
}

// BuildSecretsFromDataMap builds corev1.Secret objects from a map[string]string specific for secrets.
// Assumes you want to build ONE Secret resource named 'resourceName' with the given data map.
func BuildSecretsFromDataMap(
	secretName string,
	namespace string,
	labels map[string]string,
	annotations map[string]string, // Optional
	data map[string]string, // Data content for the secret
	secretType corev1.SecretType, // Specify type
) []client.Object { // Return slice of client.Object

	if data == nil || len(data) == 0 {
		return []client.Object{}
	}

	secret := BuildSecret(secretName, namespace, labels, annotations, data, nil, secretType) // Use StringData field

	return []client.Object{secret}
}

// BuildSecretRefVolume builds a VolumeMount referencing a Secret resource.
// This helper is used by the Pod builder.
func BuildSecretRefVolumeMount(mount common.SecretMountSpec) corev1.VolumeMount {
	// This is similar to BuildVolumeMountsFromSecrets but for a single spec.
	readOnly := commonutil.GetBoolPtrValueOrDefault(mount.ReadOnly, true)
	volumeName := mount.VolumeName
	if volumeName == "" {
		volumeName = mount.SecretName
	} // Default if not provided

	volumeMount := corev1.VolumeMount{
		Name:      volumeName,
		MountPath: mount.MountPath,
		ReadOnly:  readOnly,
		// SubPath if needed via Items logic
	}
	return volumeMount
}

// BuildSecretRefVolume builds a corev1.Volume referencing a Secret resource.
// This is used by the Pod builder's Volumes list.
func BuildSecretRefVolume(mount common.SecretMountSpec) corev1.Volume {
	// This is similar to BuildVolumesFromSecrets but for a single spec.
	volumeName := mount.VolumeName
	if volumeName == "" {
		volumeName = mount.SecretName
	} // Default if not provided

	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: mount.SecretName,
				Items:      m.Items,
				// DefaultMode if needed
			},
		},
	}
	return volume
}

// Note: Add BuildServiceAccount builder framework here if needed (moved from helper.go idea).
