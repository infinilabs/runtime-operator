// pkg/builders/k8s/pdb.go
package k8s

import (
	policyv1 "k8s.io/api/policy/v1" // Requires this specific K8s API package
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// Needed for IntOrString in PDB spec
	// common "github.com/infinilabs/operator/pkg/apis/common" // Only if referencing PdbConfig helper struct
)

// BuildPodDisruptionBudget builds a policyv1.PodDisruptionBudget resource.
// Receives the K8s PDB Spec struct directly (built by the caller/app-specific builder)
// along with standard metadata parts.
func BuildPodDisruptionBudget(
	pdbMeta metav1.ObjectMeta, // ObjectMeta for the PDB resource
	pdbSpec policyv1.PodDisruptionBudgetSpec, // The fully constructed PDB Spec
	selectorLabels map[string]string, // Selector labels MUST match the Pods it applies to
) *policyv1.PodDisruptionBudget {

	// Set the selector in the spec (redundant if already set by caller, but safe to ensure)
	if pdbSpec.Selector == nil {
		pdbSpec.Selector = &metav1.LabelSelector{}
	}
	pdbSpec.Selector.MatchLabels = selectorLabels // Ensure selector matches

	pdb := &policyv1.PodDisruptionBudget{
		TypeMeta:   metav1.TypeMeta{APIVersion: policyv1.SchemeGroupVersion.String(), Kind: "PodDisruptionBudget"},
		ObjectMeta: pdbMeta, // Use pre-built metadata
		Spec:       pdbSpec, // Use pre-built spec
	}
	return pdb // Return built PDB object pointer
}
