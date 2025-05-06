// pkg/strategy/registry.go
package strategy

import (
	"fmt"
	// Needed by AppBuilderStrategy interface
)

// builderStrategyRegistry stores registered implementations of AppBuilderStrategy.
// Key is the component type name (string from ApplicationComponent.Type).
var builderStrategyRegistry = make(map[string]AppBuilderStrategy)

// reconcileStrategyRegistry stores registered implementations of AppReconcileStrategy.
// Key is the component type name (string).
var reconcileStrategyRegistry = make(map[string]AppReconcileStrategy)

// RegisterAppBuilderStrategy registers a builder strategy for a specific component type.
// This function MUST be called from the init() function of the specific builder strategy package
// (e.g., pkg/builders/gateway/strategy.go) to ensure registration happens before the operator starts.
// Panics if a strategy for the given type is already registered.
func RegisterAppBuilderStrategy(compType string, strategy AppBuilderStrategy) {
	if compType == "" {
		panic("Cannot register builder strategy with an empty component type name")
	}
	if strategy == nil {
		panic(fmt.Sprintf("Cannot register nil builder strategy for component type: %s", compType))
	}
	if _, exists := builderStrategyRegistry[compType]; exists {
		// Or log a warning and allow override? Panic is safer during development.
		panic(fmt.Sprintf("Builder strategy already registered for component type: %s", compType))
	}
	builderStrategyRegistry[compType] = strategy
	// Use fmt.Printf for startup logging as controller logger might not be initialized yet.
	fmt.Printf("INFO: Builder strategy '%T' registered for component type '%s'\n", strategy, compType)
}

// GetAppBuilderStrategy retrieves the registered builder strategy for a component type.
// Called by the main controller to dispatch building logic.
// Returns the strategy implementation and true if found, otherwise nil and false.
func GetAppBuilderStrategy(compType string) (AppBuilderStrategy, bool) {
	strategy, ok := builderStrategyRegistry[compType]
	return strategy, ok
}

// RegisterAppReconcileStrategy registers a reconcile strategy for a specific component type.
// Called from the init() function of the specific reconcile strategy package
// (e.g., pkg/reconcilers/gateway/strategy.go).
// Panics if a strategy for the given type is already registered.
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
	fmt.Printf("INFO: Reconcile strategy '%T' registered for component type '%s'\n", strategy, compType)
}

// GetAppReconcileStrategy retrieves the registered reconcile strategy for a component type.
// Called by the main controller to dispatch reconciliation tasks.
// Returns the strategy implementation and true if found, otherwise nil and false.
func GetAppReconcileStrategy(compType string) (AppReconcileStrategy, bool) {
	strategy, ok := reconcileStrategyRegistry[compType]
	return strategy, ok
}

// Placeholder init - Actual registrations happen in specific strategy packages' init() functions.
func init() {
	fmt.Println("INFO: Strategy registry package initialized.")
	// --- DO NOT ADD REGISTRATIONS HERE ---
	// Registrations MUST occur in the init() function of the package defining the strategy implementation.
	// Example (inside pkg/builders/gateway/strategy.go):
	// func init() { strategy.RegisterAppBuilderStrategy("gateway", &GatewayBuilderStrategy{}) }
}
