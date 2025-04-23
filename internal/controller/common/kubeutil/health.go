// internal/controller/common/kubeutil/health.go
package kubeutil

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1" // Needed if checking PDB status

	"k8s.io/apimachinery/pkg/api/errors"                // For IsNotFound
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured" // To get objects dynamically
	"k8s.io/apimachinery/pkg/runtime"                   // Needed for scheme and unstructured conversion
	"k8s.io/apimachinery/pkg/runtime/schema"            // For GVK

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	commonutil "github.com/infinilabs/operator/pkg/apis/common/util"
)

// CheckHealth checks the K8s readiness status of a resource based on its Kind.
// It retrieves the object using the provided client and scheme, then checks its status fields.
// Returns: isHealthy (bool), a message describing status, and an error if check process failed (e.g., cannot get resource, conversion error).
// If isHealthy is false, the message should describe the reason (e.g., "0/3 replicas ready").
func CheckHealth(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme, ns, name, apiVersion, kind string) (bool, string, error) {
	logger := log.FromContext(ctx).WithValues("kind", kind, "apiVersion", apiVersion, "name", name, "namespace", ns)
	logger.V(1).Info("Checking resource health status")

	// 1. Get the resource dynamically using Unstructured
	gvk := schema.FromAPIVersionAndKind(apiVersion, kind)
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk) // Set GVK for client.Get

	key := client.ObjectKey{Namespace: ns, Name: name}

	if err := k8sClient.Get(ctx, key, obj); err != nil {
		if errors.IsNotFound(err) {
			// If resource is not found, it's not healthy, but the check process itself succeeded in finding it missing.
			logger.Info("Resource not found during health check")
			return false, "Resource not found", nil
		}
		// For other errors (e.g., API connection, permission denied), this is an error *during the check process*.
		logger.Error(err, "Failed to get resource during health check process")
		return false, fmt.Sprintf("Failed to get resource: %v", err), fmt.Errorf("failed to get resource %s %s/%s: %w", kind, ns, name, err) // Return error in addition to false
	}

	// 2. Convert to specific type and check health using type-specific logic.
	// Use runtime.DefaultUnstructuredConverter. Requires Scheme access.
	// Assume scheme is passed correctly.

	switch gvk.GroupKind().String() { // Use GroupKind().String() for case consistency
	case "Deployment":
		var deployment appsv1.Deployment
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deployment)
		if err != nil {
			msg := "Failed to convert unstructured to Deployment"
			logger.Error(err, msg)
			return false, fmt.Sprintf("%s: %v", msg, err), fmt.Errorf("%s %s/%s: %w", msg, ns, name, err) // Error during conversion process
		}
		return checkDeploymentHealth(&deployment) // Call type-specific simple checker

	case "StatefulSet":
		var sts appsv1.StatefulSet
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &sts)
		if err != nil {
			msg := "Failed to convert unstructured to StatefulSet"
			logger.Error(err, msg)
			return false, fmt.Sprintf("%s: %v", msg, err), fmt.Errorf("%s %s/%s: %w", msg, ns, name, err)
		}
		return checkStatefulSetHealth(&sts) // Call type-specific simple checker

	case "Service":
		var svc corev1.Service
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &svc)
		if err != nil {
			msg := "Failed to convert unstructured to Service"
			logger.Error(err, msg)
			return false, fmt.Sprintf("%s: %v", msg, err), fmt.Errorf("%s %s/%s: %w", msg, ns, name, err)
		}
		// Service health check needs to fetch Endpoints (requires client)
		return checkServiceHealth(ctx, k8sClient, &svc) // Pass client to the checker

	case "PersistentVolumeClaim":
		var pvc corev1.PersistentVolumeClaim
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pvc)
		if err != nil {
			msg := "Failed to convert unstructured to PVC"
			logger.Error(err, msg)
			return false, fmt.Sprintf("%s: %v", msg, err), fmt.Errorf("%s %s/%s: %w", msg, ns, name, err)
		}
		return checkPVCHealth(&pvc) // Call type-specific checker

	case "PodDisruptionBudget":
		var pdb policyv1.PodDisruptionBudget
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pdb)
		if err != nil {
			msg := "Failed to convert unstructured to PDB"
			logger.Error(err, msg)
			return false, fmt.Sprintf("%s: %v", msg, err), fmt.Errorf("%s %s/%s: %w", msg, ns, name, err)
		}
		// Checking PDB is less common for general component readiness, but can be implemented.
		// Often relies on 'Healthy' status or Desired vs Current healthy Pods.
		return false, "PodDisruptionBudget health check not implemented", nil // Placeholder

	// Add cases for other common K8s workloads you might check health for (e.g. Ingress, DaemonSet, Job)
	// networkingv1.Kind("Ingress").GroupKind().String(): return checkIngressHealth(...)
	// appsv1.Kind("DaemonSet").GroupKind().String(): return checkDaemonSetHealth(...)
	// appsv1.Kind("Job").GroupKind().String(): return checkJobHealth(...)

	default:
		// If the GVK is not one of the standard types we explicitly check health for,
		// assume it's healthy if the Get operation itself succeeded and no explicit check exists.
		// Log a warning/info that a specific check is missing.
		logger.V(1).Info("No specific health check implemented for this GVK, assuming exists implies healthy.", "GVK", gvk.String())
		// This might need a specific check: Is it a ConfigMap? Secret? These are usually "healthy" if they exist.
		if gvk.Group == corev1.SchemeGroupVersion.Group { // Check if it's in the core API group
			switch gvk.Kind {
			case "ConfigMap", "Secret", "ServiceAccount":
				return true, fmt.Sprintf("Exists, %s does not require specific health check beyond existence", gvk.Kind), nil
			}
		}

		// Default: Assume not ready if check not implemented and it's not a simple config/identity type.
		// In a robust system, this might require explicit registration of health check logic per GVK.
		return false, fmt.Sprintf("No specific health check implemented for GVK %s, assuming not ready", gvk.String()), nil
	}
}

