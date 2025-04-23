// pkg/reconcilers/common/task_interfaces.go
package common

import (
	"context"

	appv1 "github.com/infinilabs/operator/api/app/v1" // ApplicationDefinition, Component types
	// Common types can be imported if needed by task parameters or results
	// "github.com/infinilabs/operator/pkg/apis/common"

	// Kubernetes and controller-runtime types
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Task defines the contract for a single step or unit of work within the reconciliation flow.
// Specific task implementations (like ApplyResourcesTask, CheckWorkloadReadyTask)
// implement this interface.
type Task interface {
	// Execute performs the task logic.
	// Receives standard K8s context, client, scheme, owner object, and task-specific context.
	// Returns TaskResult (Complete, Pending, Failed) and an error if the task encountered a failure.
	Execute(ctx context.Context, client client.Client, scheme *runtime.Scheme, owner client.Object, taskContext *TaskContext) (TaskResult, error)
}

// TaskResult indicates the outcome of a Task execution.
type TaskResult string

const (
	TaskResultComplete TaskResult = "Complete" // Task finished successfully in this reconciliation cycle.
	TaskResultPending  TaskResult = "Pending"  // Task is ongoing or waiting for resources/state, needs more time. Requeue required.
	TaskResultFailed   TaskResult = "Failed"   // Task encountered an unrecoverable error. Requeue with backoff required.
	// Add other states if needed (e.g., TaskResultSkipped).
)

// TaskContext holds information available during a Task execution.
// It provides access to necessary state and data from the main reconciliation.
type TaskContext struct {
	Logger log.Logger // Logger for this task instance

	// --- Essential K8s Context ---
	// Client is passed directly to Execute methods. Scheme, Owner also passed directly.
	// Use AppDef from the main controller scope.

	// --- Application and Component Context ---
	// Pass context about the specific component instance and the overall application.
	// A Task usually operates on a specific component instance's resources/state.
	// Task needs to know WHICH component it is dealing with.
	AppDef  *appv1.ApplicationDefinition // Reference to the owner ApplicationDefinition CR
	AppComp *appv1.ApplicationComponent  // Reference to the specific Component being processed by this task

	// --- Component Status Context ---
	// Pointer to the status entry for the current component instance (mutable by tasks).
	ComponentStatus *appv1.ComponentStatusReference

	// --- Resource State Context ---
	// These fields represent the state AS OF THE START of the task execution (or phase).
	// If a task modifies resources, these might become outdated for subsequent tasks in the SAME reconcile.
	// Need a strategy to access the *live* state if tasks require it, or pass updated lists.

	// List of all DESIRED K8s objects built by the builder strategy (all components or subset?)
	// It's more practical for a task to get *its specific* relevant desired objects.
	// Let's refine TaskContext design later if complexity requires. For now, keep it simple.
	// Access state from main controller or task runner.

	// A task needs access to:
	// - The K8s resources it needs to operate on (Apply task needs DesiredObjects, CheckHealth needs Resource GVK/Name)
	// - The configuration it needs (specific to this component type) - need pointer to appSpecificConfig.
	// - Apply results map (to record its own apply outcomes if it applies resources).
	// - Event Recorder

	// Let's rethink TaskContext structure based on needs:
	// AppContext       AppReconcileContext // Contains references to AppDef, AppComp, Logger etc.
	// MergedConfig interface{} // Unmarshalled application-specific configuration

	// Maybe simpler: A Task is called WITH the specific data it needs, derived by the main controller/task runner.
	// E.g., CheckWorkloadReadyTask(..., componentStatus, primaryResourceGVK, primaryResourceName)

	// For simplicity in this framework, let TaskContext hold common info.
	// Add other needed fields directly to TaskContext based on tasks requiring them.

	// --- Context about resources this task handles (Example: passed by TaskRunner) ---
	// RelevantDesiredObjects []client.Object // Subset of desired objects for this task

	// --- Results storage (Example: passed/updated by TaskRunner or in shared state) ---
	// Results map, Applied objects list, etc. These might live in the TaskRunner or main reconcileState.

	// Example structure of TaskContext:
	// K8sContext common_reconcilers.K8sTaskContext // Client, Scheme, Logger, Owner, Recorder
	// AppCompContext common_reconcilers.AppCompTaskContext // AppDef, AppComp, ComponentStatus

	// For now, keep existing fields that are useful across simple tasks:
	// Logger, AppDef, AppComp, ComponentStatus. Add others as needed by Task implementations.

	// Add field for unmarshalled application specific configuration, since tasks need it.
	// MergedConfig interface{} // Unmarshalled app-specific config struct pointer or value (set by runner)
	// This might not be necessary if the Task is responsible for getting config itself OR strategy builds tasks specific to config type.
	// Let's pass it, simplifying task implementations.

	// Add Event Recorder to TaskContext if tasks record events
	Recorder record.EventRecorder
}

// K8sTaskContext contains common K8s related objects needed by tasks.
type K8sTaskContext struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Logger   log.Logger           // Logger for this task
	Owner    client.Object        // Owner object (e.g., ApplicationDefinition)
	Recorder record.EventRecorder // Event Recorder for this task or component
}

// AppCompTaskContext contains application and component specific context.
type AppCompTaskContext struct {
	AppDef          *appv1.ApplicationDefinition
	AppComp         *appv1.ApplicationComponent
	ComponentStatus *appv1.ComponentStatusReference // Mutable status entry
}

// Let's adjust Task interface and TaskContext to simplify. TaskContext can bundle necessary data.
// Revert to the original Task Interface signature I gave with K8s context.

/* Reverting Task Interface to earlier simple version if TaskContext bundles info

type Task interface {
	// Execute performs the task.
	// It receives the main reconcile context, client, scheme, owner, and specific task context data.
	Execute(ctx context.Context, client client.Client, scheme *runtime.Scheme, owner client.Object, taskData interface{}) (TaskResult, error) // TaskData is task specific
}
// This makes TaskRunner complex as it needs to know what 'taskData' to provide for each task type.

*/

// Stick with the Task interface definition at the top: Execute(ctx, client, scheme, owner, taskContext)

// The structure of TaskContext seems adequate if populated correctly by the TaskRunner or caller.
