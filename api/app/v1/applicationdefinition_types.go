// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Runtime Operator is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

// api/app/v1/applicationdefinition_types.go
// Package v1 contains API Schema definitions for the app v1 API group
// +kubebuilder:object:generate=true
// +groupName=infini.cloud
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// --- Constants for Annotations ---

const (
	// AnnotationChangeID is the annotation key for tracking change/reconciliation ID
	AnnotationChangeID = "infini.cloud/change-id"
	// AnnotationClusterID is the annotation key for cluster ID
	AnnotationClusterID = "infini.cloud/cluster-id"
	// AnnotationChangeWebhookURL is the annotation key for webhook URL
	AnnotationChangeWebhookURL = "infini.cloud/change-webhook-url"
)

// --- Constants for Phase and Conditions ---

// ApplicationPhase represents the current state of the ApplicationDefinition reconciliation process.
// +kubebuilder:validation:Enum=Pending;Creating;Updating;Running;Degraded;Suspended;Deleting;Failed
type ApplicationPhase string

const (
	// ApplicationPhasePending indicates the application definition is waiting to be processed.
	ApplicationPhasePending ApplicationPhase = "Pending"
	// ApplicationPhaseCreating indicates the controller is processing the definition (e.g., building objects).
	ApplicationPhaseCreating ApplicationPhase = "Creating"
	// ApplicationPhaseUpdateing indicates the controller is applying K8s resources and waiting for them to become ready.
	ApplicationPhaseUpdateing ApplicationPhase = "Updating"
	// ApplicationPhaseRunning indicates all components are reconciled and healthy.
	ApplicationPhaseRunning ApplicationPhase = "Running"
	// ApplicationPhaseDegraded indicates one or more components were previously ready but are now unhealthy or not ready.
	ApplicationPhaseDegraded ApplicationPhase = "Degraded"
	// ApplicationPhaseSuspended indicates the application is intentionally suspended (scaled to zero).
	ApplicationPhaseSuspended ApplicationPhase = "Suspended"
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
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
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

	// Suspend indicates whether the application should be suspended (scaled to 0).
	// When true, all components will be scaled to zero replicas.
	// +optional
	Suspend *bool `json:"suspend,omitempty"`
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

	// SuspendedReplicas records the replica count of components before they were suspended.
	// +optional
	SuspendedReplicas map[string]int32 `json:"suspendedReplicas,omitempty"`

	// LastChangeID records the last change ID that was processed and sent to the webhook.
	// This is used to avoid sending duplicate webhook events for the same change ID.
	// +optional
	LastChangeID string `json:"lastChangeID,omitempty"`

	// Annotations holds additional metadata annotations for the application definition.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// --- Root Object ---

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,path=applicationdefinitions,shortName=appdef,categories={infini,app}
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase",description="The current reconciliation phase of the application."
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status",description="The overall readiness status of the application."
// +kubebuilder:printcolumn:name="Components",type=string,JSONPath=".spec.components[*].name",description="Names of the components defined in the application."
// +kubebuilder:printcolumn:name="Replicas",type=string,JSONPath=".spec.components[*].properties.replicas",description="Configured replica counts for components.",priority=1
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

func init() {
	SchemeBuilder.Register(&ApplicationDefinition{}, &ApplicationDefinitionList{})
}
