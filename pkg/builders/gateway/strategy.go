// pkg/strategy/strategy.go
// Defines generic interfaces and registry for application build and reconcile strategies.
package gateway

import (
	"context"
	"fmt"

	// Import types used ONLY in the interface definitions
	appv1 "github.com/infinilabs/operator/api/app/v1" // For ApplicationComponent etc. in signatures
	"github.com/infinilabs/operator/internal/controller/common/kubeutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema" // For GVK
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// --- Builder Strategy ---

// AppBuilderStrategy defines the contract for application-specific logic during object building.
type AppBuilderStrategy interface {
	// BuildObjects builds K8s objects for a specific component instance.
	BuildObjects(
		ctx context.Context,
		k8sClient client.Client,
		scheme *runtime.Scheme,
		owner client.Object, // Owning AppDef
		appDef *appv1.ApplicationDefinition, // Full AppDef
		appComp *appv1.ApplicationComponent, // Component being processed
		appSpecificConfig interface{}, // Unmarshalled specific config
	) ([]client.Object, error)

	// GetWorkloadGVK returns the expected primary K8s workload GVK managed by this strategy.
	GetWorkloadGVK() schema.GroupVersionKind
}

// builderStrategyRegistry stores registered builder strategies. Key: component type name.
var builderStrategyRegistry = make(map[string]AppBuilderStrategy)

// RegisterAppBuilderStrategy registers a builder strategy for a component type.
// MUST be called from the init() function of the specific builder strategy package.
func RegisterAppBuilderStrategy(compType string, strategy AppBuilderStrategy) {
	if compType == "" {
		panic("Cannot register builder strategy with an empty component type name")
	}
	if strategy == nil {
		panic(fmt.Sprintf("Cannot register nil builder strategy for component type: %s", compType))
	}
	if _, exists := builderStrategyRegistry[compType]; exists {
		panic(fmt.Sprintf("Builder strategy already registered for component type: %s", compType))
	}
	builderStrategyRegistry[compType] = strategy
	fmt.Printf("INFO: Builder strategy '%T' registered for component type '%s'\n", strategy, compType) // Use fmt for init logging
}

// GetAppBuilderStrategy retrieves the registered builder strategy for a component type.
func GetAppBuilderStrategy(compType string) (AppBuilderStrategy, bool) {
	strategy, ok := builderStrategyRegistry[compType]
	return strategy, ok
}

// --- Reconcile Strategy ---

// AppReconcileStrategy defines the contract for orchestrating reconciliation tasks and health checks.
type AppReconcileStrategy interface {
	// Reconcile orchestrates post-build reconciliation tasks.
	Reconcile(
		ctx context.Context,
		k8sClient client.Client,
		scheme *runtime.Scheme,
		appDef *appv1.ApplicationDefinition,
		appComp *appv1.ApplicationComponent,
		componentStatus *appv1.ComponentStatusReference, // Mutable status
		mergedConfig interface{}, // Unmarshalled specific config
		desiredObjects []client.Object, // Built objects (consider passing map?)
		applyResults map[string]kubeutil.ApplyResult, // Results from apply phase
		recorder record.EventRecorder,
	) (needsRequeue bool, err error)

	// CheckAppHealth performs application-level health checks.
	CheckAppHealth(
		ctx context.Context,
		k8sClient client.Client,
		scheme *runtime.Scheme,
		appDef *appv1.ApplicationDefinition,
		appComp *appv1.ApplicationComponent,
		appSpecificConfig interface{}, // Unmarshalled specific config
	) (isHealthy bool, message string, err error)
}

// reconcileStrategyRegistry stores registered reconcile strategies. Key: component type name.
var reconcileStrategyRegistry = make(map[string]AppReconcileStrategy)

// RegisterAppReconcileStrategy registers a reconcile strategy for a component type.
// MUST be called from the init() function of the specific reconcile strategy package.
func RegisterAppReconcileStrategy(compType string, strategy AppReconcileStrategy) {
	if compType == "" {
		panic("Cannot register reconcile strategy with an empty component type name")
	}
	if strategy == nil {
		panic(fmt.Sprintf("Cannot register nil reconcile strategy for component type: %s", compType))
	}
	if _, exists := reconcileStrategyRegistry[compType]; exists {
		panic(fmt.Sprintf("Reconcile strategy already registered for component type: %s", compType))
	}
	reconcileStrategyRegistry[compType] = strategy
	fmt.Printf("INFO: Reconcile strategy '%T' registered for component type '%s'\n", strategy, compType) // Use fmt for init logging
}

// GetAppReconcileStrategy retrieves the registered reconcile strategy for a component type.
func GetAppReconcileStrategy(compType string) (AppReconcileStrategy, bool) {
	strategy, ok := reconcileStrategyRegistry[compType]
	return strategy, ok
}

// Placeholder init - Actual registrations happen elsewhere.
func init() {
	fmt.Println("INFO: Generic strategy registry package initialized.")
	// --- DO NOT ADD REGISTRATIONS HERE ---
}
