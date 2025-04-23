/*
Copyright 2025 infinilabs.com.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// api/app/v1/applicationdefinition_types.go
// Package v1 contains API Schema definitions for the app v1 API group
// +kubebuilder:object:generate=true
// +groupName=app.infini.cloud
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime" // Required for RawExtension
	// Although Common types are not used *directly* in Properties (using RawExtension),
	// it's good practice to import common package in API definitions if related.
	// Or import specific subpackages if they define types needed directly.
	// common "github.com/infinilabs/operator/pkg/apis/common" // Example
)

// ApplicationPhase represents the current state of the ApplicationDefinition reconciliation process.
// +kubebuilder:validation:Enum=Pending;Processing;Applying;Available;Degraded;Deleting;Failed
type ApplicationPhase string

// Constants defining the application phases.
const (
	ApplicationPhasePending    ApplicationPhase = "Pending"    // Initial state, waiting for reconciliation.
	ApplicationPhaseProcessing ApplicationPhase = "Processing" // Operator is working (config unmarshalling, building objects).
	ApplicationPhaseApplying   ApplicationPhase = "Applying"   // Objects are being applied to the cluster.
	ApplicationPhaseAvailable  ApplicationPhase = "Available"  // All components successfully applied and healthy.
	ApplicationPhaseDegraded   ApplicationPhase = "Degraded"   // Applied but some components unhealthy.
	ApplicationPhaseDeleting   ApplicationPhase = "Deleting"   // Application is being deleted.
	ApplicationPhaseFailed     ApplicationPhase = "Failed"     // Unrecoverable error during reconciliation.
)

// ApplicationComponent defines a single component instance within an ApplicationDefinition.
// This links a specific application type (via Type) to its configuration data (Properties).
type ApplicationComponent struct {
	// Name is the unique identifier for this component instance within the ApplicationDefinition.
	// This name will be used to derive resource names. Must be unique within the application definition's components list.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9_.]*)?[a-z0-9]$` // DNS subdomain safe pattern for K8s compatibility
	Name string `json:"name"`

	// Type references the `metadata.name` of a `ComponentDefinition` resource.
	// This indicates the component type (e.g., "opensearch", "gateway").
	// The operator uses this type to determine which application-specific logic (unmarshalling, building strategy) to use.
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// Properties provides the instance-specific configuration as raw JSON/YAML.
	// The structure of this data DEPENDS on the component 'type'.
	// The operator's strategy dispatcher will unmarshal and process this data according to the component type's definition.
	// This allows flexibility in the structure of config for different application types.
	// +kubebuilder:validation:Required // Mark properties as required for a component
	// +kubebuilder:pruning:PreserveUnknownFields // CRITICAL: Preserve fields for the operator to unmarshal dynamically
	// +kubebuilder:validation:Type=object // Validate it's a JSON object at K8s API level
	Properties runtime.RawExtension `json:"properties"` // Use RawExtension for flexibility

	// TODO: Add Traits field if needed later for operational capabilities layered on components.
	// Example: Auto-scaler trait config for this component.
	// +optional Traits []ApplicationTrait `json:"traits,omitempty"` // Needs ApplicationTrait definition

}

// ApplicationDefinitionSpec defines the desired state of an ApplicationDefinition.
// This includes the list of component instances that make up the application.
type ApplicationDefinitionSpec struct {
	// Components lists the constituent component instances of the application.
	// At least one component must be defined. Component names must be unique within this application definition's components list.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1 // Ensure at least one component is listed
	// +kubebuilder:validation:UniqueItems=false // Note: This validation applies to list *elements*.
	//                                           // Unique component 'name' should be validated via a webhook
	//                                           // inspecting list contents or via admission controller logic.
	Components []ApplicationComponent `json:"components"`

	// TODO: Add Application-level configurations like Workflow triggers, Policies applied to all components, Networking overrides etc.
	// These would be outside the individual component's Properties scope.
	// +optional NodeSelector map[string]string `json:"nodeSelector,omitempty"` // Apply NodeSelector to all pods
	// +optional Tolerations []corev1.Toleration `json:"tolerations,omitempty"` // Apply Tolerations to all pods
	// +optional Affinity *corev1.Affinity `json:"affinity,omitempty"` // Apply Affinity to all pods (Pointer)

}

// ComponentStatusReference provides a summary of the observed status of a deployed component's primary resource.
// This structure is part of the ApplicationDefinitionStatus.
// +kubebuilder:object:generate=true // Generate deepcopy code for this specific structure as it's used in Status

type ComponentStatusReference struct {
	// Name matches the component name in the ApplicationDefinitionSpec.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Kind is the type of the primary Kubernetes resource managed for this component (e.g., "Deployment", "StatefulSet").
	// Populated by the controller after building and applying resources for this component instance.
	// +optional
	Kind string `json:"kind,omitempty"`

	// APIVersion is the API version of the primary Kubernetes resource managed for this component (e.g., "apps/v1").
	// Populated by the controller.
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// ResourceName is the name of the primary Kubernetes resource managed for this component in K8s.
	// Populated by the controller after successful application.
	// +optional
	ResourceName string `json:"resourceName,omitempty"`

	// Namespace is the namespace where the primary resource is deployed (should be AppDef's namespace).
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Health indicates the observed health status of the component resource and application logic (if specific checks are performed).
	// Determined by the controller checking the resource status and application-level checks.
	// +optional
	Health bool `json:"health,omitempty"` // Simplified health status: true = healthy/ready, false = unhealthy/not ready/error

	// Message provides additional status details, error messages, or reasons for the current health status or failure.
	// +optional
	Message string `json:"message,omitempty"`
}

// ApplicationDefinitionStatus defines the observed state of ApplicationDefinition.
// This reflects the progress and outcome of the operator's reconciliation process for this application instance.
// +kubebuilder:object:generate=true // Ensure deepcopy is generated for Status as it's a sub-resource
type ApplicationDefinitionStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller for this ApplicationDefinition.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase represents the current overall state of the application reconciliation process.
	// Helps understand where the operator is in the lifecycle.
	// +optional
	Phase ApplicationPhase `json:"phase,omitempty"`

	// Conditions provide detailed status updates on the application's health and progress.
	// Standard conditions like "Ready" (True/False), "Progressing", "Degraded" are recommended.
	// +optional
	// +patchStrategy=merge // Strategy for merging elements in the list.
	// +patchMergeKey=type // Use the 'type' field as the key for merging.
	// +listType=map       // Indicates that this is a list of map-like objects for merging purposes.
	// +listMapKey=type    // Redundant with +patchMergeKey, but often used for clarity/compatibility.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" listType:"map" listMapKey:"type"`

	// Components provides status summaries for each component managed by the ApplicationDefinition.
	// +optional
	// This list is used by Server-Side Apply for status updates. Component name is the merge key.
	// The order in this list might not match the spec.components order.
	// +listType=map    // Allows merging individual component statuses.
	// +listMapKey=name // Use the component instance 'name' as the key for merging.
	Components []ComponentStatusReference `json:"components,omitempty" listType:"map" listMapKey:"name"`

	// TODO: Add fields for reflecting the overall state of specific reconcile tasks or phases if granular status needed.
	// Example: `Tasks` []TaskStatus // Status for complex application-specific workflow tasks.
}

//+kubebuilder:object:root=true // Indicates that this is a root level custom resource type definition.
//+kubebuilder:subresource:status // Enables the /status subresource, allowing updates without modifying spec.
//+kubebuilder:resource:scope=Namespaced,path=applicationdefinitions,shortName=appdef,categories={infini,app} // Resource configuration for Kubernetes. Defines scope, API path, short names, and categories for kubectl.
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase",description="The current phase of the application." // Configure columns for kubectl get output.
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Indicates if the application is ready."
//+kubebuilder:printcolumn:name="Components",type="string",JSONPath=".spec.components[*].name",description="Names of components in the application." // Shows names in a comma-separated string
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="CreationTimestamp is a timestamp representing the server time when this object was created." // Standard age column.
//+kubebuilder:storageversion // Indicates this is the preferred version to store in etcd.

// ApplicationDefinition is the Schema for the applicationdefinitions API.
// It represents a collection of components managed together as a single application instance.
// Users create ApplicationDefinition resources to deploy applications described by ComponentDefinitions.
type ApplicationDefinition struct {
	metav1.TypeMeta   `json:",inline"`            // Provides Kind (ApplicationDefinition) and APIVersion (app.infini.cloud/v1).
	metav1.ObjectMeta `json:"metadata,omitempty"` // Standard K8s object metadata.

	Spec   ApplicationDefinitionSpec   `json:"spec,omitempty"`   // The desired state defined by the user.
	Status ApplicationDefinitionStatus `json:"status,omitempty"` // The observed state reported by the operator.
}

// +kubebuilder:object:root=true // Indicates that this is a list of root objects (for List API calls).
// +kubebuilder:object:generate=true // Ensure deepcopy is generated for this list type.

// ApplicationDefinitionList contains a list of ApplicationDefinition.
type ApplicationDefinitionList struct {
	metav1.TypeMeta `json:",inline"`            // Provides TypeMeta for the List type itself (e.g., Kind: List).
	metav1.ListMeta `json:"metadata,omitempty"` // Standard K8s list metadata (e.g., ResourceVersion, Continue).
	Items           []ApplicationDefinition     `json:"items"` // The actual list of ApplicationDefinition objects.
}

// AddScheme adds the ApplicationDefinition types to the given scheme.
// This function is typically called from main.go to register your types with the controller-runtime manager's scheme.
// It is generated by Kubebuilder if using +kubebuilder:object:root and +kubebuilder:object:generate.
func AddScheme(scheme *runtime.Scheme) error {
	// SchemeBuilder is created implicitly for each package that defines root objects using +kubebuilder:object:root
	// It registers types within its package to itself using init().
	// We just need to add all types registered with *this package's* SchemeBuilder to the provided scheme.
	return SchemeBuilder.AddToScheme(scheme)
}

// init() function is automatically called when the package is imported.
// It is used here to register the custom types (ApplicationDefinition and ApplicationDefinitionList) with the SchemeBuilder.
// This makes them available for registration in main.go via AddScheme.
func init() {
	// Register the custom root type and its corresponding list type with the SchemeBuilder.
	// SchemeBuilder is package-scoped.
	SchemeBuilder.Register(&ApplicationDefinition{}, &ApplicationDefinitionList{})
}
