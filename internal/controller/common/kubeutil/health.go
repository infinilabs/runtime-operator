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

// internal/controller/common/kubeutil/health.go
package kubeutil

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CheckHealth checks the K8s readiness/health status of a resource based on its Kind.
// Returns: isHealthy (bool), a message describing status, and an error if check process failed.
func CheckHealth(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme, ns, name, apiVersion, kind string) (bool, string, error) {
	logger := log.FromContext(ctx).WithValues("kind", kind, "apiVersion", apiVersion, "name", name, "namespace", ns)
	logger.V(1).Info("Checking resource health status")

	gvk := schema.FromAPIVersionAndKind(apiVersion, kind)
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	key := client.ObjectKey{Namespace: ns, Name: name}

	if err := k8sClient.Get(ctx, key, obj); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Resource not found during health check")
			return false, "Resource not found", nil
		}
		logger.Error(err, "Failed to get resource during health check process")
		return false, fmt.Sprintf("Failed to get resource: %v", err), fmt.Errorf("failed to get resource %s %s/%s: %w", kind, ns, name, err)
	}

	// Convert unstructured to typed object using the provided scheme
	// Create an empty object of the correct type based on GVK
	typedObj, err := scheme.New(gvk)
	if err != nil {
		msg := fmt.Sprintf("Scheme failed to create new object for GVK %s", gvk.String())
		logger.Error(err, msg)
		return false, msg, fmt.Errorf("%s: %w", msg, err)
	}

	// Convert the unstructured data into the typed object
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, typedObj); err != nil {
		msg := fmt.Sprintf("Failed to convert unstructured to typed object for GVK %s", gvk.String())
		logger.Error(err, msg)
		return false, fmt.Sprintf("%s: %v", msg, err), fmt.Errorf("%s %s/%s: %w", msg, ns, name, err)
	}

	// --- Check health based on the *typed* object ---
	switch resource := typedObj.(type) {
	case *appsv1.Deployment:
		return checkDeploymentHealth(resource)
	case *appsv1.StatefulSet:
		return checkStatefulSetHealth(resource)
	case *corev1.Service:
		return checkServiceHealth(ctx, k8sClient, resource) // Pass typed object
	case *corev1.PersistentVolumeClaim:
		return checkPVCHealth(resource)
	case *policyv1.PodDisruptionBudget:
		return checkPdbHealth(resource)
	case *policyv1beta1.PodDisruptionBudget:
		// If using policy/v1beta1, handle it similarly to policy/v1
		return checkPdbHealthBeta1(resource)
	case *corev1.ConfigMap, *corev1.Secret, *corev1.ServiceAccount:
		// These types are generally considered healthy if they exist.
		return true, fmt.Sprintf("%s exists", kind), nil
	// Add cases for other types (DaemonSet, Job, Ingress etc.)
	default:
		logger.V(1).Info("No specific health check implemented for this GVK, assuming exists implies healthy.", "GVK", gvk.String())
		return true, fmt.Sprintf("Exists, specific health check for %s not implemented", gvk.Kind), nil
	}
}

// --- Simple type-specific checkers (Internal to health.go) ---

func checkDeploymentHealth(deployment *appsv1.Deployment) (bool, string, error) {
	// Check if the deployment is paused
	if deployment.Spec.Paused {
		return false, "Deployment is paused", nil
	}

	// Check observedGeneration vs Generation
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return false, fmt.Sprintf("Waiting for rollout to be observed (generation %d < desired %d)",
			deployment.Status.ObservedGeneration, deployment.Generation), nil
	}

	// Check standard conditions
	var availableCond, progressingCond *appsv1.DeploymentCondition
	for i := range deployment.Status.Conditions {
		cond := deployment.Status.Conditions[i]
		switch cond.Type {
		case appsv1.DeploymentAvailable:
			availableCond = &cond
		case appsv1.DeploymentProgressing:
			progressingCond = &cond
		}
	}

	if availableCond == nil || availableCond.Status != corev1.ConditionTrue {
		msg := "Deployment not available"
		if availableCond != nil {
			msg = fmt.Sprintf("%s: %s (%s)", msg, availableCond.Reason, availableCond.Message)
		}
		return false, msg, nil
	}

	// Progressing condition should be True with Reason=NewReplicaSetAvailable for a stable state
	if progressingCond == nil || progressingCond.Status != corev1.ConditionTrue || progressingCond.Reason != "NewReplicaSetAvailable" {
		reason := "Unknown"
		if progressingCond != nil {
			reason = progressingCond.Reason
		}
		return false, fmt.Sprintf("Deployment rollout not complete (Progressing reason: %s)", reason), nil
	}

	// Final check comparing replica counts (should match desired if conditions are met)
	desiredReplicas := int32(1)
	if deployment.Spec.Replicas != nil {
		desiredReplicas = *deployment.Spec.Replicas
	}
	if deployment.Status.UpdatedReplicas < desiredReplicas {
		return false, fmt.Sprintf("Waiting for update: %d/%d updated replicas", deployment.Status.UpdatedReplicas, desiredReplicas), nil
	}
	if deployment.Status.ReadyReplicas < desiredReplicas {
		return false, fmt.Sprintf("Waiting for readiness: %d/%d ready replicas", deployment.Status.ReadyReplicas, desiredReplicas), nil
	}
	if deployment.Status.AvailableReplicas < desiredReplicas {
		return false, fmt.Sprintf("Waiting for availability: %d/%d available replicas", deployment.Status.AvailableReplicas, desiredReplicas), nil
	}

	return true, fmt.Sprintf("Deployment available (%d/%d replicas ready)", deployment.Status.ReadyReplicas, desiredReplicas), nil
}

