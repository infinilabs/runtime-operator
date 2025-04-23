// pkg/builders/k8s/pdb.go
package k8s

import (
	policyv1 "k8s.io/api/policy/v1" // Requires this specific K8s API package for PDB
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Scheme
	// App types
	"github.com/infinilabs/operator/pkg/apis/common" // Common types (PdbConfig)
	// common_util "github.com/infinilabs/operator/pkg/apis/common/util" // Utils if needed
)

// BuildPodDisruptionBudget builds a policyv1.PodDisruptionBudget resource.
// Needs PDB configuration from common.types and context for metadata and selector.
func BuildPodDisruptionBudget(pdbConfig *common.PodDisruptionBudgetSpecPart, resourceName string, namespace string, labels map[string]string, selectorLabels map[string]string) *policyv1.PodDisruptionBudget {
	// Ensure PDB config is non-nil. Caller should handle this.
	if pdbConfig == nil {
		return nil
	} // Should not be called with nil config

	pdb := &policyv1.PodDisruptionBudget{
		TypeMeta:   metav1.TypeMeta{APIVersion: policyv1.SchemeGroupVersion.Version, Kind: "PodDisruptionBudget", Group: policyv1.SchemeGroupVersion.Group},
		ObjectMeta: BuildObjectMeta(resourceName, namespace, labels, nil), // Use generic metadata builder, no annotations in PdbConfig
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector:       &metav1.LabelSelector{MatchLabels: selectorLabels}, // Selector must match the Pods it applies to
			MinAvailable:   pdbConfig.MinAvailable,                             // Pointer or Value depending on PdbConfig def
			MaxUnavailable: pdbConfig.MaxUnavailable,                           // Pointer or Value
			// Add other PDB spec fields from PdbConfig if needed (e.g., ExpectedPodHbms, HealthyPodEvictionPolicy)
		},
	}

	return pdb // Return built PDB object pointer
}
