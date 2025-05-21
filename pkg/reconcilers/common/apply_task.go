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

// pkg/reconcilers/common/apply_task.go
package common

import (
	"context"
	"fmt"

	"github.com/infinilabs/runtime-operator/internal/controller/common/kubeutil"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Ensure our task implementation complies with the Task interface
var _ Task = &ApplyResourcesTask{}

// ApplyResourcesTask is a generic task to ensure a list of resources are applied using SSA.
// It reads desired objects from TaskContext.DesiredObjects and updates TaskContext.ApplyResults.
type ApplyResourcesTask struct {
	// OperatorName is used as the FieldManager for Server-Side Apply.
	FieldManager string
}

// GetName returns the unique name for this task type.
func (t *ApplyResourcesTask) GetName() string {
	return "ApplyResourcesTask"
}

// Execute implements Task interface for ApplyResourcesTask.
// *** UPDATED Signature and uses TaskContext fields ***
func (t *ApplyResourcesTask) Execute(ctx context.Context, taskContext *TaskContext) (TaskResult, error) {
	// --- Get resources from TaskContext ---
	logger := taskContext.Logger
	cli := taskContext.Client        // Get client from context
	scheme := taskContext.Scheme     // Get scheme from context
	owner := taskContext.Owner       // Get owner from context
	recorder := taskContext.Recorder // Get recorder from context
	// Get desired objects map from context (Key: GVKString/NsName)
	objectsToApplyMap := taskContext.DesiredObjects

	if len(objectsToApplyMap) == 0 {
		logger.V(1).Info("ApplyResourcesTask: No objects found in context to apply.")
		return TaskResultComplete, nil // Nothing to apply
	}

	var firstTaskErr error // Tracks the first critical error encountered in *this task*

	// Ensure ApplyResults map exists in context or initialize it
	if taskContext.ApplyResults == nil {
		taskContext.ApplyResults = make(map[string]kubeutil.ApplyResult)
	}

	logger.Info("Applying resources", "count", len(objectsToApplyMap))

	// Iterate through the map of desired objects to apply
	for resultMapKey, obj := range objectsToApplyMap {
		// Basic validation on the object from the map
		if obj == nil || obj.GetName() == "" || obj.GetNamespace() == "" {
			err := fmt.Errorf("skipping apply for object with key '%s' due to missing name or namespace", resultMapKey)
			logger.Error(err, "Invalid object found in desired state map")
			taskContext.ApplyResults[resultMapKey] = kubeutil.ApplyResult{Error: err}
			if firstTaskErr == nil {
				firstTaskErr = err
			}
			continue // Skip this invalid object
		}

		// Set Owner Reference - CRITICAL step BEFORE applying
		if err := controllerutil.SetControllerReference(owner, obj, scheme); err != nil { // Use owner/scheme from context
			errMsg := fmt.Sprintf("Failed to set OwnerReference on %s %s/%s: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetNamespace(), obj.GetName(), err)
			logger.Error(err, errMsg)
			if recorder != nil {
				recorder.Eventf(owner.(runtime.Object), "Warning", "SetOwnerRefFailed", errMsg)
			}
			if firstTaskErr == nil {
				firstTaskErr = err
			}
			taskContext.ApplyResults[resultMapKey] = kubeutil.ApplyResult{Error: err}
			// Treat OwnerRef failure as critical for this task execution? Yes.
			return TaskResultFailed, firstTaskErr // Stop task on critical error
		}

		// Call kubeutil.ApplyObject (implements SSA)
		applyResult := kubeutil.ApplyObject(ctx, cli, obj, t.FieldManager) // Use cli from context and FieldManager from task
		taskContext.ApplyResults[resultMapKey] = applyResult               // Store the result using unique key

		if applyResult.Error != nil {
			// Record apply failure but continue to attempt applying other resources unless it's a fatal error type?
			// For Apply task, let's consider any apply error as something to report, but maybe not stop the whole task run immediately.
			errMsg := fmt.Sprintf("Failed to apply resource %s %s/%s: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetNamespace(), obj.GetName(), applyResult.Error)
			logger.Error(applyResult.Error, errMsg)
			if recorder != nil {
				recorder.Eventf(owner.(runtime.Object), "Warning", "ResourceApplyFailed", errMsg)
			}
			if firstTaskErr == nil {
				firstTaskErr = applyResult.Error
			} // Track the first apply error encountered
			// Continue applying other objects even if one fails
		} else {
			logger.V(1).Info("Successfully applied resource", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetNamespace()+"/"+obj.GetName(), "operation", applyResult.Operation)
		}
	} // End apply loop

	// Determine Task Result based on if any *critical* errors occurred (currently only OwnerRef failure)
	// or if any apply call returned an error.
	if firstTaskErr != nil {
		// If any apply call returned an error, the task overall encountered issues.
		// Return Failed to signal this to the Task Runner.
		logger.Error(firstTaskErr, "ApplyResourcesTask finished with errors")
		// ComponentStatus message should be updated by the main controller using the applyResults map.
		return TaskResultFailed, firstTaskErr
	}

	// If the loop completed without any critical error.
	logger.V(1).Info("ApplyResourcesTask completed all apply calls successfully (individual errors may exist in results).")
	// Even if individual applies failed (non-critical), the *task* of attempting all applies completed.
	// The overall success/failure/pending state depends on subsequent health checks.
	return TaskResultComplete, nil
}
