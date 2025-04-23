// pkg/reconcilers/common/task_runner.go
package common

import (
	"context"
	"fmt"
	"reflect" // For getting Task type name
	"strings"

	appv1 "github.com/infinilabs/operator/api/app/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TaskRunner executes a list of Tasks for a specific component.
type TaskRunner struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder // For recording events

	// Task implementations registry might be needed here or accessible.
	// For simplicity, the TaskRunner could receive the Task implementations list directly.
}

// RunTasks executes a sequence of tasks for a component instance.
// It receives a list of Task Implementations (concrete structs implementing Task interface).
// appDef is the overall application (owner). appComp is the specific component.
// componentStatus is the status entry for THIS component instance (mutable).
// taskSpecificData (Optional) - If tasks need access to pre-calculated data or config specific to THIS task execution (e.g. specific objects to apply).
func (r *TaskRunner) RunTasks(ctx context.Context, appDef *appv1.ApplicationDefinition, appComp *appv1.ApplicationComponent, componentStatus *appv1.ComponentStatusReference, taskList []Task) (TaskResult, error) {
	logger := log.FromContext(ctx).WithValues("component", appComp.Name, "type", appComp.Type) // Logger for this component
	owner := appDef                                                                            // ApplicationDefinition is the owner

	// --- Prepare Task Context (Shared across all tasks run by THIS runner) ---
	taskContext := &TaskContext{
		Logger:          logger, // Task-specific logger (component context)
		AppDef:          appDef,
		AppComp:         appComp,
		ComponentStatus: componentStatus, // Mutable status pointer
		// Add other context info populated by main controller BEFORE calling runner
		// e.g., DesiredObjects (list), AppliedObjects, ApplyResults map, unmarshalledAppSpecificConfig etc.
		Recorder: r.Recorder, // Recorder for events
	}

	for _, task := range taskList {
		// Use a unique name for this task instance for logging
		taskName := reflect.TypeOf(task).Elem().String() // Example task name (removes pointer indicator *)
		// Clean up TaskName if it includes package path for logging
		taskNameParts := strings.Split(taskName, ".")
		if len(taskNameParts) > 0 {
			taskName = taskNameParts[len(taskNameParts)-1]
		} // Use just the type name

		taskLogger := logger.WithValues("task", taskName)
		taskLogger.V(1).Info("Starting task execution")

		// --- Execute the current task ---
		// taskContext is passed by reference, so tasks can update the ComponentStatus message.
		taskResult, err := task.Execute(ctx, r.Client, r.Scheme, owner, taskContext)
		// --- Task execution finished ---

		// --- Handle task results and errors ---
		if err != nil {
			// If any task fails, the task sequence is interrupted. Overall result is Failed.
			logger.Error(err, "Task execution failed", "task", taskName)
			// The task's Execute method is responsible for setting a detailed message in taskContext.ComponentStatus.
			// If message wasn't set, set a default failure message here.
			if taskContext.ComponentStatus.Message == "Processing" || taskContext.ComponentStatus.Message == "Initializing" {
				taskContext.ComponentStatus.Message = fmt.Sprintf("Task %s failed with error: %v", taskName, err)
			}

			return TaskResultFailed, fmt.Errorf("task '%s' failed: %w", taskName, err) // Wrap error and return to caller (main controller)
		}

		if taskResult == TaskResultPending {
			// If a task returns Pending, stop the sequence and signal pending state.
			logger.V(1).Info("Task returned pending, stopping sequence", "task", taskName)
			// The task should set a descriptive message in taskContext.ComponentStatus.Message
			return TaskResultPending, nil // Signal overall task runner is pending
		}

		if taskResult == TaskResultComplete {
			logger.V(1).Info("Task completed successfully", "task", taskName)
			// Continue to the next task
			// Ensure taskContext.ComponentStatus.Message reflects this if no subsequent tasks will overwrite.
			// The *last* message from a completed task often becomes the final message before health check.
			if taskContext.ComponentStatus.Message == "Processing" || taskContext.ComponentStatus.Message == "Initializing" {
				taskContext.ComponentStatus.Message = fmt.Sprintf("Task %s complete.", taskName)
			} else if strings.Contains(taskContext.ComponentStatus.Message, "Building") { // Example
				taskContext.ComponentStatus.Message = "Objects built, starting apply." // Message reflects transition
			} else if strings.Contains(taskContext.ComponentStatus.Message, "Applied") {
				taskContext.ComponentStatus.Message = "Objects applied, checking readiness."
			}

			continue // Move to the next task in the list
		}

		// Handle other TaskResults if defined

	} // End task list loop

	// If the runner finished the task list without interruption
	logger.V(1).Info("All tasks completed for this component/stage successfully")
	return TaskResultComplete, nil // Overall task sequence for this component completed successfully
}
