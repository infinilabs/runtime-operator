// pkg/builders/k8s/secret.go
package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildSecret builds a standard corev1.Secret resource.
// Accepts stringData (K8s will base64 encode) and binaryData (already base64 encoded or raw bytes).
func BuildSecret(
	secretMeta metav1.ObjectMeta, // Pre-built metadata
	stringData map[string]string, // Data as strings
	binaryData map[string][]byte, // Binary data
	secretType corev1.SecretType, // Opaque, kubernetes.io/tls, etc. Defaults to Opaque if empty.
) *corev1.Secret {

	// Use Opaque as default type if none provided
	sType := secretType
	if sType == "" {
		sType = corev1.SecretTypeOpaque
	}

	secret := &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "Secret"},
		ObjectMeta: secretMeta,
		StringData: stringData, // Use StringData for easier definition (K8s handles encoding)
		Data:       binaryData, // Use Data for binary content (caller must handle encoding if needed)
		Type:       sType,      // Set Secret Type
	}

	// K8s API server handles validation for Data/StringData mutual exclusivity based on Type.

	return secret
}

// BuildSecretsFromDataMap builds corev1.Secret objects from a map[string]string specific for secrets.
// Assumes you want to build ONE Secret resource named 'secretName'.
// Converts string values to bytes for the Secret.Data field.
// For StringData use case, modify this function or use BuildSecret directly.
func BuildSecretsFromDataMap(
	secretName string, // Derived name for the secret resource
	namespace string,
	labels map[string]string,
	annotations map[string]string, // Optional
	data map[string]string, // String data content for the secret
	secretType corev1.SecretType, // Specify type (e.g., Opaque)
) ([]client.Object, error) { // Return client.Object slice

	if data == nil || len(data) == 0 {
		return []client.Object{}, nil
	}

	// Build metadata
	secretMeta := BuildObjectMeta(secretName, namespace, labels, annotations)

	// Build the Secret object using StringData field for simplicity
	secret := BuildSecret(secretMeta, data, nil, secretType) // Pass nil for binaryData

	return []client.Object{secret}, nil
}
