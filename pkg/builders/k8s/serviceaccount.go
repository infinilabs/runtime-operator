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

// pkg/builders/k8s/serviceaccount.go
package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// Need access to helpers if used
	// "github.com/infinilabs/runtime-operator/pkg/apis/common" // Not directly needed here
	// commonutil "github.com/infinilabs/runtime-operator/pkg/apis/common/util"
)

// BuildServiceAccount builds a corev1.ServiceAccount resource.
// Expects pre-derived name, namespace, labels, and annotations.
func BuildServiceAccount(
	saMeta metav1.ObjectMeta, // Pre-built metadata (includes derived name, namespace, labels, annotations)
	// Optional: ImagePullSecrets, Secrets if ServiceAccount config structure included them
	// imagePullSecrets []corev1.LocalObjectReference,
	// secrets []corev1.ObjectReference,
) *corev1.ServiceAccount {

	sa := &corev1.ServiceAccount{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "ServiceAccount"},
		ObjectMeta: saMeta, // Use pre-built metadata

		// Assign optional fields if provided
		// ImagePullSecrets: imagePullSecrets,
		// Secrets: secrets,
		// AutomountServiceAccountToken: // Add if configurable
	}

	return sa // Return the built Service Account object pointer
}
