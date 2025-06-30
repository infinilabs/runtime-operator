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

// pkg/builders/k8s/configmap.go
package k8s

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildConfigMap builds a corev1.ConfigMap resource.
// It takes the desired name, namespace, labels, annotations, and the data map.
func BuildConfigMap(
	cmMeta metav1.ObjectMeta, // Pre-built metadata (name, namespace, labels, annotations)
	data map[string]string, // Config data (filename -> content)
	binaryData map[string][]byte, // Optional binary data
) *corev1.ConfigMap {

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.Version,
			Kind:       "ConfigMap",
		},
		ObjectMeta: cmMeta,     // Use pre-built metadata
		Data:       data,       // Assign string data
		BinaryData: binaryData, // Assign binary data (optional)
	}
	return cm
}

// BuildConfigMapsFromAppData builds ConfigMap objects from AppConfigData map[string]string.
// Creates ONE ConfigMap named based on resourceName.
// If specific files need to be Secrets, the app-specific builder should filter them out
// before calling this function.
func BuildConfigMapsFromAppData(appConfigData map[string]string, resourceName string, namespace string, labels map[string]string) ([]client.Object, error) { // Return client.Object slice
	if len(appConfigData) == 0 {
		return []client.Object{}, nil // Nothing to build
	}

	// Build metadata for the single ConfigMap
	cmMeta := BuildObjectMeta(resourceName, namespace, labels, nil) // Use common helper, no annotations for now

	// Build the ConfigMap object
	cm := BuildConfigMap(cmMeta, appConfigData, nil) // Pass nil for binaryData

	// Return a slice containing the built ConfigMap
	return []client.Object{cm}, nil
}

func HashConfigMap(cm *corev1.ConfigMap) (string, error) {
	// Only hash the data field (not metadata)
	data, err := json.Marshal(cm.Data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}