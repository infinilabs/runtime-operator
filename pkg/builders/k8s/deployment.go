// pkg/builders/k8s/deployment.go
package k8s

import (
	appsv1 "k8s.io/api/apps/v1" // Needed for PodTemplateSpec type
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildDeployment builds an appsv1.Deployment object.
// It takes fully assembled ObjectMeta and DeploymentSpec as input.
// The caller (e.g., application-specific builder) is responsible for constructing these specs correctly.
func BuildDeployment(
	deployMeta metav1.ObjectMeta, // ObjectMeta for the Deployment resource
	deploySpec appsv1.DeploymentSpec, // The complete Deployment Spec
) *appsv1.Deployment {

	// Basic validation can be added here if needed, e.g., check selector and template labels match.
	// However, the caller (App-specific builder) should ensure consistency.

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(), // Use correct GVK info
			Kind:       "Deployment",
		},
		ObjectMeta: deployMeta, // Use the pre-built metadata
		Spec:       deploySpec, // Use the pre-built spec
	}

	// Return the assembled typed object pointer
	return deployment
}
