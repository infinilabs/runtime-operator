// pkg/builders/k8s/headlessservice.go
package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Needed if ServiceConfig links to app types
	"github.com/infinilabs/operator/pkg/apis/common" // Common config types (ServiceSpecPart)
	// Common utils (Ptr helpers)
	// You might need these imports if builders require more context than inputs provided.
	// client "sigs.k8s.io/controller-runtime/pkg/client"
	// runtime "k8s.io/apimachinery/pkg/runtime"
)

// BuildHeadlessService builds a corev1.Service resource with ClusterIP=None.
// This builder is typically used in conjunction with StatefulSets.
// It takes inputs derived from common.ServiceSpecPart configuration and metadata/selector context.
func BuildHeadlessService(
	serviceConfig *common.ServiceSpecPart, // Common Service config part (pointer), needed for ports
	serviceMeta metav1.ObjectMeta, // ObjectMeta for the Headless Service resource
	selectorLabels map[string]string, // Labels to select pods for this service (should match workload selector)

	// Pass component context if needed for derived logic or specific ports/conventions
	// appComp *appv1.ApplicationComponent,

) *corev1.Service {
	// Ensure config pointer is non-nil or provide defaults for necessary fields (especially ports).
	if serviceConfig == nil {
		// If Headless Service is required (e.g., by a StatefulSet workload),
		// config must provide port information or specify a default.
		// Let's return a service with empty ports if config is nil or ports are nil/empty,
		// and rely on K8s validation or other logic to handle.
		serviceConfig = &common.ServiceSpecPart{} // Use an empty struct if nil
	}

	// Convert common.PortSpec to K8s corev1.ServicePort list
	k8sPorts := BuildServicePorts(serviceConfig.Ports) // Use common helper in builders.helpers.go

	// Validate: Ports are typically needed for Headless Service DNS records.
	// While K8s *allows* creating a headless service without ports, it might not be useful for discovery.
	// Add validation earlier or log warning here if ports are missing.
	if len(k8sPorts) == 0 {
		// Warning: Building Headless Service with no ports.
	}

	headlessService := &corev1.Service{
		TypeMeta:   metaviz / metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "Service", Group: corev1.SchemeGroupVersion.Group},
		ObjectMeta: serviceMeta, // Use the pre-built metadata (includes Name, Namespace, Labels)

		Spec: corev1.ServiceSpec{
			Selector:  selectorLabels,              // Use the provided selector labels to link Service to Pods
			ClusterIP: corev1.ClusterIPNone,        // REQUIRED for headless service
			Type:      corev1.ServiceTypeClusterIP, // Type MUST be ClusterIP for headless (API server validation enforces)

			// Ports definition based on common PortSpecs (already converted)
			Ports: k8sPorts, // Use the built ports list (even if empty)

			// Optional: PublishNotReadyAddresses is common for headless discovery of pods BEFORE probe passes
			// If this is configurable in common.ServiceSpecPart, add parameter and assign here.
			// e.g., PublishNotReadyAddresses: commonutil.GetBoolPtrValueOrDefault(serviceConfig.PublishNotReadyAddresses, false),
		},
	}

	return headlessService // Return the built headless service object pointer
}

// Note: BuildServiceMetadata helper (used for serviceMeta input) is in builders.helpers.go
