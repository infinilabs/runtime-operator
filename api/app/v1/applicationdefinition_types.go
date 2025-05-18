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
	"k8s.io/apimachinery/pkg/runtime"
)

// --- Constants for Phase and Conditions ---

// ApplicationPhase represents the current state of the ApplicationDefinition reconciliation process.
// +kubebuilder:validation:Enum=Pending;Processing;Applying;Available;Degraded;Deleting;Failed
type ApplicationPhase string

const (
	// ApplicationPhasePending indicates the application definition is waiting to be processed.
	ApplicationPhasePending ApplicationPhase = "Pending"
	// ApplicationPhaseProcessing indicates the controller is processing the definition (e.g., building objects).
	ApplicationPhaseProcessing ApplicationPhase = "Processing"
	// ApplicationPhaseApplying indicates the controller is applying K8s resources and waiting for them to become ready.
	ApplicationPhaseApplying ApplicationPhase = "Applying"
	// ApplicationPhaseAvailable indicates all components are reconciled and healthy.
	ApplicationPhaseAvailable ApplicationPhase = "Available"
	// ApplicationPhaseDegraded indicates one or more components were previously ready but are now unhealthy or not ready.
	ApplicationPhaseDegraded ApplicationPhase = "Degraded"
	// ApplicationPhaseDeleting indicates the application definition is being deleted.
	ApplicationPhaseDeleting ApplicationPhase = "Deleting"
	// ApplicationPhaseFailed indicates a critical error occurred during reconciliation.
	ApplicationPhaseFailed ApplicationPhase = "Failed"
)

// ConditionType is a string alias for standard condition types.
type ConditionType string

const (
	// ConditionReady signifies that the application as a whole is ready and available.
	// Its status reflects the overall health based on all components.
	ConditionReady ConditionType = "Ready"
	// Add other standard condition types if needed, e.g., "Progressing"
)

// --- Component Definition ---

// ApplicationComponent defines a single component instance within an ApplicationDefinition.
type ApplicationComponent struct {
	// Name is the unique identifier for this component instance within the ApplicationDefinition.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9_.]*)?[a-z0-9]$`
	Name string `json:"name"`

	// Type references the `metadata.name` of a `ComponentDefinition` resource in the same namespace.
	Type string `json:"type,omitempty"`

	// Properties provides the instance-specific configuration as raw JSON.
	// The structure is determined by the component 'type' and validated by the corresponding builder strategy.
	// +kubebuilder:validation:Required
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	Properties runtime.RawExtension `json:"properties"`
}

// --- Spec and Status ---

// ApplicationDefinitionSpec defines the desired state of an ApplicationDefinition.
type ApplicationDefinitionSpec struct {
	// Components lists the desired component instances for this application.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	Components []ApplicationComponent `json:"components"`
}

// ComponentStatusReference provides a summary of the status of a deployed component's primary resource.
// +kubebuilder:object:generate=true
type ComponentStatusReference struct {
	// Name matches the name of the component in the spec.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Kind is the Kubernetes Kind of the primary workload resource managed for this component (e.g., StatefulSet, Deployment).
	// Derived from the corresponding ComponentDefinition.
	// +optional
	Kind string `json:"kind,omitempty"`

	// APIVersion is the API version of the primary workload resource (e.g., apps/v1).
	// Derived from the corresponding ComponentDefinition.
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// ResourceName is the actual name of the primary workload resource created in Kubernetes.
	// +optional
	ResourceName string `json:"resourceName,omitempty"`

	// Namespace is the namespace where the primary workload resource resides (usually the same as the AppDef).
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Health indicates the observed health status of the component (considering both K8s readiness and app-level checks).
	// True means healthy, False means unhealthy or not yet ready.
	// +optional
	Health bool `json:"health,omitempty"`

	// Message provides a human-readable status message or error details for the component.
	// +optional
	Message string `json:"message,omitempty"`
}

// ApplicationDefinitionStatus defines the observed state of ApplicationDefinition.
// +kubebuilder:object:generate=true
type ApplicationDefinitionStatus struct {
	// ObservedGeneration reflects the generation of the ApplicationDefinition spec that was last processed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase represents the current overall state of the application reconciliation.
	// +optional
	Phase ApplicationPhase `json:"phase,omitempty"`

	// Conditions provide observations of the application's state.
	// Known condition types include "Ready".
	// +optional
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Components provides a summary status for each component defined in the spec.
	// +optional
	// +listType=map
	// +listMapKey=name
	Components []ComponentStatusReference `json:"components,omitempty" listType:"map" listMapKey:"name"`
}

// --- Root Object ---

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,path=applicationdefinitions,shortName=appdef,categories={infini,app}
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase",description="The current reconciliation phase of the application."
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status",description="The overall readiness status of the application."
// +kubebuilder:printcolumn:name="Components",type=string,JSONPath=".spec.components[*].name",description="Names of the components defined in the application."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:storageversion

// ApplicationDefinition is the Schema for the applicationdefinitions API, defining a composite application.
type ApplicationDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationDefinitionSpec   `json:"spec,omitempty"`
	Status ApplicationDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationDefinitionList contains a list of ApplicationDefinition.
type ApplicationDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationDefinition `json:"items"`
}

// AddScheme adds the ApplicationDefinition types to the given scheme.
// Deprecated: Use SchemeBuilder.AddToScheme directly.
func AddScheme(scheme *runtime.Scheme) error {
	return SchemeBuilder.AddToScheme(scheme)
}

func init() {
	SchemeBuilder.Register(&ApplicationDefinition{}, &ApplicationDefinitionList{})
}
