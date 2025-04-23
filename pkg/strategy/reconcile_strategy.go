// pkg/strategy/reconcile_strategy.go
package strategy

import (
	"context"

	appv1 "github.com/infinilabs/operator/api/app/v1" // App types
	"sigs.k8s.io/controller-runtime/pkg/client"       // Client access
)

// AppReconcileStrategy defines the contract for orchestrating the reconciliation flow for an application type.
// It determines the sequence of tasks to be performed.
// In a simple implementation, this might just provide flags or data needed for health checks.
// In a complex one, it defines a list of Task types to run.
type AppReconcileStrategy interface {
	// CheckAppHealth performs application-level health check (beyond K8s readiness).
	// Receives the ApplicationDefinition context, specific component context, client.
	// It should retrieve necessary application objects/endpoints from K8s and perform app-specific checks.
	// Returns isHealthy (bool), a descriptive message (string), and an error *during the check process* itself.
	// +kubebuilder:validation:Required
	CheckAppHealth(ctx context.Context, k8sClient client.Client, appDef *appv1.ApplicationDefinition, appComp *appv1.ApplicationComponent) (bool, string, error)

	// Optional: Define methods for getting task lists, e.g.,
	// GetReconcileTasks() []string // Returns a list of task type names to run
}
