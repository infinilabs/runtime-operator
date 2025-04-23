// pkg/builders/k8s/statefulset.go
package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// ApplicationDefinition, Component types
	// Common config types
)

// BuildStatefulSet builds an appsv1.StatefulSet resource.
// Called by App-Specific Builder's BuildObjects method.
// Takes inputs that map directly to K8s StatefulSet and Pod Template specs.
func BuildStatefulSet(
	// Required inputs for StatefulSet metadata and spec
	stsMeta metav1.ObjectMeta, // ObjectMeta for the StatefulSet resource
	selectorLabels map[string]string, // Labels used for the StatefulSet selector AND Pod metadata selector part

	// Required inputs for StatefulSet Spec
	replicas *int32, // Resolved replicas count (pointer)
	podTemplateSpec corev1.PodTemplateSpec, // Fully built PodTemplateSpec
	serviceName string, // Name of the Headless Service (StatefulSet requirement)
	volumeClaimTemplates []corev1.PersistentVolumeClaim, // List of built Volume Claim Templates
	updateStrategy appsv1.StatefulSetUpdateStrategy, // Determined update strategy
	podManagementPolicy appsv1.PodManagementPolicyType, // Determined pod management policy

	// Pass component/app context if needed internally (e.g. for builder helper calls)
	// appDef *appv1.ApplicationDefinition,
	// appComp *appv1.ApplicationComponent,

) *appsv1.StatefulSet {

	// Validation (should mostly be done by caller)
	// Ensure selector labels not empty, Pod template valid, Headless Service name set.
	// Ensure VolumeClaimTemplates list is valid if StatefulSet is created.

	statefulSet := &appsv1.StatefulSet{
		TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.Version, Kind: "StatefulSet"},
		ObjectMeta: stsMeta, // Use the pre-built metadata
		Spec: appsv1.StatefulSetSpec{
			Replicas:             replicas,                                           // Set replicas (pointer)
			Selector:             &metav1.LabelSelector{MatchLabels: selectorLabels}, // Set selector
			ServiceName:          serviceName,                                        // Link to Headless Service name
			Template:             podTemplateSpec,                                    // Assign pre-built Pod template (value type)
			VolumeClaimTemplates: volumeClaimTemplates,                               // Assign list of VCTs
			UpdateStrategy:       updateStrategy,                                     // Assign update strategy (value type)
			PodManagementPolicy:  podManagementPolicy,                                // Assign pod management policy (value type)

			// Add other direct StatefulSet spec fields if supported (e.g. RevisionHistoryLimit)
		},
	}

	return statefulSet // Return the built typed object pointer
}

// Example of calling this from an application-specific builder (like pkg/builders/gateway/builders.go):
/*
func (b *GatewayBuilderStrategy) BuildObjects(...) ([]client.Object, error) {
     // ... (Unmarshal config, derive values like replicas, build labels, build pod template) ...

     // Resolve StatefulSet specific spec fields from config or defaults
     headlessSvcName := builders.DeriveResourceName(instanceName) + "-headless" // Convention
     // Optionally override from GatewayConfig if it has StatefulSetOverrides
     // if gatewayConfig.StatefulSetOverrides != nil && gatewayConfig.StatefulSetOverrides.ServiceName != nil {
     //      headlessSvcName = *gatewayConfig.StatefulSetOverrides.ServiceName
     // }

     updateStrategy := builders.GetStatefulSetUpdateStrategyOrDefault(nil) // Requires overrides pointer input
     podManagementPolicy := builders.GetStatefulSetPodManagementPolicyOrDefault(nil) // Requires overrides pointer input

     // Build Volume Claim Templates based on config.Storage
     vctList, err := builders.BuildVolumeClaimTemplates(gatewayConfig.Storage, commonLabels) // Needs BuildVolumeClaimTemplates helper

     // Build StatefulSet ObjectMeta
      stsMeta := builders.BuildObjectMeta(resourceName, namespace, commonLabels, nil) // Annotations?

     // Call BuildStatefulSet
     statefulSet := builders.BuildStatefulSet(
         appDef, // Owner (optional argument if set by caller)
         appComp, // Context (optional argument)
         stsMeta, // Metadata
         selectorLabels, // Selector
         &replicas, // Resolved replicas (pointer)
         *builtPodTemplateSpec, // Dereferenced pod template
         headlessSvcName, // Headless service name string
         vctList, // Pre-built VCTs
         updateStrategy, // Resolved strategy
         podManagementPolicy, // Resolved policy
     )

     objects := []client.Object{statefulSet} // Start list

     // ... Build Headless Service, Client Service, CMs, Secrets, PDB etc. ...

     return objects, nil

}
*/
