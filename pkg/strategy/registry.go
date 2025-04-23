// pkg/strategy/registry.go
package strategy

import (
	"fmt"
	"reflect"
)

// This registry stores implementations of AppBuilderStrategy.
// Key is the component type name (string from ApplicationComponent.Type).
var builderStrategyRegistry = make(map[string]AppBuilderStrategy)

// This registry stores implementations of AppReconcileStrategy.
// Key is the component type name (string).
var reconcileStrategyRegistry = make(map[string]AppReconcileStrategy)

// RegisterAppBuilderStrategy registers a builder strategy for a specific component type.
// This function should be called from the init() function of the specific builder package.
func RegisterAppBuilderStrategy(compType string, strategy AppBuilderStrategy) {
	if _, exists := builderStrategyRegistry[compType]; exists {
		panic(fmt.Sprintf("Builder strategy already registered for component type: %s", compType))
	}
	builderStrategyRegistry[compType] = strategy
	fmt.Printf("Builder strategy '%s' registered for type '%s'\n", reflect.TypeOf(strategy).String(), compType) // Log registration
}

// GetAppBuilderStrategy retrieves the registered builder strategy for a component type.
func GetAppBuilderStrategy(compType string) (AppBuilderStrategy, bool) {
	strategy, ok := builderStrategyRegistry[compType]
	return strategy, ok
}

// RegisterAppReconcileStrategy registers a reconcile strategy for a specific component type.
// This function should be called from the init() function of the specific reconcile strategy package.
func RegisterAppReconcileStrategy(compType string, strategy AppReconcileStrategy) {
	if _, exists := reconcileStrategyRegistry[compType]; exists {
		panic(fmt.Sprintf("Reconcile strategy already registered for component type: %s", compType))
	}
	reconcileStrategyRegistry[compType] = strategy
	fmt.Printf("Reconcile strategy '%s' registered for type '%s'\n", reflect.TypeOf(strategy).String(), compType) // Log registration
}

// GetAppReconcileStrategy retrieves the registered reconcile strategy for a component type.
func GetAppReconcileStrategy(compType string) (AppReconcileStrategy, bool) {
	strategy, ok := reconcileStrategyRegistry[compType]
	return strategy, ok
}

// Example of using a Strategy (will be in main controller or task runner)
// Example: Get Strategy for "opensearch" and call its method
/*
import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	appv1 "github.com/infinilabs/operator/api/app/v1"
)

func executeStrategy(ctx context.Context, cli client.Client, appDef *appv1.ApplicationDefinition, appComp *appv1.ApplicationComponent, appSpecificConfig interface{}) error {
    compType := appComp.Type // "opensearch"

    builderStrategy, ok := GetAppBuilderStrategy(compType)
    if !ok {
        return fmt.Errorf("no builder strategy found for type %s", compType)
    }

    // Check workload GVK before building
    // expectedGVK := builderStrategy.GetWorkloadGVK()
    // Compare with what's in ComponentDefinition.Workload field?

    builtObjects, err := builderStrategy.BuildObjects(ctx, cli, nil, nil, appDef, appComp, appSpecificConfig) // Pass nil scheme/owner as context/owner is passed

    if err != nil {
         return fmt.Errorf("builder strategy failed for type %s: %w", compType, err)
    }

    // Now builtObjects contains []client.Object specific to OpenSearch deployment

    reconcileStrategy, ok := GetAppReconcileStrategy(compType)
    if !ok {
         // Handle missing reconcile strategy - default to basic ensure resources strategy?
    }

    // Execute the reconcile strategy's flow (e.g., applying resources, running checks)
    // needsRequeue, reconcileErr := reconcileStrategy.Reconcile(ctx, cli, nil, appDef, nil, builtObjects, nil) // Pass needed state
     return nil // Placeholder

}
*/
