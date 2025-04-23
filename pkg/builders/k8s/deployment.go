// pkg/builders/k8s/deployment.go
package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// Needed for ApplicationDefinition, Component context
	// common_util "github.com/infinilabs/operator/pkg/apis/common/util" // Needed if calling common utils
	// Needed if builders set OwnerReference (recommended caller sets OwnerRef)
)

// BuildDeployment builds an appsv1.Deployment resource.
// This builder is called by App-Specific Builder's BuildObjects method.
// It takes inputs that map directly to K8s Deployment and Pod Template specs, derived by the caller.
func BuildDeployment(
	// Required inputs for Deployment metadata and spec
	deployMeta metav1.ObjectMeta, // ObjectMeta for the Deployment resource
	selectorLabels map[string]string, // Labels used for the Deployment selector AND Pod metadata selector part

	// Required inputs for Deployment Spec
	replicas *int32, // Resolved replicas count (pointer for optionality/defaulting by caller)
	podTemplateSpec corev1.PodTemplateSpec, // Fully built PodTemplateSpec (contains PodSpec and Pod metadata template)
	strategy appsv1.DeploymentStrategy, // Determined Deployment Strategy (value type)

	// Pass component/app context if needed internally by common builders (e.g. DeriveResourceName calls etc)
	// appDef *appv1.ApplicationDefinition,
	// appComp *appv1.ApplicationComponent,

) *appsv1.Deployment {
	// Validation (should mostly be done by caller after resolving inputs from config)
	// Ensure selector labels are not empty
	// Ensure Pod template spec is valid

	deployment := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.Version, Kind: "Deployment"}, // Explicitly set TypeMeta
		ObjectMeta: deployMeta,                                                                         // Use the pre-built metadata (includes Name, Namespace, Labels)
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,                                           // Set replicas count (pointer)
			Selector: &metav1.LabelSelector{MatchLabels: selectorLabels}, // Set selector using pointer
			Template: podTemplateSpec,                                    // Assign the pre-built Pod template (value type)
			Strategy: strategy,                                           // Assign the determined strategy (value type)
		},
	}

	return deployment // Return the built typed object pointer
}

// Example of calling this from an application-specific builder (like pkg/builders/gateway/builders.go):
/*
func (b *GatewayBuilderStrategy) BuildObjects(...) ([]client.Object, error) {
    // ... (Unmarshal config, derive values like replicas, build common labels) ...

    // Build the PodTemplateSpec first
    // Need to pass inputs like image, resources, ports, env, etc. to the pod builder helpers.
    // These come from the unmarshalled *common.GatewayConfig.
    mainContainerSpec, err := builders.BuildMainContainerSpec(...) // pass gatewayConfig.Image etc.
    initContainersList := buildGatewayInitContainers(gatewayConfig) // App specific init builder helper
    allVolumesList := buildGatewayVolumes(gatewayConfig) // App specific volume builder helper
    mainContainerMountsList := buildGatewayVolumeMounts(gatewayConfig) // App specific mount builder helper

    // Pass necessary fields for PodTemplateSpec builders
    podLabels := builders.BuildCommonLabels(appName, appComp.Type, instanceName)
    podSelectorLabels := builders.BuildSelectorLabels(instanceName)

    // Add Mounts to the main container built spec! THIS IS CRUCIAL AND NEEDS TO BE DONE BEFORE BUILDING POD TEMPLATE
    mainContainerSpec.VolumeMounts = mainContainerMountsList


    podTemplate, err := builders.BuildPodTemplateSpec(
        mainContainerSpec,    // The core container built
        initContainersList, // List of init containers
        allVolumesList,     // List of all volumes needed for pod
        // mainContainerMountsList is now inside mainContainer
        podSelectorLabels,  // Pod labels (contains selector)
        nil, // No pod annotations from gatewayConfig example currently

        // Pass Pod Spec common fields from config here if needed by PodTemplateSpec builder
        // podSecurityConfig: gatewayConfig.PodSecurityContext, // If this is handled by PodTemplate builder
        // serviceAccountName: builders.DeriveServiceAccountName(instanceName, gatewayConfig.ServiceAccount), // If this is handled by PodTemplate builder
        // nodeSelector: gatewayConfig.NodeSelector, // If handled by PodTemplate builder
        // ... other fields ...

    )


    // Derive values for Deployment
    resourceName := builders.DeriveResourceName(instanceName)
    deployMeta := builders.BuildObjectMeta(resourceName, namespace, commonLabels, nil) // Needs annotation if any

    // Get Deployment Strategy
    // Strategy definition in common.types might need to map specific config to standard K8s strategy
    strategy := builders.GetDeploymentStrategyOrDefault(nil) // Needs common DeploymentOverrides spec

    // Call BuildDeployment
    deployment := builders.BuildDeployment(
        appDef, // Owner (optional argument in builder func if SetOwnerRef done later)
        appComp, // Component context (optional)
        resourceName, commonLabels, selectorLabels, // Derived names/labels
        &replicas, // Resolved replicas
        *podTemplate, // The built template (dereferenced)
        strategy, // Resolved strategy
    )

    objects := []client.Object{deployment} // Add the built deployment

    // ... (Build Services, CMs, Secrets, PVC objects similarly) ...

    // Return objects list
    return objects, nil

}
*/
