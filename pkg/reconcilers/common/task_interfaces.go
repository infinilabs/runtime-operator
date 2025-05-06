// pkg/reconcilers/common/task_interfaces.go
package common

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appv1 "github.com/infinilabs/operator/api/app/v1"                    // App types needed for context
	"github.com/infinilabs/operator/internal/controller/common/kubeutil" // For ApplyResult type
	"k8s.io/apimachinery/pkg/runtime"                                    // Scheme needed in context/task execution
	"k8s.io/client-go/tools/record"                                      // Recorder needed in context
	"sigs.k8s.io/controller-runtime/pkg/client"                          // K8s client needed
)

// Task defines the contract for a single, potentially idempotent, step
// within the reconciliation flow for a component.
type Task interface {
	// GetName returns a unique identifier for this task type.
	GetName() string

	// Execute performs the task logic using the provided context.
	// It should be designed to be idempotent if possible.
	// ctx: Go context for cancellation and timeouts.
	// taskContext: Provides access to shared state and helpers for this reconcile run.
	// Returns:
	//   - TaskResult: Indicates if the task completed, is pending, or failed.
	//   - error: A critical error that occurred during task execution, preventing further progress.
	//          If result is TaskResultPending, error should typically be nil (indicating waiting, not failure).
	Execute(ctx context.Context, taskContext *TaskContext) (TaskResult, error)
}

// TaskResult indicates the outcome of a Task execution.
type TaskResult string

const (
	TaskResultComplete TaskResult = "Complete" // Task finished successfully in this reconciliation cycle.
	TaskResultPending  TaskResult = "Pending"  // Task is waiting (e.g., resource readiness), needs requeue.
	TaskResultFailed   TaskResult = "Failed"   // Task encountered an unrecoverable error, stopping the sequence.
	TaskResultSkipped  TaskResult = "Skipped"  // Task determined it didn't need to run (optional).
)

// TaskContext holds information available during a Task execution.
// It's populated by the TaskRunner before executing each task for a specific component.
type TaskContext struct {
	// --- Core Kubernetes Clients & Context ---
	Client   client.Client        // Kubernetes API client
	Scheme   *runtime.Scheme      // Kubernetes scheme for object types
	Owner    client.Object        // The owning ApplicationDefinition CR
	Recorder record.EventRecorder // For recording Kubernetes events
	Logger   logr.Logger          // Logger instance with component context

	// --- Application & Component Context ---
	AppDef  *appv1.ApplicationDefinition // The owner ApplicationDefinition CR
	AppComp *appv1.ApplicationComponent  // The specific Component being processed

	// --- Mutable State ---
	// Pointer to the status entry for the current component instance.
	// Tasks can update the Message and potentially Health fields based on their outcome.
	ComponentStatus *appv1.ComponentStatusReference

	// --- Data from Previous Stages (Builders, Apply) ---
	// The unmarshalled application-specific configuration for this component.
	// Type assertion needed within tasks (e.g., config := tc.MergedConfig.(*common.GatewayConfig)).
	MergedConfig interface{}

	// Map of desired K8s objects built by the builder strategy for this component.
	// Key: Unique identifier (e.g., GVK string + NsName string).
	DesiredObjects map[string]client.Object

	// Map of apply results from the initial apply phase for desired objects.
	// Key: Unique identifier (e.g., GVK string + NsName string).
	ApplyResults map[string]kubeutil.ApplyResult

	// TODO: Add fields for sharing state *between* tasks if needed (e.g., primary resource GVK/Name identified by an earlier task).
	// Example: PrimaryResourceKey string // Key to the primary workload in DesiredObjects/ApplyResults
}

// Helper to easily get desired object by key
func (tc *TaskContext) GetDesiredObject(gvkNsName string) (client.Object, bool) {
	obj, ok := tc.DesiredObjects[gvkNsName]
	return obj, ok
}

// Helper to easily get apply result by key
func (tc *TaskContext) GetApplyResult(gvkNsName string) (kubeutil.ApplyResult, bool) {
	result, ok := tc.ApplyResults[gvkNsName]
	return result, ok
}

// Helper to update status message safely
func (tc *TaskContext) SetStatusMessage(format string, args ...interface{}) {
	if tc.ComponentStatus != nil {
		newMessage := fmt.Sprintf(format, args...)
		tc.Logger.V(1).Info("Updating component status message", "oldMessage", tc.ComponentStatus.Message, "newMessage", newMessage)
		tc.ComponentStatus.Message = newMessage
	} else {
		tc.Logger.Error(nil, "Attempted to set status message but ComponentStatus is nil in TaskContext")
	}
}

// Helper to update health status safely
func (tc *TaskContext) SetHealthStatus(isHealthy bool) {
	if tc.ComponentStatus != nil {
		if tc.ComponentStatus.Health != isHealthy {
			tc.Logger.V(1).Info("Updating component health status", "newHealth", isHealthy)
		}
		tc.ComponentStatus.Health = isHealthy
	} else {
		tc.Logger.Error(nil, "Attempted to set health status but ComponentStatus is nil in TaskContext")
	}
}
