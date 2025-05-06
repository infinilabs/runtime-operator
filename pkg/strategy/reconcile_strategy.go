// pkg/strategy/reconcile_strategy.go
package strategy

import (
	"context"

	appv1 "github.com/infinilabs/operator/api/app/v1"                    // App types
	"github.com/infinilabs/operator/internal/controller/common/kubeutil" // For ApplyResult type

	// Common Task Interface/Types
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record" // For Recorder
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AppReconcileStrategy defines the contract for orchestrating the reconciliation flow
// and performing application-specific health checks for a particular application type.
// It typically defines a sequence of reconcile tasks to be executed.
type AppReconcileStrategy interface {
	// Reconcile orchestrates the specific reconciliation tasks (post-build and initial apply) for this application type.
	// It receives the main reconcile context, the objects built by the builder strategy,
	// the results from the initial apply phase, and other necessary state information.
	// This method typically defines a list of common.Task implementations and uses a TaskRunner
	// to execute them sequentially.
	//
	// Parameters:
	//   - ctx: Go context.
	//   - k8sClient: Controller's Kubernetes client.
	//   - scheme: Runtime scheme.
	//   - appDef: The owner ApplicationDefinition resource.
	//   - appComp: The specific ApplicationComponent being processed.
	//   - componentStatus: Pointer to the mutable status entry for this component. Tasks update this.
	//   - mergedConfig: Unmarshalled application-specific configuration struct pointer (e.g., *common.GatewayConfig).
	//   - desiredObjects: Slice of all desired K8s objects built by the builder strategy.
	//   - applyResults: Map containing results from the initial apply phase (Key: GVKString/NsName).
	//   - recorder: Event recorder.
	//
	// Returns:
	//   - bool: True if requeuing is needed (e.g., a task returned Pending), False otherwise.
	//   - error: The first critical error encountered during task execution. Returning an error signals overall reconcile failure.
	Reconcile(
		ctx context.Context,
		k8sClient client.Client,
		scheme *runtime.Scheme,
		appDef *appv1.ApplicationDefinition,
		appComp *appv1.ApplicationComponent,
		componentStatus *appv1.ComponentStatusReference,
		mergedConfig interface{},
		desiredObjects []client.Object,
		applyResults map[string]kubeutil.ApplyResult, // Pass apply results map
		recorder record.EventRecorder, // Pass recorder
	) (bool, error)

	// CheckAppHealth performs an application-level health check (beyond basic K8s resource readiness).
	// This method is typically called by the main controller *after* the Reconcile method above
	// has successfully completed (or reported Pending) and basic K8s resource health checks have passed.
	// It requires access to the specific application configuration to know *how* to check health (e.g., API endpoints, expected status).
	//
	// Parameters:
	//   - ctx: Go context.
	//   // Add other necessary parameters like scheme, owner, appDef, appComp, appSpecificConfig
	//   - k8sClient: Controller's Kubernetes client.
	//   - scheme: Runtime scheme.
	//   - appDef: The owner ApplicationDefinition resource.
	//   - appComp: The specific ApplicationComponent being checked.
	//   - appSpecificConfig: The UNMARSHALLED application-specific configuration struct pointer.
	//
	// Returns:
	//   - bool: True if the application is considered healthy at the application level, False otherwise.
	//   - string: A descriptive message about the application health status.
	//   - error: An error if the health check process itself failed (e.g., cannot connect to API).
	CheckAppHealth(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme, appDef *appv1.ApplicationDefinition, appComp *appv1.ApplicationComponent, appSpecificConfig interface{}) (bool, string, error)

	// TODO: Optional methods for future expansion:
	// GetCleanupTasks() []common_reconcilers.Task // List of tasks to run during finalizer cleanup
	// ValidateConfig(config interface{}) error // Perform deep validation on appSpecificConfig
	// GetUpgradeTasks(fromVersion, toVersion string) []common_reconcilers.Task // Tasks for version upgrades
}
