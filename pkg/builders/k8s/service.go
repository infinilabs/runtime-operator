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

// pkg/builders/k8s/service.go
package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// common "github.com/infinilabs/runtime-operator/pkg/apis/common" // Common types if needed for internal helpers
	// commonutil "github.com/infinilabs/runtime-operator/pkg/apis/common/util" // Common utils if needed
)

// BuildService builds a standard corev1.Service resource (ClusterIP, NodePort, or LoadBalancer).
// It builds the service spec based on common.ServiceSpecPart configuration passed from specific builders.
// Requires pre-built metadata and selector labels.
func BuildService(
	serviceMeta metav1.ObjectMeta, // ObjectMeta for the Service resource
	selectorLabels map[string]string, // Labels to select pods for this service
	serviceSpec corev1.ServiceSpec, // The complete desired Service Spec (Type, Ports, ClusterIP etc already set)
) *corev1.Service {

	// Basic Validation: Check selector, ensure ports are defined if not ExternalName.
	// Caller (App-specific builder) should ensure serviceSpec is correctly populated based on config.

	service := &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "Service"},
		ObjectMeta: serviceMeta, // Use the pre-built metadata
		Spec:       serviceSpec, // Use the pre-built spec
	}

	return service // Return the built service object pointer
}

// BuildHeadlessService builds a corev1.Service with ClusterIP=None.
// It requires pre-built metadata, selector, and the list of ports to expose.
func BuildHeadlessService(
	serviceMeta metav1.ObjectMeta, // ObjectMeta for the Headless Service resource
	selectorLabels map[string]string, // Labels to select pods for this service (should match workload selector)
	ports []corev1.ServicePort, // List of K8s ServicePort structs derived from config
	// publishNotReadyAddresses bool, // Optional parameter if needed
) *corev1.Service {

	// Ensure ports list is not nil (can be empty, but not nil)
	if ports == nil {
		ports = []corev1.ServicePort{} // Initialize empty slice
	}

	headlessService := &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.Version, Kind: "Service"},
		ObjectMeta: serviceMeta, // Use the pre-built metadata
		Spec: corev1.ServiceSpec{
			Selector:  selectorLabels,              // Use the provided selector labels
			ClusterIP: corev1.ClusterIPNone,        // REQUIRED for headless service
			Type:      corev1.ServiceTypeClusterIP, // Type MUST be ClusterIP for headless

			Ports: ports, // Assign the list of Service Ports

			// Optional: PublishNotReadyAddresses can be configured here if needed
			// PublishNotReadyAddresses: publishNotReadyAddresses,
		},
	}
	return headlessService // Return the built headless service object pointer
}