func checkPVCHealth(persistentVolumeClaim *corev1.PersistentVolumeClaim) (bool, string, error) {
	// Check if PVC is Bound
	if persistentVolumeClaim.Status.Phase == corev1.ClaimBound {
		return true, "PVC is Bound", nil
	}
	// Other phases (Pending, Lost, Bound) - pending usually means waiting, Lost is an error.
	// Checking for 'Pending' with reasonable message might be useful here.
	if persistentVolumeClaim.Status.Phase == corev1.ClaimPending {
		return false, fmt.Sprintf("PVC is Pending (waiting for volume provision). Current phase: %s", persistentVolumeClaim.Status.Phase), nil
	}
	if persistentVolumeClaim.Status.Phase == corev1.ClaimLost {
		return false, fmt.Sprintf("PVC is Lost: %s", persistentVolumeClaim.Status.Phase), nil // Indicate Lost is a problem
	}

	return false, fmt.Sprintf("PVC phase is %s (needs Bound)", persistentVolumeClaim.Status.Phase), nil // All other phases (incl empty) are not healthy/ready
}

// --- Simple type-specific checkers (Internal to health.go) ---
// These functions perform the actual health check logic on a *typed* Kubernetes object.

func checkDeploymentHealth(deployment *appsv1.Deployment) (bool, string, error) {
	// Check observedGeneration vs Generation (optional check for progress). If spec changed, status needs to catch up.
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return false, fmt.Sprintf("Rollout progress observed: generation %d < desired %d", deployment.Status.ObservedGeneration, deployment.Generation), nil
	}
	// Use standard conditions if available (Available, Progressing). Recommended.
	// The Deployment controller sets these conditions.
	available := false
	progressing := false
	for _, cond := range deployment.Status.Conditions {
		if cond.Type == appsv1.DeploymentAvailable && cond.Status == corev1.ConditionTrue {
			available = true
		}
		if cond.Type == appsv1.DeploymentProgressing && cond.Status == corev1.ConditionTrue {
			// Optional: Check if Progressing is reason NewReplicaSetAvailable
			progressing = true
		}
		// Consider checking ReplicaFailure if Deployment has issues
		// if cond.Type == appsv1.DeploymentReplicaFailure && cond.Status == corev1.ConditionTrue { ... log error ... }
	}

	// More precise check using readyReplicas/availableReplicas against desired Replicas.
	desiredReplicas := commonutil.GetInt32ValueOrDefault(deployment.Spec.Replicas, 1)
	ready := deployment.Status.ReadyReplicas >= desiredReplicas               // Pods reporting ready via probes
	availableStatus := deployment.Status.AvailableReplicas >= desiredReplicas // Pods available (ready for at least minReadySeconds)
	updated := deployment.Status.UpdatedReplicas >= desiredReplicas           // Pods on the latest spec

	// Combine checks: requires availability, progression, and all desired pods ready.
	isHealthy := availableStatus && updated && ready // Simplified

	// Provide status message based on progress.
	message := "Checking deployment progress..."
	if deployment.Generation > deployment.Status.ObservedGeneration {
		message = "Waiting for rollout to be observed by Deployment controller."
	} else if !available {
		message = fmt.Sprintf("Deployment not available: %d/%d available replicas.", deployment.Status.AvailableReplicas, desiredReplicas)
	} else if !updated {
		message = fmt.Sprintf("Waiting for rollout: %d/%d updated replicas.", deployment.Status.UpdatedReplicas, desiredReplicas)
	} else if !ready {
		message = fmt.Sprintf("Waiting for readiness: %d/%d ready replicas.", deployment.Status.ReadyReplicas, desiredReplicas)
	} else if available && updated && ready { // All conditions met based on replica counts
		message = fmt.Sprintf("Deployment is ready (%d/%d replicas ready)", deployment.Status.ReadyReplicas, desiredReplicas)
		isHealthy = true // Confirm healthy if counts match
	} else if !isHealthy && (available || updated || ready || progressing) {
		// If some checks pass but not all, indicate still processing.
		message = "Deployment progressing but not fully ready."
	}

	// If Pods have issues, ReplicaFailure might be True. Need to incorporate checking conditions more robustly.

	return isHealthy, message, nil // Return status based on state, no check *process* error
}

