// pkg/strategy/builder_strategy.go
package strategy

import (
	"context" // Needed if strategy methods require context

	appv1 "github.com/infinilabs/operator/api/app/v1" // For ApplicationDefinition, ApplicationComponent
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema" // For GVK

	"sigs.k8s.io/controller-runtime/pkg/client" // Needed if strategy needs client
	// Import common types or application specific types if they are inputs/outputs
	// Example: common "github.com/infinilabs/operator/pkg/apis/common"
	// opensearch "github.com/infinilabs/operator/pkg/apis/opensearch/types" // Example for app-specific types
)

// AppBuilderStrategy defines the contract for application-specific logic needed by the K8s object building phase.
// Each application type (like "opensearch", "gateway") that requires building K8s resources implements this interface.
type AppBuilderStrategy interface {
	// BuildObjects builds the necessary K8s objects (Deployment, StatefulSet, Services, CMs, Secrets, etc.)
	// for a specific application component instance.
	// It receives the ApplicationDefinition (as owner context), the component context (AppComp),
	// and the UNMARSHALLED application-specific configuration.
	// The strategy implementation is responsible for:
	//   1. Validating the structure and content of the 'appSpecificConfig' based on application type.
	//   2. Mapping data from 'appSpecificConfig' (and appDef/appComp/ComponentDefinition info)
	//      to the specific K8s resource specs (DeploymentSpec, StatefulSetSpec, PodSpec etc.).
	//   3. Calling generic or application-specific *builder functions* (from pkg/builders)
	//      to construct the concrete K8s object structs (*appsv1.Deployment, *corev1.Service etc.).
	//   4. Returning a list of client.Object interfaces representing the desired resources.
	//   5. Returning the first error encountered during validation or building.
	// Note: Setting OwnerReference is usually done by the caller (controller) after receiving objects.
	BuildObjects(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme, owner client.Object, appDef *appv1.ApplicationDefinition, appComp *appv1.ApplicationComponent, appSpecificConfig interface{}) ([]client.Object, error)

	// GetWorkloadGVK returns the expected primary K8s workload GVK for this application type (e.g., StatefulSet for Opensearch).
	// This is often used by the controller for basic checks or logging.
	GetWorkloadGVK() schema.GroupVersionKind
}

// Strategy registration (Actual registration happens in init() of specific strategy implementations)
// See registry.go
