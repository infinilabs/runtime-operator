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

// pkg/builders/k8s/helpers.go
package k8s

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/infinilabs/runtime-operator/pkg/apis/common"
)

// --- Naming and Labeling Helpers ---

// BuildCommonLabels creates a map of standard labels for Kubernetes resources.
func BuildCommonLabels(appName string, compType string, instanceName string) map[string]string {
	return map[string]string{
		common.ManagedByLabel:        common.OperatorName,
		"app.kubernetes.io/name":     compType,
		"app.kubernetes.io/instance": instanceName,
		common.AppNameLabel:          appName,
		common.CompNameLabel:         compType,
		common.CompInstanceLabel:     instanceName,
	}
}

// BuildSelectorLabels creates labels used for workload selectors (Deployment, StatefulSet).
func BuildSelectorLabels(instanceName string, compType string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     compType,
		"app.kubernetes.io/instance": instanceName,
	}
}

// DeriveResourceName generates a DNS-safe name for Kubernetes resources.
func DeriveResourceName(instanceName string) string {
	name := strings.ToLower(instanceName)
	// Basic sanitization - replace underscores, ensure max length
	name = strings.ReplaceAll(name, "_", "-")
	if len(name) > 63 {
		name = name[:63]
	}
	// Ensure it doesn't end with a hyphen
	name = strings.TrimRight(name, "-")
	// TODO: Add more robust DNS label validation/sanitization if needed
	return name
}

// DeriveContainerName generates a DNS-safe name for the primary container.
func DeriveContainerName(compType string) string {
	name := strings.ToLower(strings.ReplaceAll(compType, " ", "-"))
	name = strings.ReplaceAll(name, "_", "-") // Also replace underscores
	if len(name) > 63 {
		name = name[:63]
	}
	// Ensure start/end alphanumeric (basic check)
	if len(name) > 0 {
		firstChar := name[0]
		lastChar := name[len(name)-1]
		if !((firstChar >= 'a' && firstChar <= 'z') || (firstChar >= '0' && firstChar <= '9')) {
			name = "c-" + name // Prepend if starts invalidly
			if len(name) > 63 {
				name = name[:63]
			} // Re-check length
		}
		// Re-check last char after potential prepend
		lastChar = name[len(name)-1]
		if !((lastChar >= 'a' && lastChar <= 'z') || (lastChar >= '0' && lastChar <= '9')) {
			name = name[:len(name)-1] + "c" // Append if ends invalidly (replace last char)
		}
	} else {
		name = "container" // Default if somehow empty
	}
	return name
}

// BuildObjectMeta builds standard Kubernetes ObjectMeta for a resource.
func BuildObjectMeta(name, namespace string, labels, annotations map[string]string) metav1.ObjectMeta {
	// Ensure labels map is initialized if nil to avoid panics later
	if labels == nil {
		labels = make(map[string]string)
	}
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Labels:      labels,
		Annotations: annotations, // Can be nil
	}
}

// --- K8s Spec Field Helpers ---

// BuildImageName constructs the full container image string.
func BuildImageName(repository, tag string) string {
	if repository == "" {
		return "" // Should likely error upstream if image is required
	}
	if tag != "" {
		return fmt.Sprintf("%s:%s", repository, tag)
	}
	// Decide default tag behavior: either implicitly "latest" or require tag upstream
	return repository // Or return fmt.Sprintf("%s:latest", repository)
}

// GetImagePullPolicy returns the policy string or a default.
func GetImagePullPolicy(policy corev1.PullPolicy, imageTag string) corev1.PullPolicy {
	if policy != "" {
		return policy
	}
	// K8s default behavior: If the tag is :latest, the default policy is Always. Otherwise, IfNotPresent.
	if strings.HasSuffix(imageTag, ":latest") || imageTag == "" { // Handle empty tag as latest? Or rely on BuildImageName default? Let's assume empty means latest for policy.
		return corev1.PullAlways
	}
	return corev1.PullIfNotPresent
}

// BuildK8sResourceRequirements builds corev1.ResourceRequirements from common spec.
func BuildK8sResourceRequirements(spec *common.ResourcesSpec) corev1.ResourceRequirements {
	if spec == nil {
		return corev1.ResourceRequirements{}
	}
	req := corev1.ResourceRequirements{}
	if spec.Limits != nil {
		req.Limits = spec.Limits.DeepCopy()
	}
	if spec.Requests != nil {
		req.Requests = spec.Requests.DeepCopy()
	}
	return req
}

// BuildContainerPorts maps []common.PortSpec to []corev1.ContainerPort.
func BuildContainerPorts(portSpecs []common.PortSpec) []corev1.ContainerPort {
	if portSpecs == nil {
		return []corev1.ContainerPort{}
	}
	k8sPorts := make([]corev1.ContainerPort, 0, len(portSpecs))
	for _, ps := range portSpecs {
		k8sPorts = append(k8sPorts, corev1.ContainerPort{
			Name:          ps.Name,
			ContainerPort: ps.ContainerPort,
			Protocol:      ps.Protocol, // Defaults to TCP if empty
			// HostPort, HostIP not typically set in operators
		})
	}
	return k8sPorts
}