func checkStatefulSetHealth(sts *appsv1.StatefulSet) (bool, string, error) {
	// Check observedGeneration vs Generation (optional check for progress)
	if sts.Status.ObservedGeneration < sts.Generation {
		return false, fmt.Sprintf("Rollout progress observed: generation %d < desired %d", sts.Status.ObservedGeneration, sts.Generation), nil
	}

	desiredReplicas := commonutil.GetInt32ValueOrDefault(sts.Spec.Replicas, 1)
	readyReplicas := sts.Status.ReadyReplicas

	// Check if rollout is complete (current and update revisions match OR desired=0)
	rolloutComplete := (sts.Status.CurrentRevision == sts.Status.UpdateRevision) || desiredReplicas == 0

	// Check if replicas are matching desired (or are being scaled down gracefully)
	// Pod Management Policy OrderedReady -> requires sequential readiness.
	replicasMatchingDesired := sts.Status.Replicas == desiredReplicas

	// Final Readiness: requires replicas to be Ready AND rollout to be complete AND replicas matching desired
	isHealthy := (readyReplicas >= desiredReplicas) && rolloutComplete && replicasMatchingDesired

	message := "Checking StatefulSet progress..."

	if sts.Generation > sts.Status.ObservedGeneration {
		message = "Waiting for rollout to be observed by StatefulSet controller."
	} else if !rolloutComplete {
		message = fmt.Sprintf("Rollout in progress: current revision %s != update revision %s", sts.Status.CurrentRevision, sts.Status.UpdateRevision)
	} else if !replicasMatchingDesired {
		// This is complex. If scaling up, replicasMatch = false is expected during process.
		// If scaling down, Replicas count decreases but might not match until done.
		// A better check might use specific StatefulSet conditions (if they exist).
		// For now, if rollout complete but replica count != desired, check ready count.
		if readyReplicas < desiredReplicas {
			message = fmt.Sprintf("Waiting for readiness: %d/%d ready replicas.", readyReplicas, desiredReplicas)
		} else { // Replicas mismatch but readyCount matches desired - likely scaling down or something stuck
			message = fmt.Sprintf("StatefulSet ready (%d/%d replicas ready) but total replicas mismatch desired (%d)", readyReplicas, desiredReplicas, sts.Status.Replicas)
		}

	} else if readyReplicas < desiredReplicas { // Rollout complete, replicas match, but not all ready
		message = fmt.Sprintf("Waiting for readiness: %d/%d ready replicas.", readyReplicas, desiredReplicas)
	} else if isHealthy {
		message = fmt.Sprintf("StatefulSet is ready (%d/%d replicas ready)", readyReplicas, desiredReplicas)
	}

	// Check specific StatefulSet Conditions if needed for more detail.

	return isHealthy, message, nil // Return status
}

