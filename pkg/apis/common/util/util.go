// pkg/apis/common/util/util.go
// Package util provides common utility functions for the common API types and general K8s interactions.
package util

import (
	// Keep if future helpers need context
	"encoding/json" // Needed for UnmarshalAppSpecificConfig
	"fmt"

	// Keep if future helpers use reflection
	// Keep for string helpers
	// Keep if future helpers need time
	// For standard types referenced by helpers if needed
	// For Quantity if helpers handle it
	"k8s.io/apimachinery/pkg/runtime" // For runtime.RawExtension and Scheme in Unmarshal helper
	// Needed for runtime.Decode/Encode in Unmarshal helper
	// Keep if future helpers need IntOrString
	// Import common types ONLY IF helpers need to operate directly on them
	// For UnmarshalAppSpecificConfig, we need the target struct types.
	"github.com/infinilabs/operator/pkg/apis/common"
)

// --- Ptr Helpers ---
// Functions to get pointers to primitive values.
func Int32Ptr(val int32) *int32    { return &val }
func BoolPtr(val bool) *bool       { return &val }
func StringPtr(val string) *string { return &val }

// --- Get pointer values, return default if pointer is nil ---
// Safely dereference pointers, providing a default value if the pointer is nil.

// GetInt32ValueOrDefault returns the value of an int32 pointer or a default value.
func GetInt32ValueOrDefault(ptr *int32, defaultValue int32) int32 {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// GetBoolValueOrDefault returns the value of a bool pointer or a default value.
func GetBoolValueOrDefault(ptr *bool, defaultValue bool) bool {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// GetStringValueOrDefault returns the value of a string pointer or a default value.
func GetStringValueOrDefault(ptr *string, defaultValue string) string {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// --- Configuration Unmarshalling Helper ---
// Unmarshals raw ApplicationComponent.Properties based on the component type string
// into the correct specific Go configuration struct pointer.
// Returns the unmarshalled struct pointer (as interface{}) or nil if no properties are provided.
// Returns error if unmarshalling fails for the given type or type is unknown.
func UnmarshalAppSpecificConfig(appCompType string, rawProperties runtime.RawExtension) (interface{}, error) {
	if len(rawProperties.Raw) == 0 {
		// Return nil if no properties provided. Builders should handle defaults for nil config.
		return nil, nil
	}

	var specificConfig interface{} // Target Go struct pointer

	// --- Type Mapping based on component type string ---
	// Add cases for ALL supported application types.
	//switch appCompType {
	//case "opensearch":
	//	specificConfig = &common.OpensearchClusterConfig{} // Must be defined in common.types
	//case "elasticsearch":
	//	specificConfig = &common.ElasticsearchClusterConfig{} // Must be defined in common.types
	//case "gateway":
	//	specificConfig = &common.ResourceConfig{} // Must be defined in common.types
	//// case "console": specificConfig = &common.ConsoleConfig{} // Add ConsoleConfig etc.
	//
	//default: // Unsupported component type
	//	return nil, fmt.Errorf("unsupported component type '%s' for configuration unmarshalling", appCompType)
	//}

	// 不需要区分不同的type
	specificConfig = &common.ResourceConfig{}

	// --- Perform Unmarshalling ---
	// Use json.Unmarshal as RawExtension contains JSON/YAML bytes.
	if err := json.Unmarshal(rawProperties.Raw, specificConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal properties for component type '%s' into expected type %T: %w", appCompType, specificConfig, err)
	}

	// Optional: Add post-unmarshalling validation specific to the type here if needed.

	return specificConfig, nil // Return the pointer to the unmarshalled specific struct
}

// --- Merge Logic (Removed - No longer needed in this util package) ---

// --- Other Potential Utils ---

// MergeStringMaps merges two maps[string]string. Override keys take precedence.
func MergeStringMaps(base map[string]string, override map[string]string) map[string]string {
	if base == nil && override == nil {
		return nil
	}
	// Return deep copies to avoid modifying originals if they might be reused.
	if override == nil {
		newMap := make(map[string]string, len(base))
		for k, v := range base {
			newMap[k] = v
		}
		return newMap
	}
	if base == nil {
		newMap := make(map[string]string, len(override))
		for k, v := range override {
			newMap[k] = v
		}
		return newMap
	}

	// Create a new map starting with base
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

// ContainsString checks if a string exists in a slice.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// AddScheme - Not typically needed in common util package.
// func AddScheme(scheme *runtime.Scheme) error { return nil }

// init - Not typically needed in common util package.
// func init() {}
