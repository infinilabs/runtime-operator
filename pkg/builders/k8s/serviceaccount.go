// pkg/builders/k8s/serviceaccount.go
package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Need if ServiceAccount config contains app context
	"github.com/infinilabs/operator/pkg/apis/common" // Common types (ServiceAccountSpec)
	// Common utils
)

// BuildServiceAccount builds a corev1.ServiceAccount resource.
// createFlag: determines if the SA should be built at all (config.ServiceAccount.Create value)
// saConfig: common.ServiceAccountSpec configuration (pointer)
// derivedName: the resolved name for the ServiceAccount (e.g., derived from component instance)
// namespace: the namespace for the SA
// commonLabels: labels to apply
func BuildServiceAccount(saConfig *common.ServiceAccountSpec, derivedName string, namespace string, commonLabels map[string]string) *corev1.ServiceAccount {
	// Ensure the ServiceAccount should be created (Check the 'create' flag from config)
	// This should probably be handled by the caller (app-specific builder) BEFORE calling this,
	// based on common.ServiceAccountSpec.Create.

	// Let's assume caller ensures this is called ONLY when creation is enabled.
	// This builder just *builds* the SA object.

	// Use default name if config name is empty
	saName := derivedName // Assumes derivedName already includes handling for config.Name

	sa := &corev1.ServiceAccount{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "ServiceAccount"},
		ObjectMeta: BuildObjectMeta(saName, namespace, commonLabels, GetServiceAccountAnnotations(saConfig)), // Use generic metadata builder and SA annotation helper
		// ImagePullSecrets and Secrets lists are optional, usually not set here unless standard pattern
	}

	return sa // Return the built Service Account object pointer
}

// GetServiceAccountName derives a Service Account name based on component instance name and optional config.
// Used by builders. Needs Component Instance name as input.
/* Moved from helpers.go or stay there.
func DeriveServiceAccountName(instanceName string, config *common.ServiceAccountSpec) string { ... } // Implementation already in helpers.go
func GetServiceAccountAnnotations(config *common.ServiceAccountSpec) map[string]string { ... } // Implementation already in helpers.go
*/