func checkServiceHealth(ctx context.Context, k8sClient client.Client, svc *corev1.Service) (bool, string, error) {
	logger := log.FromContext(ctx).WithValues("service", svc.Name, "namespace", svc.Namespace)

	// Headless services (ClusterIP=None) are usually considered healthy if they exist and selector finds pods.
	if svc.Spec.ClusterIP == corev1.ClusterIPNone {
		// Optionally check if endpoints exist for a headless service.
		endpoints := &corev1.Endpoints{}
		key := client.ObjectKey{Namespace: svc.Namespace, Name: svc.Name}
		if err := k8sClient.Get(ctx, key, endpoints); err != nil {
			if errors.IsNotFound(err) {
				return false, "Headless service endpoints not found", nil
			} // Endpoints missing
			return false, fmt.Sprintf("Failed to get Headless service endpoints: %v", err), err // Check process error
		}
		// Check if there are any subsets with addresses (ready or not ready)
		hasAnyEndpoints := false
		if endpoints.Subsets != nil {
			for _, subset := range endpoints.Subsets {
				if subset.Addresses != nil && len(subset.Addresses) > 0 {
					hasAnyEndpoints = true
				}
				if subset.NotReadyAddresses != nil && len(subset.NotReadyAddresses) > 0 {
					hasAnyEndpoints = true
				}
			}
		}
		if !hasAnyEndpoints {
			return false, "Headless service exists but no endpoints found", nil
		}
		return true, "Headless service exists and endpoints found", nil // Exists and has *some* endpoints
	}

	// For ClusterIP, NodePort, LoadBalancer, check if the Service has ready endpoints.
	if svc.Spec.Type != corev1.ServiceTypeExternalName { // ExternalName does not have endpoints
		// Check if endpoints exist for this service (matching its selector)
		endpoints := &corev1.Endpoints{}
		key := client.ObjectKey{Namespace: svc.Namespace, Name: svc.Name}
		if err := k8sClient.Get(ctx, key, endpoints); err != nil {
			if errors.IsNotFound(err) {
				return false, "Service endpoints not found", nil // No endpoints yet
			}
			logger.Error(err, "Failed to get Endpoints for Service health check process")
			return false, fmt.Sprintf("Failed to get Endpoints: %v", err), err // Error during check process
		}

		// Check if endpoints has ready addresses in subsets
		hasReadyEndpoints := false
		readyCount := 0
		totalCount := 0
		if endpoints.Subsets != nil {
			for _, subset := range endpoints.Subsets {
				if subset.Addresses != nil {
					readyCount += len(subset.Addresses)
				}
				if subset.NotReadyAddresses != nil {
					totalCount += len(subset.NotReadyAddresses)
				} // Count not-ready too
				if subset.Addresses != nil && len(subset.Addresses) > 0 {
					hasReadyEndpoints = true
				} // Flag if at least one ready endpoint subset exists
			}
			totalCount += readyCount
		}

		if !hasReadyEndpoints {
			return false, fmt.Sprintf("No ready endpoints found for Service (%d/%d ready/total)", readyCount, totalCount), nil // No healthy pods connected via Service
		}
		// All checks passed
		return true, fmt.Sprintf("Service has ready endpoints (%d/%d ready/total)", readyCount, totalCount), nil // Service points to healthy pods
	}

	// For ExternalName type, just existence implies healthy (it points to external name)
	return true, "ExternalName Service exists", nil
}

