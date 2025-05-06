// pkg/builders/k8s/statefulset.go
package k8s

import (
	appsv1 "k8s.io/api/apps/v1" // Needed for VolumeClaimTemplate type
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildStatefulSet builds an appsv1.StatefulSet resource.
// It takes fully assembled ObjectMeta and StatefulSetSpec as input.
// The caller (e.g., application-specific builder) is responsible for constructing these specs correctly,
// including PodTemplateSpec and VolumeClaimTemplates.
func BuildStatefulSet(
	stsMeta metav1.ObjectMeta, // ObjectMeta for the StatefulSet resource
	stsSpec appsv1.StatefulSetSpec, // The complete StatefulSet Spec
) *appsv1.StatefulSet {

	// Basic validation: Ensure ServiceName is set if replicas > 0, check selector matches template labels.
	// Caller should ensure these consistencies.

	statefulSet := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "StatefulSet",
		},
		ObjectMeta: stsMeta, // Use the pre-built metadata
		Spec:       stsSpec, // Use the pre-built spec
	}

	return statefulSet // Return the built typed object pointer
}
