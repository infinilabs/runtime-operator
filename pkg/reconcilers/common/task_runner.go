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

// pkg/reconcilers/common/task_runner.go
package common

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	// For ApplyResult type

	appv1 "github.com/infinilabs/runtime-operator/api/app/v1"
	"github.com/infinilabs/runtime-operator/internal/controller/common/kubeutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TaskRunner executes a list of Tasks for a specific component.
// It prepares the TaskContext and iterates through the task list, handling results.
type TaskRunner struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	// TaskRegistry can be added here if tasks are looked up by name/type string
	// taskRegistry map[string]TaskFactory // Factory returns new Task instance
}

// NewTaskRunner creates a new TaskRunner instance.
func NewTaskRunner(client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder) *TaskRunner {
	return &TaskRunner{
		Client:   client,
		Scheme:   scheme,
		Recorder: recorder,
	}
}

// RunTasks executes a sequence of predefined Task implementations.
// It orchestrates the execution flow based on task results.
// owner: The owning ApplicationDefinition CR.
// taskContext: Pre-populated context for this component's reconcile run.
// taskList: Slice of Task interface implementations to execute sequentially.
// Returns the final TaskResult (Pending if any task is pending, Failed if any task fails critically, Complete otherwise)
// and the first critical error encountered during task execution.
func (r *TaskRunner) RunTasks(
	ctx context.Context,
	owner client.Object, // Pass owner (AppDef)
	appComp *appv1.ApplicationComponent,
	componentStatus *appv1.ComponentStatusReference,
	mergedConfig interface{},
	desiredObjects map[string]client.Object, // Pass map
	applyResults map[string]kubeutil.ApplyResult, // Pass map
	taskList []Task,
) (TaskResult, error) {
	logger := log.FromContext(ctx).WithValues("component", appComp.Name, "type", appComp.Type)

	if len(taskList) == 0 {
		return TaskResultComplete, nil
	}

	var firstError error
	overallResult := TaskResultComplete

	// --- Prepare Task Context (NOW includes Client, Scheme, Owner) ---
	taskContext := &TaskContext{
		Client:          r.Client,   // Populate from TaskRunner
		Scheme:          r.Scheme,   // Populate from TaskRunner
		Owner:           owner,      // Populate from owner passed to RunTasks
		Recorder:        r.Recorder, // Populate from TaskRunner
		Logger:          logger,
		AppDef:          owner.(*appv1.ApplicationDefinition), // Type assert owner
		AppComp:         appComp,
		ComponentStatus: componentStatus,
		MergedConfig:    mergedConfig,
		DesiredObjects:  desiredObjects, // Pass the maps
		ApplyResults:    applyResults,
	}

	for _, task := range taskList {
		taskName := getTaskName(task)
		taskLogger := logger.WithValues("task", taskName)
		taskContext.Logger = taskLogger // Update logger for this task

		startTime := time.Now()
		taskLogger.V(1).Info("Executing task")

		// *** UPDATED Execute Call ***
		// Call Execute with only ctx and taskContext
		taskResult, err := task.Execute(ctx, taskContext) // Pass the updated context

		duration := time.Since(startTime)
		taskLogger.V(1).Info("Task execution finished", "result", taskResult, "error", err, "duration", duration.String())

		// Handle task outcome (logic remains the same)
		if err != nil {
			firstError = err
			overallResult = TaskResultFailed
			break
		}
		if taskResult == TaskResultFailed {
			err = fmt.Errorf("task %s reported status Failed but returned nil error", taskName)
			logger.Error(err, "Task execution inconsistency")
			firstError = err
			overallResult = TaskResultFailed
			break
		}
		if taskResult == TaskResultPending {
			overallResult = TaskResultPending
			break
		}
		if taskResult == TaskResultSkipped {
			logger.V(1).Info("Task skipped execution")
		}
		// If TaskResultComplete, continue implicitly.

	} // End task list loop

	return overallResult, firstError
}

// getTaskName extracts a readable name from a Task implementation type using reflection.
func getTaskName(task Task) string {
	if task == nil {
		return "nil-task"
	}
	t := reflect.TypeOf(task)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	} // Get element type if pointer
	name := t.String() // Gets package.Type name
	// Clean up package path if present
	parts := strings.Split(name, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	} // Return just the type name
	return name // Fallback to full name
}
