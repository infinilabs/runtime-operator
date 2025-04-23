// pkg/apis/common/util/util.go
// Package util provides common utility functions for the common API types.
package util

import (
	// If helpers need context
	"encoding/json" // Needed for unmarshalling specific config types from RawExtension
	"fmt"

	// If needed for type assertions or reflection-based helpers

	"k8s.io/apimachinery/pkg/runtime" // Needed for DeepCopy, Unstructured conversion
	// Needed for runtime.Decode/Encode
	// Needed for IntOrString
	"github.com/infinilabs/operator/pkg/apis/common" // Import common types
)

// --- Ptr Helpers ---
// Simple helper functions to get a pointer to a primitive value.
func Int32Ptr(val int32) *int32    { return &val }
func BoolPtr(val bool) *bool       { return &val }
func StringPtr(val string) *string { return &val }

// Get pointer values, return default if pointer is nil
func GetInt32ValueOrDefault(ptr *int32, defaultValue int32) int32 {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}
func GetBoolValueOrDefault(ptr *bool, defaultValue bool) bool {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}
func GetStringValueOrDefault(ptr *string, defaultValue string) string {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// --- Configuration Unmarshalling Helper ---

// UnmarshalAppSpecificConfig unmarshals ApplicationComponent.Properties (RawExtension)
// based on the component type string into the correct specific struct type.
// Returns the unmarshalled struct pointer or nil if no properties are provided.
// Returns error if unmarshalling fails for the given type.
func UnmarshalAppSpecificConfig(appCompType string, rawProperties runtime.RawExtension, scheme *runtime.Scheme) (interface{}, error) {
	// If Raw is empty, return nil as no properties are provided
	if len(rawProperties.Raw) == 0 {
		// It's possible specific applications need a default non-nil config struct even if properties are empty.
		// Builder logic for that type should handle this by applying its own internal defaults to a *nil* specific config pointer.
		// Or create an empty default struct here? No, let the builder decide default value based on its internal convention.
		return nil, nil
	}

	// Determine the target specific struct type based on the component type string.
	// These types must be defined in pkg/apis/common and be registerable or handled by a CodecFactory.
	var specificConfig interface{} // This will hold the unmarshalled specific struct pointer

	// --- Type Mapping ---
	// Need to know the concrete type to unmarshal into based on the string identifier.
	// Using a map to look up type names is one way, but it requires type registration.
	// Simpler: Use a switch on the appCompType string and target a pointer to the correct specific struct.
	// For the target struct to be unmarshallable, its fields must match the JSON/YAML data.

	switch appCompType {
	case "opensearch":
		// Define the target specific struct type pointer. Needs *common.OpensearchClusterConfig defined.
		specificConfig = &common.OpensearchClusterConfig{} // target specific struct type
		// Error if target struct wasn't added to scheme with a dummy GVK if needed by runtime.Decode.
		// Simpler with json.Unmarshal from Raw bytes.
	case "gateway":
		// Define the target specific struct type pointer. Needs *common.GatewayConfig defined.
		specificConfig = &common.GatewayConfig{} // target specific struct type

	// Add cases for other supported component types. Ensure their structures are defined in common.types.go.
	// case "console": specificConfig = &common.ConsoleConfig{}

	default: // Unsupported component type
		// Returning an error here will stop the reconcile for this component.
		return nil, fmt.Errorf("unsupported component type '%s' for configuration unmarshalling", appCompType)
	}

	// --- Perform Unmarshalling from RawExtension bytes into the specific struct pointer ---
	// RawExtension contains JSON bytes. Use json.Unmarshal or runtime.Decode if using Scheme+codecs.
	// Json.Unmarshal is often simpler for RawExtension -> Specific Struct.
	if err := json.Unmarshal(rawProperties.Raw, specificConfig); err != nil {
		// Provide error message including the expected type
		return nil, fmt.Errorf("failed to unmarshal properties for component type '%s' into expected type %T: %w", appCompType, specificConfig, err)
	}

	// --- Optional: Add basic validation on the specific struct after unmarshalling ---
	// E.g., check required fields defined within the specific struct using code.
	// If the specific struct uses "+kubebuilder:validation:Required" for fields, standard Go zero-value check isn't enough.
	// Validation logic specific to the content goes here or within a dedicated validation helper/webhook.
	// Example (Placeholder validation call):
	// if validationErr := validateSpecificConfig(specificConfig, appCompType); validationErr != nil {
	//      return nil, fmt.Errorf("validation failed for component type '%s' config: %w", appCompType, validationErr)
	// }

	// Return the unmarshalled specific configuration struct pointer.
	return specificConfig, nil
}

// --- Basic Merge Helper Functions (These are typically internal to specific Builders now) ---
// Removed the Composite MergeComponentConfigs and generic nested merge helpers.
// Merge logic should reside where config fields are *used* to build objects.
// However, simple helpers like merging maps or appending slices are useful.

// MergeStringMaps merges two maps[string]string. Override keys take precedence.
func MergeStringMaps(base map[string]string, override map[string]string) map[string]string {
	// Handle nil maps safely.
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	// Create a copy of base
	merged := make(map[string]string, len(base))
	for k, v := range base {
		merged[k] = v
	}

	// Add/override with keys from override
	for k, v := range override {
		merged[k] = v
	}
	return merged
}

// AppendStringSlices appends override slice to base slice.
func AppendStringSlices(base []string, override []string) []string {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}
	return append(base, override...) // Append base to override
}

// --- Example Helper function definition if specific merge helpers were needed (placeholder) ---
// func mergeImageSpec(base *common.ImageSpec, override *common.ImageSpec) *common.ImageSpec { ... }

// AddScheme - Not needed in common utils package. Registration happens in CRD packages.
// init - Not needed in common utils package.
