// pkg/reconcilers/common/tasks.go
package common

import (
	"context"
	"fmt"

	// For specific workload types in health checks
	// For Service, PVC, etc. in health checks
	"k8s.io/apimachinery/pkg/runtime" // For Scheme
	// For GVK
	appv1 "github.com/infinilabs/operator/api/app/v1" // App types (for ComponentStatusReference)
	// Common utils
	"github.com/infinilabs/operator/internal/controller/common/kubeutil" // Kubeutil for client ops

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// --- Reconciler Task Framework (Conceptual) ---
// Task Interface definition
type Task interface {
	// Execute performs the task.
	// client: K8s client. scheme: runtime scheme. owner: owner object (AppDef).
	// taskContext: Contains info needed by the task (logger, componentStatus, desiredObjects specific to this task etc.).
	// Returns TaskResult (Complete, Pending, Failed) and an error if the task failed.
	Execute(ctx context.Context, client client.Client, scheme *runtime.Scheme, owner client.Object, taskContext *TaskContext) (TaskResult, error)
}

// TaskResult indicates the outcome of a Task execution.
type TaskResult string

const (
	TaskResultComplete TaskResult = "Complete" // Task finished successfully
	TaskResultPending  TaskResult = "Pending"  // Task is ongoing, needs more time (e.g. waiting for pods)
	TaskResultFailed   TaskResult = "Failed"   // Task encountered a failure
	// Add other states if needed (e.g., Skipped, RollingBack)
)

// TaskContext holds information available during a Task execution.
// It might be populated by the Task Runner (e.g., which component, which objects it needs to handle).
type TaskContext struct {
	Logger          log.Logger                      // Logger for this task
	ComponentStatus *appv1.ComponentStatusReference // Status for the component this task is reconciling (can update message)
	DesiredObjects  []client.Object                 // Subset of desired objects specific to this task's responsibility (e.g. just Deployment)
	AppliedObjects  map[string]client.Object        // Objects that have been successfully applied by this task run
	ApplyResults    map[string]kubeutil.ApplyResult // Results of apply calls made during this task run

	// Add other context info as needed (e.g., AppDef itself, MergedConfig slice for this component)
	AppDef *appv1.ApplicationDefinition // Need AppDef for context/owner reference during nested apply calls

	// Task specific parameters (if Task definition had parameters)
	// Parameters map[string]interface{} // If Task definitions had parameters

}

// --- Generic Reconciler Task Implementations ---

// ApplyResourcesTask is a generic task to ensure a list of resources are applied.
// It takes a list of objects to apply and attempts to apply them using SSA.
type ApplyResourcesTask struct {
	// ObjectsToApply []client.Object // List of K8s objects to apply (Pass via TaskContext.DesiredObjects instead)
	// Needs a reference to Task Runner to apply objects and get results? Or call kubeutil directly.
}

// Execute implements Task interface for ApplyResourcesTask.
// It iterates through the provided DesiredObjects in the TaskContext, applies them using kubeutil,
// updates apply results in TaskContext, and determines the task result.
func (t *ApplyResourcesTask) Execute(ctx context.Context, cli client.Client, scheme *runtime.Scheme, owner client.Object, taskContext *TaskContext) (TaskResult, error) {
	logger := taskContext.Logger // Use task-specific logger
	appDef := taskContext.AppDef // Get AppDef owner

	// Ensure required data exists in context
	if taskContext.DesiredObjects == nil || len(taskContext.DesiredObjects) == 0 {
		logger.V(1).Info("ApplyResourcesTask received no objects to apply, marking complete.")
		return TaskResultComplete, nil // Nothing to apply
	}

	var firstTaskErr error // Error for this task specifically

	// Apply objects using kubeutil.ApplyObject (implements SSA)
	appliedSuccessfully := true
	// Clear previous apply results for objects this task is applying
	taskContext.ApplyResults = make(map[string]kubeutil.ApplyResult) // Start with a fresh map for this task

	for _, obj := range taskContext.DesiredObjects {
		gvk := obj.GetObjectKind().GroupVersionKind()
		objKey := client.ObjectKeyFromObject(obj)
		resultMapKey := gvk.String() + "/" + objKey.String()

		// Ensure Owner Reference is set before applying - CRITICAL
		if err := controllerutil.SetOwnerReference(appDef, obj, scheme); err != nil { // Use appDef from TaskContext or owner? owner is usually appDef
			errMsg := fmt.Sprintf("Failed to set OwnerReference for %s %s/%s: %v", gvk.Kind, obj.GetNamespace(), obj.GetName(), err)
			logger.Error(err, errMsg)
			// Record this failure but continue applying others if possible.
			taskContext.ApplyResults[resultMapKey] = kubeutil.ApplyResult{Error: err}
			if firstTaskErr == nil {
				firstTaskErr = err
			} // Track first error for this task
			appliedSuccessfully = false // Task did not fully succeed
			continue                    // Continue to next object
		}

		// Call kubeutil.ApplyObject
		applyResult := kubeutil.ApplyObject(ctx, cli, obj, "operator-name") // Pass operator name FieldManager
		taskContext.ApplyResults[resultMapKey] = applyResult                // Store result using unique key

		if applyResult.Error != nil {
			errMsg := fmt.Sprintf("Failed to apply resource %s %s/%s: %v", gvk.Kind, obj.GetNamespace(), obj.GetName(), applyResult.Error)
			logger.Error(applyResult.Error, errMsg)
			if firstTaskErr == nil {
				firstTaskErr = applyResult.Error
			} // Track first error for this task
			appliedSuccessfully = false // Task did not fully succeed
		} else {
			logger.V(1).Info("Successfully applied resource", "kind", gvk.Kind, "name", obj.GetNamespace()+"/"+obj.GetName(), "operation", applyResult.Operation)
			// Store the applied object in a list within TaskContext or reconcileState if needed later by other tasks (e.g. for status update).
			// taskContext.AppliedObjects[resultMapKey] = obj // Example
		}
	} // End apply loop

	// Determine Task Result based on apply results
	if firstTaskErr != nil {
		// If any apply call failed, the task is Failed.
		logger.Error(firstTaskErr, "ApplyResourcesTask failed")
		return TaskResultFailed, firstTaskErr
	}

	// If all apply calls succeeded, the task is Complete (from Apply perspective, not necessarily Ready).
	logger.V(1).Info("ApplyResourcesTask completed successfully for all objects")
	return TaskResultComplete, nil
}

// NewApplyResourcesTask creates a new ApplyResourcesTask.
func NewApplyResourcesTask() *ApplyResourcesTask {
	return &ApplyResourcesTask{}
}

// CheckWorkloadReadyTask is a generic task to check K8s readiness of a workload (Deployment/StatefulSet).
// It relies on kubeutil.CheckHealth internally.
type CheckWorkloadReadyTask struct {
	// Needs GVK, Name, Namespace of the workload to check. Pass via TaskContext.
	// It assumes the desired workload object (e.g. Deployment) is provided in the TaskContext.
}

// Execute implements Task interface for CheckWorkloadReadyTask.
// Checks the K8s readiness of the primary workload resource (Deployment or StatefulSet)
// identified in the ComponentStatus or derived from a single DesiredObject.
func (t *CheckWorkloadReadyTask) Execute(ctx context.Context, cli client.Client, scheme *runtime.Scheme, owner client.Object, taskContext *TaskContext) (TaskResult, error) {
	logger := taskContext.Logger

	// Find the primary workload object among DesiredObjects if possible, OR rely on ComponentStatus info.
	// Assuming TaskContext provides a reference to the component status for this task's component.
	compStatus := taskContext.ComponentStatus // Status for THIS component instance

	// Ensure compStatus has workload info populated by the build stage.
	if compStatus == nil || compStatus.ResourceName == "" || compStatus.Kind == "" || compStatus.APIVersion == "" {
		logger.Error(nil, "Component status info incomplete, cannot check workload readiness")
		// Task fails because prerequisite info is missing.
		return TaskResultFailed, fmt.Errorf("prerequisite: component status missing resource info")
	}

	// Perform live health check using kubeutil.CheckHealth.
	isHealthy, message, checkProcessErr := kubeutil.CheckHealth(ctx, cli, compStatus.Namespace, compStatus.ResourceName, compStatus.APIVersion, compStatus.Kind)

	// Update Component Status Message (might be handled by Task Runner aggregating messages)
	compStatus.Message = message // Overwrite message with readiness status

	if checkProcessErr != nil {
		// Error occurred *during the check process* itself (e.g. API server communication)
		logger.Error(checkProcessErr, "Failed to execute K8s workload readiness check process")
		// This indicates a potential transient issue. Mark task as Pending or Failed depending on severity.
		// Returning Pending allows requeue and retry, often suitable for transient errors.
		// If it's a persistent check error (e.g., bad resource name), Maybe Failed.
		return TaskResultPending, checkProcessErr // Return error to controller-runtime for backoff
	}

	// Health check process succeeded. Determine Task Result based on the component's actual readiness.
	if isHealthy {
		logger.V(1).Info("K8s workload readiness check passed", "kind", compStatus.Kind, "name", compStatus.ResourceName)
		return TaskResultComplete, nil // Workload is ready.
	} else {
		// Workload is not ready yet. The message describes why (e.g., 0/1 replicas ready).
		logger.V(1).Info("K8s workload not yet ready", "kind", compStatus.Kind, "name", compStatus.ResourceName, "status", message)
		return TaskResultPending, nil // Task is pending completion.
	}
}

// NewCheckWorkloadReadyTask creates a new CheckWorkloadReadyTask.
func NewCheckWorkloadReadyTask() *CheckWorkloadReadyTask {
	return &CheckWorkloadReadyTask{}
}

// TODO: Add more common tasks:
// - EnsureServiceAccountTask (create SA if needed)
// - EnsureConfigMapTask (create CMs if needed)
// - EnsureSecretTask (create Secrets if needed)
// - ExpandVolumeTask (handle PVC expansion)
// - CleanupOrphanedResourcesTask (garbage collection logic)