func checkPVCHelith(pvc *corev1.PersistentVolumeClaim) (bool, string, error) {
	if pvc.Status.Phase == corev1.ClaimBound {
		// Optionally check access modes/volume mode against spec if needed.
		// Optionally check allocated resources if requested resources significantly differ.
		return true, "PVC is Bound", nil
	}
	// Other phases (Pending, Lost, Bound) - pending usually means waiting, Lost is an error.
	// Checking for 'Pending' with reasonable message might be useful here.
	if pvc.Status.Phase == corev1.ClaimPending {
		// PVC is Pending, provide a message about potential reason.
		message := fmt.Sprintf("PVC is Pending (waiting for volume provision). Current phase: %s", pvc.Status.Phase)
		if pvc.Status.Capacity != nil && !pvc.Status.Capacity.Storage().IsZero() {
			// If Capacity is set, maybe it's Bound but some post-bind step is pending? (Rare)
			// Rely on standard phase logic.
		}
		// Optionally check events related to the PVC if more debug needed (Controller level)
		// Requeue is needed here.
		return false, message, nil
	}
	if pvc.Status.Phase == corev1.ClaimLost {
		return false, fmt.Sprintf("PVC is Lost: %s", pvc.Status.Phase), nil // Indicate Lost is a problem
	}

	return false, fmt.Sprintf("PVC phase is %s (needs Bound)", pvc.Status.Phase), nil // All other phases (incl empty) are not healthy/ready
}

// Add simple checkers for other types (PDB, Ingress etc) as needed based on their status fields.
/*
func checkPdbHealth(pdb *policyv1.PodDisruptionBudget) (bool, string, error) {
	// Check if PDB is "Healthy" (desired-healthy >= disruption-allowed)
	// PDB status includes ObservedGeneration, DisruptedPods, Current, Desired, Expected disruptions.
	// Check if ObservedGeneration is >= Generation (reflects latest spec).
	// Check if Status.Current >= Status.DesiredHealthy or Status.Current >= Status.DisruptionsAllowed
	// Check relevant PDB Conditions if available (PodDisruptionBudgetAllowed).
    if pdb.Status.ObservedGeneration < pdb.Generation { return false, fmt.Sprintf("PDB sync in progress (gen %d < desired %d)", pdb.Status.ObservedGeneration, pdb.Generation), nil }
    if pdb.Status.CurrentHealthy >= pdb.Status.DesiredHealthy {
         // Check against expected disruptions? Needs complexity.
          return true, fmt.Sprintf("PDB healthy (%d/%d healthy)", pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy), nil
    }
    return false, fmt.Sprintf("PDB not healthy (%d/%d healthy)", pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy), nil // Or provide more detail from status.DisruptedPods/conditions
}

func checkIngressHealth(ingress *networkingv1.Ingress) (bool, string, error) {
    // Check if ingress.status.loadBalancer.ingress has addresses/hostnames assigned.
    // If ingress is managed by external controller (like nginx ingress), check its status.
    if ingress.Status.LoadBalancer.Ingress == nil || len(ingress.Status.LoadBalancer.Ingress) == 0 {
        return false, "Ingress LoadBalancer status is empty", nil
    }
     // Optionally check specific IPs/hostnames or conditions if any
     return true, "Ingress LoadBalancer status assigned", nil
}

func checkDaemonSetHealth(ds *appsv1.DaemonSet) (bool, string, error) {
    // Check ObservedGeneration vs Generation
    // Check status.NumberReady == status.DesiredNumberScheduled
    // Check status.UpdatedNumberScheduled == status.DesiredNumberScheduled (for rollout)
    // Similar logic to Deployment/StatefulSet but tracking nodes instead of fixed replica count.
     return false, "DaemonSet health check not implemented", nil
}

func checkJobHealth(job *appsv1.Job) (bool, string, error) {
    // Check status.conditions for Type=Complete or Type=Failed.
     for _, cond := range job.Status.Conditions {
         if cond.Type == appsv1.JobComplete && cond.Status == corev1.ConditionTrue { return true, "Job completed successfully", nil }
         if cond.Type == appsv1.JobFailed && cond.Status == corev1.ConditionTrue { return false, fmt.Sprintf("Job failed: %s", cond.Reason), nil }
     }
    // If no Complete or Failed condition yet, it's still running/progressing/pending.
     return false, "Job is in progress or pending", nil
}
*/
