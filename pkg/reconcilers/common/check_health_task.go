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

// pkg/reconcilers/common/check_health_task.go
package common

import (
	"context"
	"fmt"

	// "time"

	// App types

	// Scheme needed in context

	// "sigs.k8s.io/controller-runtime/pkg/log" // Logger from context

	"github.com/infinilabs/runtime-operator/internal/controller/common/kubeutil" // Kubeutil for health check
)

// Ensure our task implementation complies with the Task interface
var _ Task = &CheckK8sHealthTask{}

// CheckK8sHealthTask is a generic task to check K8s readiness/health of the primary workload resource.
type CheckK8sHealthTask struct {
	// No task-specific fields needed. Relies solely on TaskContext.
}

// NewCheckK8sHealthTask creates a new instance of CheckK8sHealthTask.
func NewCheckK8sHealthTask() *CheckK8sHealthTask {
	return &CheckK8sHealthTask{}
}

// GetName returns the unique name for this task type.
func (t *CheckK8sHealthTask) GetName() string {
	return "CheckK8sHealthTask"
}

// Execute implements Task interface for CheckK8sHealthTask.
func (t *CheckK8sHealthTask) Execute(ctx context.Context, taskContext *TaskContext) (TaskResult, error) {
	// --- Get resources from TaskContext ---
	logger := taskContext.Logger
	cli := taskContext.Client    // Get client from context
	scheme := taskContext.Scheme // Get scheme from context
	// owner := taskContext.Owner // Owner might not be needed directly by health check task
	compStatus := taskContext.ComponentStatus

	// --- Prerequisites Check ---
	if compStatus == nil {
		err := fmt.Errorf("internal error: TaskContext is missing ComponentStatus")
		logger.Error(err, "Cannot check health")
		return TaskResultFailed, err
	}
	if compStatus.ResourceName == "" || compStatus.Kind == "" || compStatus.APIVersion == "" {
		logger.V(1).Info("Skipping health check: Component status missing primary resource info.", "component", compStatus.Name)
		compStatus.Health = false
		if compStatus.Message == "Initializing" || compStatus.Message == "Processing" || compStatus.Message == "Built successfully" || compStatus.Message == "Applied successfully, awaiting health check" {
			compStatus.Message = "Skipped health check: Resource info missing."
		}
		// Task completed its check (finding nothing to check), but component is unhealthy.
		// Return Complete as the *task* action is done. Overall app health depends on compStatus.Health.
		return TaskResultComplete, nil
	}

	// --- Perform live health check using kubeutil.CheckHealth ---
	isHealthy, message, checkProcessErr := kubeutil.CheckHealth(ctx, cli, scheme, // Pass client and scheme from context
		compStatus.Namespace,
		compStatus.ResourceName,
		compStatus.APIVersion,
		compStatus.Kind,
	)

	// --- Update Component Status based on check result ---
	compStatus.Health = isHealthy
	compStatus.Message = message

	// --- Determine Task Result ---
	if checkProcessErr != nil {
		logger.Error(checkProcessErr, "Failed to execute K8s workload readiness check process")
		// Task execution failed due to error during the check itself.
		// Return Failed and the error for backoff retry.
		return TaskResultFailed, checkProcessErr
	}

	if isHealthy {
		logger.V(1).Info("K8s workload readiness check passed", "kind", compStatus.Kind, "name", compStatus.ResourceName)
		return TaskResultComplete, nil // Workload is ready.
	} else {
		logger.V(1).Info("K8s workload not yet ready", "kind", compStatus.Kind, "name", compStatus.ResourceName, "status", message)
		// Workload not ready, task is pending. Signal to Task Runner to stop and requeue.
		return TaskResultPending, nil // Task is pending, no *task execution* error.
	}
}
