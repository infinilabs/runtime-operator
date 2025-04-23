// pkg/builders/k8s/service.go
package k8s

import (

	// Needed for lowercase/replace operations

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Needed for IntOrString in ServicePort TargetPort
	// Not used directly, but in related helpers
	"github.com/infinilabs/operator/pkg/apis/common" // Common config types
	// Common utils (ptr helpers)
)

// BuildService builds a standard corev1.Service resource (ClusterIP, NodePort, or LoadBalancer).
// It builds the service spec based on common.ServiceSpecPart configuration.
func BuildService(
	serviceConfig *common.ServiceSpecPart, // Common Service config part (pointer)
	serviceMeta metav1.ObjectMeta, // ObjectMeta for the Service resource
	selectorLabels map[string]string, // Labels to select pods for this service
) *corev1.Service {
	// Ensure config pointer is non-nil, provide defaults for necessary fields
	if serviceConfig == nil {
		// Service config missing but BuildService was called? Error or return minimal Service?
		// Let's return nil service and handle in caller.
		return nil // Indicates no service config provided
	}

	// Determine service type, default to ClusterIP
	serviceType := corev1.ServiceTypeClusterIP // Default
	if serviceConfig.Type != nil {
		serviceType = *serviceConfig.Type // Use value if pointer non-nil
	}

	// Convert common.PortSpec to K8s corev1.ServicePort list
	k8sPorts := BuildServicePorts(serviceConfig.Ports) // Uses helper in builders.helpers.go

	// Validate: Ports must be defined if type is not ExternalName
	if serviceType != corev1.ServiceTypeExternalName && (k8sPorts == nil || len(k8sPorts) == 0) {
		// Log a warning or error. A Service needs ports!
		// Return nil and expect caller to handle this.
		return nil // Error logged in caller is better, as we don't have component context here.
	}

	service := &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "Service", Group: corev1.SchemeGroupVersion.Group},
		ObjectMeta: serviceMeta, // Use the pre-built metadata
		Spec: corev1.ServiceSpec{
			Selector: selectorLabels, // Use the provided selector labels
			Type:     serviceType,    // Set service type

			// Ports definition based on common PortSpecs (already converted)
			Ports: k8sPorts,

			// Optional K8s spec fields based on common.ServiceSpecPart
			// ClusterIP: serviceConfig.ClusterIP, // if serviceConfig.ClusterIP is *string
			// SessionAffinity: ... // if added to ServiceSpecPart

		},
	}

	// Specific K8s fields based on Service Type (e.g. NodePort values for Type=NodePort)
	// Note: common.ServiceSpecPart allows explicit NodePorts using a map.
	// Need to ensure consistency with ports list generation. If port has a name, map name->NodePort.
	if serviceType == corev1.ServiceTypeNodePort && serviceConfig.NodePorts != nil && len(serviceConfig.NodePorts) > 0 {
		// Iterate over K8s service ports already added
		for i := range k8sPorts {
			k8sPort := &k8sPorts[i] // Get pointer to modify in place
			// Check if common config provided a NodePort for this specific port by name
			if nodePort, exists := serviceConfig.NodePorts[k8sPort.Name]; exists {
				k8sPort.NodePort = nodePort // Set the specific node port
			}
			// If no NodePort provided for a given port name, K8s will auto-assign.
		}
		service.Spec.Ports = k8sPorts // Assign back potentially modified slice
	}

	return service // Return the built service object pointer
}

// BuildHeadlessService builds a corev1.Service with ClusterIP=None.
// This builder is typically used for StatefulSets.
func BuildHeadlessService(
	serviceConfig *common.ServiceSpecPart, // Common Service config part (pointer, needed for ports)
	serviceMeta metav1.ObjectMeta, // ObjectMeta for the Headless Service resource
	selectorLabels map[string]string, // Labels to select pods for this service (usually matching StatefulSet)
) *corev1.Service {

	// Ensure config pointer is non-nil, provide defaults for necessary fields
	if serviceConfig == nil {
		serviceConfig = &common.ServiceSpecPart{}
	} // Use an empty struct if nil

	// Convert common.PortSpec to K8s corev1.ServicePort list
	k8sPorts := BuildServicePorts(serviceConfig.Ports) // Uses helper

	// Validate: Ports are essential for Headless Service DNS records
	if k8sPorts == nil || len(k8sPorts) == 0 {
		// Log warning or error. Headless service is often useless without ports for discovery.
		// Return nil service? Or build without ports and let K8s report problem?
		// Let's build with empty ports list and rely on K8s validation/status.
	}

	headlessService := &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "Service"},
		ObjectMeta: serviceMeta, // Use the pre-built metadata
		Spec: corev1.ServiceSpec{
			Selector:  selectorLabels,              // Use the provided selector labels
			ClusterIP: corev1.ClusterIPNone,        // REQUIRED for headless
			Type:      corev1.ServiceTypeClusterIP, // Type MUST be ClusterIP for headless
			Ports:     k8sPorts,                    // Assign ports (even if empty list)

			// Optional: PublishNotReadyAddresses is common for headless discovery of pods before probes pass
			// PublishNotReadyAddresses: // Add parameter for this in ServiceSpecPart?

		},
	}
	return headlessService // Return the built headless service object pointer
}

// BuildServiceMetadata creates standard Kubernetes ObjectMeta for a Service.
// This is separated from the main builder functions if metadata naming logic is common but service spec logic differs.
func BuildServiceMetadata(name string, namespace string, labels map[string]string) metav1.ObjectMeta {
	// Annotations might be needed here too. Add annotations parameter if required.
	return BuildObjectMeta(name, namespace, labels, nil) // Use generic ObjectMeta builder helper (needs annotations if needed)
}
