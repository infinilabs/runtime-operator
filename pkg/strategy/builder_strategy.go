// pkg/strategy/builder_strategy.go
package strategy

import (
	"context" // Needed for methods

	appv1 "github.com/infinilabs/operator/api/app/v1" // For ApplicationDefinition, ApplicationComponent
	"k8s.io/apimachinery/pkg/runtime"                 // For Scheme
	"k8s.io/apimachinery/pkg/runtime/schema"          // For GVK

	"sigs.k8s.io/controller-runtime/pkg/client" // Needed if strategy needs client
)

// AppBuilderStrategy defines the contract for application-specific logic
// needed during the Kubernetes object building phase of reconciliation.
// Each supported application type (e.g., "opensearch", "gateway") needs a
// concrete implementation of this interface registered in the registry.
type AppBuilderStrategy interface {
	// BuildObjects builds the necessary K8s objects (Deployment, StatefulSet, Services, CMs, Secrets, etc.)
	// for a specific application component instance based on its configuration.
	//
	// Parameters:
	//   - ctx: Go context.
	//   - k8sClient: Controller's Kubernetes client.
	//   - scheme: Runtime scheme for object GVK lookup.
	//   - owner: The owning ApplicationDefinition resource (used for OwnerReferences).
	//   - appDef: The full ApplicationDefinition resource.
	//   - appComp: The specific ApplicationComponent being processed.
	//   - appSpecificConfig: The UNMARSHALLED application-specific configuration struct
	//     (e.g., *common.GatewayConfig, *common.OpensearchClusterConfig) corresponding to appComp.Type.
	//     The implementation MUST type assert this interface{} to its expected concrete type.
	//
	// Returns:
	//   - []client.Object: A slice of Kubernetes objects (e.g., *appsv1.StatefulSet, *corev1.Service)
	//     that represent the desired state for this component instance. Builders should ensure
	//     these objects have correct TypeMeta (GVK) and essential ObjectMeta (Name, Namespace, Labels).
	//     OwnerReferences will be set by the main controller after this method returns.
	//   - error: The first critical error encountered during config validation or object building.
	//     Returning an error here will typically cause the reconciliation for the *entire* ApplicationDefinition
	//     to fail and requeue.
	BuildObjects(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme, owner client.Object, appDef *appv1.ApplicationDefinition, appComp *appv1.ApplicationComponent, appSpecificConfig interface{}) ([]client.Object, error)

	// GetWorkloadGVK returns the expected primary K8s workload GVK (e.g., StatefulSet for Opensearch)
	// managed by this application type strategy. This should match the ComponentDefinition.
	// Used by the controller for informational purposes or validation.
	GetWorkloadGVK() schema.GroupVersionKind
}