// BuildServicePorts maps []common.PortSpec to []corev1.ServicePort.
func BuildServicePorts(portSpecs []common.PortSpec) []corev1.ServicePort {
	if portSpecs == nil {
		return []corev1.ServicePort{}
	}
	k8sPorts := make([]corev1.ServicePort, 0, len(portSpecs))
	portNames := make(map[string]bool) // Track names to avoid duplicates

	for _, ps := range portSpecs {
		targetPort := intstr.FromInt32(ps.ContainerPort) // Default target to container port
		if ps.TargetPort != nil {
			targetPort = *ps.TargetPort // Override if specified
		}

		protocol := ps.Protocol
		if protocol == "" {
			protocol = corev1.ProtocolTCP // Default protocol
		}

		// Ensure unique name for the service port
		portName := ps.Name
		if portName == "" {
			// Derive a default name if empty
			portName = fmt.Sprintf("port-%d-%s", ps.ContainerPort, strings.ToLower(string(protocol)))
		}
		// Ensure the final name is unique within the service
		finalPortName := portName
		counter := 1
		for portNames[finalPortName] {
			finalPortName = fmt.Sprintf("%s-%d", portName, counter)
			counter++
		}
		portNames[finalPortName] = true

		sp := corev1.ServicePort{
			Name:       finalPortName,
			Port:       ps.ContainerPort, // Service port usually matches container port unless overridden
			TargetPort: targetPort,
			Protocol:   protocol,
			// NodePort: // Only set if Service Type is NodePort and value is provided/required
		}
		k8sPorts = append(k8sPorts, sp)
	}
	return k8sPorts
}

// BuildProbe builds a K8s corev1.Probe struct pointer, applying defaults.
func BuildProbe(probeSpec *corev1.Probe) *corev1.Probe {
	if probeSpec == nil {
		return nil
	}
	probeCopy := probeSpec.DeepCopy()
	// Apply common defaults if not set by user
	if probeCopy.PeriodSeconds == 0 {
		probeCopy.PeriodSeconds = 10 // Default K8s period
	}
	if probeCopy.TimeoutSeconds == 0 {
		probeCopy.TimeoutSeconds = 1 // Default K8s timeout
	}
	if probeCopy.SuccessThreshold == 0 {
		probeCopy.SuccessThreshold = 1 // Default K8s success threshold
	}
	if probeCopy.FailureThreshold == 0 {
		probeCopy.FailureThreshold = 3 // Default K8s failure threshold
	}
	// Apply scheme default for HTTPGet
	if probeCopy.HTTPGet != nil && probeCopy.HTTPGet.Scheme == "" {
		probeCopy.HTTPGet.Scheme = corev1.URISchemeHTTP
	}
	return probeCopy
}

// ++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// +++ ADD THIS FUNCTION ++++++++++++++++++++++++++++++++++
// ++++++++++++++++++++++++++++++++++++++++++++++++++++++++

// DeriveServiceAccountName determines the name of the ServiceAccount to use.
// It respects the Create flag and explicit Name override in the config.
// If Create is false, returns empty string (use K8s default).
// If Create is true (or default) and Name is set, returns the configured name.
// If Create is true (or default) and Name is not set, returns a derived default name.
func DeriveServiceAccountName(instanceName string, config *common.ServiceAccountSpec) string {
	createSA := false // Default to creating the SA unless explicitly disabled
	if config != nil && config.Create != nil {
		createSA = *config.Create
	}

	if !createSA {
		return "" // Use default K8s service account if creation is disabled
	}

	// If creating, check for explicit name override
	if config != nil && config.Name != "" {
		return strings.ToLower(config.Name) // Use provided name (ensure lowercase)
	}

	// Derive default name if creating and no override provided
	return DeriveResourceName(instanceName) + "-sa" // Use helper for base name + suffix
}

// GetStatefulSetUpdateStrategyOrDefault returns the StatefulSet update strategy or a default.
func GetStatefulSetUpdateStrategyOrDefault(strategy *appsv1.StatefulSetUpdateStrategy) appsv1.StatefulSetUpdateStrategy {
	if strategy != nil {
		return *strategy.DeepCopy()
	}
	return appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
		// RollingUpdate: nil, // RollingUpdate default partition is 0
	}
}

// GetStatefulSetPodManagementPolicyOrDefault returns the Pod Management Policy or the default.
func GetStatefulSetPodManagementPolicyOrDefault(policy *appsv1.PodManagementPolicyType) appsv1.PodManagementPolicyType {
	if policy != nil {
		return *policy
	}
	return appsv1.OrderedReadyPodManagement
}

// GetAffinityOrDefault returns the Affinity pointer or nil after deep copy.
func GetAffinityOrDefault(affinity *corev1.Affinity) *corev1.Affinity {
	if affinity == nil {
		return nil
	}
	return affinity.DeepCopy()
}

// GetPodSecurityContextOrDefault returns the Pod Security Context pointer or nil after deep copy.
func GetPodSecurityContextOrDefault(psc *corev1.PodSecurityContext) *corev1.PodSecurityContext {
	if psc == nil {
		return nil
	}
	return psc.DeepCopy()
}

// GetContainerSecurityContextOrDefault returns the Container Security Context pointer or nil after deep copy.
func GetContainerSecurityContextOrDefault(csc *corev1.SecurityContext) *corev1.SecurityContext {
	if csc == nil {
		return nil
	}
	return csc.DeepCopy()
}

// --- Other helpers ---

// MergeMaps merges maps, preferring keys from the 'override' map. Returns a new map.
func MergeMaps(base, override map[string]string) map[string]string {
	if base == nil && override == nil {
		return nil // Or return empty map: make(map[string]string)
	}
	merged := make(map[string]string)
	if base != nil {
		for k, v := range base {
			merged[k] = v
		}
	}
	if override != nil {
		for k, v := range override {
			merged[k] = v // Override keys from base
		}
	}
	return merged
}
