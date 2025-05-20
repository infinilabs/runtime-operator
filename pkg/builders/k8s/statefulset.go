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

// pkg/builders/k8s/statefulset.go
package k8s

import (
	appsv1 "k8s.io/api/apps/v1" // Needed for VolumeClaimTemplate type
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildStatefulSet builds an appsv1.StatefulSet resource.
// It takes fully assembled ObjectMeta and StatefulSetSpec as input.
// The caller (e.g., application-specific builder) is responsible for constructing these specs correctly,
// including PodTemplateSpec and VolumeClaimTemplates.
func BuildStatefulSet(
	stsMeta metav1.ObjectMeta, // ObjectMeta for the StatefulSet resource
	stsSpec appsv1.StatefulSetSpec, // The complete StatefulSet Spec
) *appsv1.StatefulSet {

	// Basic validation: Ensure ServiceName is set if replicas > 0, check selector matches template labels.
	// Caller should ensure these consistencies.

	statefulSet := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "StatefulSet",
		},
		ObjectMeta: stsMeta, // Use the pre-built metadata
		Spec:       stsSpec, // Use the pre-built spec
	}

	return statefulSet // Return the built typed object pointer
}
