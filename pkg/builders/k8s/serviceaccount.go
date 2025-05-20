// pkg/builders/k8s/serviceaccount.go
package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// Need access to helpers if used
	// "github.com/infinilabs/runtime-operator/pkg/apis/common" // Not directly needed here
	// commonutil "github.com/infinilabs/runtime-operator/pkg/apis/common/util"
)

// BuildServiceAccount builds a corev1.ServiceAccount resource.
// Expects pre-derived name, namespace, labels, and annotations.
func BuildServiceAccount(
	saMeta metav1.ObjectMeta, // Pre-built metadata (includes derived name, namespace, labels, annotations)
	// Optional: ImagePullSecrets, Secrets if ServiceAccount config structure included them
	// imagePullSecrets []corev1.LocalObjectReference,
	// secrets []corev1.ObjectReference,
) *corev1.ServiceAccount {

	sa := &corev1.ServiceAccount{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "ServiceAccount"},
		ObjectMeta: saMeta, // Use pre-built metadata

		// Assign optional fields if provided
		// ImagePullSecrets: imagePullSecrets,
		// Secrets: secrets,
		// AutomountServiceAccountToken: // Add if configurable
	}

	return sa // Return the built Service Account object pointer
}
