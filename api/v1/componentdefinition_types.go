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

// api/v1/componentdefinition_types.go
// Package v1 contains API Schema definitions for the core v1 API group
// +kubebuilder:object:generate=true
// +groupName=core.infini.cloud
package v1

import (
	"github.com/infinilabs/runtime-operator/pkg/apis/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkloadReference uses the common definition.
type WorkloadReference = common.WorkloadReference

// ComponentDefinitionSpec defines the desired state of ComponentDefinition.
// In this simplified model, it primarily identifies the component type and its target workload.
type ComponentDefinitionSpec struct {
	// Workload defines the primary Kubernetes workload kind this component definition primarily maps to
	// (e.g., Deployment, StatefulSet). This informs the controller's building strategy.
	// +kubebuilder:validation:Required
	Workload common.WorkloadReference `json:"workload"`

	// Description is a brief description of the component definition (what this type represents).
	// +optional
	Description string `json:"description,omitempty"`

	// No Defaults field anymore. Default values are handled by Builders based on type.
}

// ComponentDefinitionStatus defines the observed state of ComponentDefinition.
// +kubebuilder:object:generate=true
type ComponentDefinitionStatus struct {
	// Conditions represent the latest available observations of the object's state.
	// Potentially used by validating webhooks (e.g., to check if type is supported).
	// +optional
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" listType:"map" listMapKey:"type"`

	// ObservedGeneration is the most recent generation observed by a controller (if any).
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,path=componentdefinitions,shortName=compdef,categories={infini,core}
//+kubebuilder:printcolumn:name="Workload Kind",type=string,JSONPath=".spec.workload.kind"
//+kubebuilder:printcolumn:name="Description",type=string,JSONPath=".spec.description"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:storageversion

// ComponentDefinition is the Schema for the componentdefinitions API.
// It primarily serves as a type identifier and workload hint for Application components.
type ComponentDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentDefinitionSpec   `json:"spec,omitempty"`
	Status ComponentDefinitionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:object:generate=true

// ComponentDefinitionList contains a list of ComponentDefinition.
type ComponentDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentDefinition{}, &ComponentDefinitionList{})
}
