// pkg/reconcilers/common/apply_task.go
package common

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	// Import needed kubeutil for apply
	"github.com/infinilabs/operator/internal/controller/common/kubeutil"
)

// Ensure our task implementation complies with the interface
var _ Task = &ApplyResourcesTask{}

// ApplyResourcesTask is a generic task to ensure a list of resources are applied using SSA.
// It expects a list of desired K8s objects to be available in the TaskContext.DesiredObjects.
type ApplyResourcesTask struct {
	// Task doesn't hold its own state or objects. It operates on context.
	// Any specific config for this task instance would be parameters if task definition had them.
}

// NewApplyResourcesTask creates a new instance of ApplyResourcesTask.
// Callers in Strategy define *which* Task implementations to run.
func NewApplyResourcesTask() *ApplyResourcesTask {
	return &ApplyResourcesTask{}
}

// Execute implements Task interface for ApplyResourcesTask.
// It applies all objects found in taskContext.DesiredObjects using SSA.
// It stores results in taskContext.ApplyResults.
// Returns TaskResultComplete if all apply calls were attempted and none returned a critical error.
// Returns TaskResultFailed if any apply call returned a critical error.
// Does NOT return Pending (waiting for Apply is not a common scenario).
func (t *ApplyResourcesTask) Execute(ctx context.Context, cli client.Client, scheme *runtime.Scheme, owner client.Object, taskContext *TaskContext) (TaskResult, error) {
	logger := taskContext.Logger // Use task-specific logger
	appDef := taskContext.AppDef // Get AppDef owner context

	// --- Prerequisites ---
	// Expect TaskContext to have the list of desired K8s objects to apply.
	// This list should be populated by the builder stage *before* this task is executed.
	objectsToApply := taskContext.DesiredObjects // Use objects from context
	if objectsToApply == nil || len(objectsToApply) == 0 {
		logger.V(1).Info("ApplyResourcesTask received no objects to apply, marking complete.")
		// If it's the first task for this component and no objects were built,
		// might indicate a build error happened and object list is empty.
		// Need to coordinate this outcome with the caller (TaskRunner/Controller).
		// Returning Complete signals success of *this task*. If the reason is empty object list,
		// and this was unexpected, a prior task/stage should have returned an error.
		return TaskResultComplete, nil // Nothing to apply
	}

	var firstApplyCallError error // Tracks the first actual API apply call error encountered by this task

	// --- Execute Application of Objects ---
	// Initialize or clear apply results map for objects handled by this task run
	// We can either store results per object or just accumulate state (success/failure).
	// Let's update the apply results map passed in TaskContext.
	taskContext.ApplyResults = make(map[string]kubeutil.ApplyResult) // Clear/initialize results map

	logger.Info("Applying resources", "count", len(objectsToApply))

	for _, obj := range objectsToApply {
		// Ensure metadata like Name and Namespace are set (should be set by builders)
		if obj.GetName() == "" || obj.GetNamespace() == "" {
			err := fmt.Errorf("object type %s missing name or namespace, cannot apply", obj.GetObjectKind().GroupVersionKind().String())
			logger.Error(err, "Skipping apply for object")
			// Record this object failure even if apply call wasn't made.
			// Add to apply results using a fabricated error result.
			gvk := obj.GetObjectKind().GroupVersionKind()
			objKey := client.ObjectKeyFromObject(obj)
			resultMapKey := gvk.String() + "/" + objKey.String()
			taskContext.ApplyResults[resultMapKey] = kubeutil.ApplyResult{Error: err}
			if firstTaskErr == nil {
				firstTaskErr = err
			} // Track error for this task
			continue // Skip this object, but process others
		}

		// Create a unique string key for the applyResults map for *this* object instance.
		gvk := obj.GetObjectKind().GroupVersionKind()
		objKey := client.ObjectKeyFromObject(obj)
		resultMapKey := gvk.String() + "/" + objKey.String()

		// Set Owner Reference - CRITICAL step BEFORE applying
		// Uses the owner object passed in TaskContext (likely AppDef).
		// Use the K8s scheme for OwnerReference logic.
		if err := controllerutil.SetOwnerReference(owner, obj, scheme); err != nil {
			errMsg := fmt.Sprintf("Failed to set OwnerReference on %s %s/%s: %v", gvk.Kind, obj.GetNamespace(), obj.GetName(), err)
			logger.Error(err, errMsg)
			// Record this failure, but continue applying others if possible.
			taskContext.ApplyResults[resultMapKey] = kubeutil.ApplyResult{Error: err} // Store error result
			if firstTaskErr == nil {
				firstTaskErr = err
			} // Track the first apply *process* error
			continue // Skip applying this object
		}

		// Call kubeutil.ApplyObject (implements SSA)
		applyResult := kubeutil.ApplyObject(ctx, cli, obj, "operator-name") // Need field manager, use constant
		taskContext.ApplyResults[resultMapKey] = applyResult                // Store the result using unique key

		if applyResult.Error != nil {
			// Record apply failure but continue to attempt applying other resources.
			errMsg := fmt.Sprintf("Failed to apply resource %s %s/%s: %v", gvk.Kind, obj.GetNamespace(), obj.GetName(), applyResult.Error)
			logger.Error(applyResult.Error, errMsg)
			if firstTaskErr == nil {
				firstTaskErr = applyResult.Error
			} // Track the first error for THIS task execution
		} else {
			logger.V(1).Info("Successfully applied resource", "kind", gvk.Kind, "name", obj.GetNamespace()+"/"+obj.GetName(), "operation", applyResult.Operation)
			// If needed by other tasks, you might store the applied object itself here or reference it from DesiredObjects
			// taskContext.AppliedObjects[resultMapKey] = obj.DeepCopyObject().(client.Object) // Store a copy
		}
	} // End apply loop

	// Determine Task Result
	if firstTaskErr != nil {
		// If any apply call returned an error, the task fails.
		logger.Error(firstTaskErr, "ApplyResourcesTask finished with errors")
		// The Task Runner should receive TaskResultFailed and the first error encountered by this task.
		return TaskResultFailed, firstTaskErr // Return first error encountered during applies
	}

	// If the loop completed without encountering any error from kubeutil.ApplyObject, the task is complete.
	// Resources are *attempted* to be applied. Health check is a separate task.
	logger.V(1).Info("ApplyResourcesTask completed successfully")
	return TaskResultComplete, nil // All apply calls succeeded without error.
}