func checkStatefulSetHealth(sts *appsv1.StatefulSet) (bool, string, error) {
	// Check observedGeneration vs Generation
	if sts.Status.ObservedGeneration < sts.Generation {
		return false, fmt.Sprintf("Waiting for rollout to be observed (generation %d < desired %d)",
			sts.Status.ObservedGeneration, sts.Generation), nil
	}

	desiredReplicas := int32(1)
	if sts.Spec.Replicas != nil {
		desiredReplicas = *sts.Spec.Replicas
	}
	readyReplicas := sts.Status.ReadyReplicas
	currentReplicas := sts.Status.CurrentReplicas // Pods running based on current revision
	updatedReplicas := sts.Status.UpdatedReplicas // Pods running based on update revision

	// Check if rollout is complete (all replicas are on the update revision)
	// Note: sts.Status.CurrentRevision is DEPRECATED, use updatedReplicas.
	if updatedReplicas < desiredReplicas {
		return false, fmt.Sprintf("Rollout in progress: %d/%d pods updated", updatedReplicas, desiredReplicas), nil
	}

	// Check if the number of ready replicas matches the desired number.
	if readyReplicas < desiredReplicas {
		return false, fmt.Sprintf("Waiting for readiness: %d/%d ready replicas", readyReplicas, desiredReplicas), nil
	}

	// Check if the number of current replicas matches the desired number (implies updated replicas are stable)
	if currentReplicas < desiredReplicas {
		return false, fmt.Sprintf("Waiting for stability: %d/%d current replicas", currentReplicas, desiredReplicas), nil
	}

	// Check partition status if using RollingUpdate with Partition
	if sts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType && sts.Spec.UpdateStrategy.RollingUpdate != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		partition := *sts.Spec.UpdateStrategy.RollingUpdate.Partition
		if updatedReplicas < (desiredReplicas - partition) {
			return false, fmt.Sprintf("Waiting for partitioned rollout: %d/%d updated replicas above partition %d", updatedReplicas, desiredReplicas, partition), nil
		}
	}

	return true, fmt.Sprintf("StatefulSet available (%d/%d replicas ready)", readyReplicas, desiredReplicas), nil
}

func checkServiceHealth(ctx context.Context, k8sClient client.Client, svc *corev1.Service) (bool, string, error) {
	logger := log.FromContext(ctx).WithValues("service", svc.Name, "namespace", svc.Namespace)

	if svc.Spec.ClusterIP == corev1.ClusterIPNone {
		// Headless services don't have health in the same way. Check if Endpoints exist.
		endpoints := &corev1.Endpoints{}
		key := client.ObjectKey{Namespace: svc.Namespace, Name: svc.Name}
		if err := k8sClient.Get(ctx, key, endpoints); err != nil {
			if errors.IsNotFound(err) {
				return false, "Headless service endpoints not found", nil
			}
			return false, fmt.Sprintf("Failed to get Headless service endpoints: %v", err), err
		}
		hasAnyEndpoints := false
		if endpoints.Subsets != nil {
			for _, subset := range endpoints.Subsets {
				if len(subset.Addresses) > 0 || len(subset.NotReadyAddresses) > 0 {
					hasAnyEndpoints = true
					break
				}
			}
		}
		if !hasAnyEndpoints {
			return false, "Headless service exists but no endpoints found", nil
		}
		return true, "Headless service exists and endpoints found", nil
	}

	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			// Check if IP or Hostname is assigned
			hasIngress := false
			for _, ingress := range svc.Status.LoadBalancer.Ingress {
				if ingress.IP != "" || ingress.Hostname != "" {
					hasIngress = true
					break
				}
			}
			if hasIngress {
				return true, "LoadBalancer has assigned ingress point(s)", nil
			}
		}
		return false, "Waiting for LoadBalancer ingress assignment", nil
	}

	if svc.Spec.Type == corev1.ServiceTypeExternalName {
		return true, "ExternalName Service exists", nil // Existence is health for ExternalName
	}

	// For ClusterIP and NodePort, check if there are ready endpoints.
	endpoints := &corev1.Endpoints{}
	key := client.ObjectKey{Namespace: svc.Namespace, Name: svc.Name}
	if err := k8sClient.Get(ctx, key, endpoints); err != nil {
		if errors.IsNotFound(err) {
			return false, "Service endpoints not found", nil
		}
		logger.Error(err, "Failed to get Endpoints for Service health check process")
		return false, fmt.Sprintf("Failed to get Endpoints: %v", err), err
	}
	hasReadyEndpoints := false
	readyCount := 0
	totalCount := 0
	if endpoints.Subsets != nil {
		for _, subset := range endpoints.Subsets {
			if subset.Addresses != nil {
				readyCount += len(subset.Addresses)
				hasReadyEndpoints = hasReadyEndpoints || len(subset.Addresses) > 0
			}
			if subset.NotReadyAddresses != nil {
				totalCount += len(subset.NotReadyAddresses)
			}
		}
		totalCount += readyCount
	}
	if !hasReadyEndpoints {
		return false, fmt.Sprintf("No ready endpoints found for Service (%d/%d ready/total)", readyCount, totalCount), nil
	}
	return true, fmt.Sprintf("Service has ready endpoints (%d/%d ready/total)", readyCount, totalCount), nil
}

