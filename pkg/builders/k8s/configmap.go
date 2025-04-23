// pkg/builders/k8s/configmap.go
package k8s

import (

	// For String manipulation

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// Needed for Scheme
	// No appv1 or common dependency directly here unless building from those structs
	// common "github.com/infinilabs/operator/pkg/apis/common"
)

// BuildConfigMapsFromAppData builds a list of corev1.ConfigMap objects from AppConfigData.
// AppConfigData is typically map[string]string where key is filename and value is content string.
// It builds one ConfigMap object with all files from the map.
// You could extend this to build multiple ConfigMaps if needed, e.g., one per filename key prefix.
func BuildConfigMapsFromAppData(appConfigData map[string]string, resourceName string, namespace string, labels map[string]string) ([]client.Object, error) { // Return slice of client.Object
	if appConfigData == nil || len(appConfigData) == 0 {
		return []client.Object{}, nil
	} // Nothing to build

	// Ensure required input fields are present (e.g., resourceName, namespace) - Handled by caller
	// resourceName should be unique per application instance per file concept?
	// Or a single CM holds ALL config files for an instance? Let's assume a single CM holds the map data.
	cmName := resourceName // Example naming: CM name matches the resource name (Deployment/StatefulSet name) + suffix?
	// Example: CM name is the component instance name + "-config" suffix
	cmName = resourceName + "-config" // Common convention

	cm := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "ConfigMap"},
		ObjectMeta: BuildObjectMeta(cmName, namespace, labels, nil), // Use general helper for metadata
		Data:       appConfigData,                                   // Assign the map directly as ConfigMap Data (K8s accepts map[string]string)
		// BinaryData is not used here.
	}

	// Need to also ensure a Volume and VolumeMount reference this ConfigMap in the Pod Template.
	// This logic is handled by the Pod template builder, but the name derived here must match the name used there.
	// The app-specific builder needs to orchestrate passing both the CM object AND the derived volume/mount config.

	return []client.Object{cm}, nil // Return list containing the built ConfigMap object
}
