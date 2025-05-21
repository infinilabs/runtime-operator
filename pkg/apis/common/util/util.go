package util

import (
	"encoding/json"
	"fmt"

	"github.com/infinilabs/runtime-operator/pkg/apis/common"
	"k8s.io/apimachinery/pkg/runtime"
)

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
	specificConfig = &common.ResourceConfig{}

	// --- Perform Unmarshalling ---
	// Use json.Unmarshal as RawExtension contains JSON/YAML bytes.
	if err := json.Unmarshal(rawProperties.Raw, specificConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal properties for component type '%s' into expected type %T: %w", appCompType, specificConfig, err)
	}

	// Optional: Add post-unmarshalling validation specific to the type here if needed.

	return specificConfig, nil // Return the pointer to the unmarshalled specific struct
}