func checkPVCHealth(pvc *corev1.PersistentVolumeClaim) (bool, string, error) { // Renamed function
	if pvc.Status.Phase == corev1.ClaimBound {
		return true, "PVC is Bound", nil
	}
	if pvc.Status.Phase == corev1.ClaimPending {
		return false, fmt.Sprintf("PVC is Pending (waiting for volume provision). Phase: %s", pvc.Status.Phase), nil
	}
	if pvc.Status.Phase == corev1.ClaimLost {
		return false, fmt.Sprintf("PVC is Lost. Phase: %s", pvc.Status.Phase), nil // Treat Lost as unhealthy
	}
	return false, fmt.Sprintf("PVC phase is %s (needs Bound)", pvc.Status.Phase), nil
}

func checkPdbHealth(pdb *policyv1.PodDisruptionBudget) (bool, string, error) {
	if pdb.Status.ObservedGeneration < pdb.Generation {
		return false, fmt.Sprintf("PDB sync in progress (gen %d < desired %d)", pdb.Status.ObservedGeneration, pdb.Generation), nil
	}
	// Check if current healthy pods meet desired healthy count
	if pdb.Status.CurrentHealthy >= pdb.Status.DesiredHealthy {
		// Additional check: Are disruptions allowed based on current state?
		if pdb.Status.DisruptionsAllowed > 0 {
			return true, fmt.Sprintf("PDB allows disruptions (%d allowed, %d/%d healthy)", pdb.Status.DisruptionsAllowed, pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy), nil
		} else {
			// Healthy count met, but disruptions NOT allowed (e.g., due to MaxUnavailable)
			return true, fmt.Sprintf("PDB healthy (%d/%d), but no disruptions allowed", pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy), nil
		}
	}
	// Not enough healthy pods according to PDB spec
	return false, fmt.Sprintf("PDB not healthy (%d/%d healthy, %d disruptions allowed)", pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy, pdb.Status.DisruptionsAllowed), nil
}

func checkPdbHealthBeta1(pdb *policyv1beta1.PodDisruptionBudget) (bool, string, error) {
	if pdb.Status.ObservedGeneration < pdb.Generation {
		return false, fmt.Sprintf("PDB sync in progress (gen %d < desired %d)", pdb.Status.ObservedGeneration, pdb.Generation), nil
	}
	// Check if current healthy pods meet desired healthy count
	if pdb.Status.CurrentHealthy >= pdb.Status.DesiredHealthy {
		// Additional check: Are disruptions allowed based on current state?
		if pdb.Status.DisruptionsAllowed > 0 {
			return true, fmt.Sprintf("PDB allows disruptions (%d allowed, %d/%d healthy)", pdb.Status.DisruptionsAllowed, pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy), nil
		} else {
			// Healthy count met, but disruptions NOT allowed (e.g., due to MaxUnavailable)
			return true, fmt.Sprintf("PDB healthy (%d/%d), but no disruptions allowed", pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy), nil
		}
	}
	// Not enough healthy pods according to PDB spec
	return false, fmt.Sprintf("PDB not healthy (%d/%d healthy, %d disruptions allowed)", pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy, pdb.Status.DisruptionsAllowed), nil
}
