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

// Package app reconciles ApplicationDefinition objects
package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cisco-open/operator-tools/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appv1 "github.com/infinilabs/runtime-operator/api/app/v1"
	"github.com/infinilabs/runtime-operator/internal/controller/common/kubeutil"
	"github.com/infinilabs/runtime-operator/pkg/apis/common"
	commonutil "github.com/infinilabs/runtime-operator/pkg/apis/common/util"
	"github.com/infinilabs/runtime-operator/pkg/strategy"
	"github.com/infinilabs/runtime-operator/pkg/webrecorder"
)

const (
	appDefFinalizer   = "infini.cloud/finalizer"
	appNameLabel      = "infini.cloud/application-name"
	compNameLabel     = "infini.cloud/component-name"
	compInstanceLabel = "infini.cloud/component-instance"
)

// reconcileState holds the state throughout a single reconciliation loop.
type reconcileState struct {
	appDef              *appv1.ApplicationDefinition
	originalStatus      *appv1.ApplicationDefinitionStatus
	desiredObjects      []client.Object                            // List of objects built by strategies
	componentStatuses   map[string]*appv1.ComponentStatusReference // Current status per component
	applyResults        map[string]kubeutil.ApplyResult            // Results from applying desiredObjects
	unmarshalledConfigs map[string]interface{}                     // Store unmarshalled config per component [Added]
	firstError          error                                      // First critical error encountered
}

// ApplicationDefinitionReconciler reconciles ApplicationDefinition objects.
type ApplicationDefinitionReconciler struct {
	Client     client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	RESTMapper meta.RESTMapper // Keep if needed for GC or complex lookups
	Reconciler reconciler.ResourceReconciler
}

// RBAC markers... (Ensure they cover all necessary types, including ComponentDefinitions)
//+kubebuilder:rbac:groups=infini.cloud,resources=applicationdefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infini.cloud,resources=applicationdefinitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infini.cloud,resources=applicationdefinitions/finalizers,verbs=update
//+kubebuilder:rbac:groups=core.infini.cloud,resources=componentdefinitions,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=services;persistentvolumeclaims;configmaps;secrets;serviceaccounts,verbs=get;list;watch;create;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups=apps,resources=deployments;statefulsets;daemonsets,verbs=get;list;watch;create;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch

func (r *ApplicationDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// check if the request is for the correct namespace
	if req.NamespacedName.Namespace != common.Namespace {
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("appdefinition", req.NamespacedName)
	startTime := time.Now()
	logger.V(1).Info("Starting ApplicationDefinition reconciliation")

	// 1. Initialize state and fetch the ApplicationDefinition
	state := &reconcileState{
		appDef:              &appv1.ApplicationDefinition{},
		desiredObjects:      []client.Object{},
		componentStatuses:   make(map[string]*appv1.ComponentStatusReference),
		applyResults:        make(map[string]kubeutil.ApplyResult),
		unmarshalledConfigs: make(map[string]interface{}), // Initialize map [Added]
	}

	if err := r.Client.Get(ctx, req.NamespacedName, state.appDef); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate requeue.
		// No need to change state if we don't find the object.
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to get ApplicationDefinition")
			return ctrl.Result{}, err // Return error for retry
		}
		logger.Info("ApplicationDefinition resource not found, assuming deleted")
		return ctrl.Result{}, nil // Object is gone, stop reconciliation
	}

	appDef := state.appDef
	// Initialize Status if nil
	if appDef.Status.Conditions == nil {
		appDef.Status.Conditions = []metav1.Condition{}
	}

	// DeepCopy the status for comparison later
	state.originalStatus = state.appDef.Status.DeepCopy()

	// Record reconciliation start event
	r.recordEvent(state.appDef, "Reconcile", webrecorder.StatusInProgress, "SyncComponent",
		corev1.EventTypeNormal, "ReconcileStarted", "Starting reconciliation")

	// Initialize component status map based on current spec
	if err := r.initializeComponentStatuses(state); err != nil {
		// Initialization error is critical, update status and stop
		return r.handleReconcileError(ctx, state, "InitializationFailed", err)
	}
	logger.V(1).Info("Initialized component status map", "count", len(state.componentStatuses))

	// Handle case where application has no components defined
	if len(state.appDef.Spec.Components) == 0 {
		return r.handleEmptyApp(ctx, state)
	}

	// 2. Handle Finalizer logic
	isDeleted, err := r.handleFinalizer(ctx, state)
	if err != nil {
		return ctrl.Result{}, err
	}
	if isDeleted {
		return ctrl.Result{}, nil
	}

	// 3. Set initial processing phase if needed
	phaseUpdated, err := r.setInitialPhase(ctx, state)
	if err != nil {
		// Error updating status, retry
		return ctrl.Result{}, err
	}
	if phaseUpdated {
		// Status was updated, requeue immediately to continue with the new phase
		return ctrl.Result{Requeue: true}, nil
	}

	// 3.5. Check if application is suspended
	isSuspended := state.appDef.Spec.Suspend != nil && *state.appDef.Spec.Suspend
	if isSuspended && state.appDef.Status.Phase == appv1.ApplicationPhaseSuspended {
		// Already suspended and phase is set, skip reconciliation
		logger.Info("Application is suspended, skipping reconciliation")
		return ctrl.Result{}, nil
	}
	// If suspended but phase not yet set, continue to apply the suspend logic below

	// 4. Process components: Unmarshal Config, Dispatch to Builder Strategy, Build Objects
	processErr := r.processComponentsAndBuildObjects(ctx, state)
	if processErr != nil {
		// Building failed, update status and stop reconciliation for this cycle
		state.firstError = processErr // Store the error
		return r.handleReconcileError(ctx, state, "ProcessingFailed", processErr)
	}
	logger.V(1).Info("Object building successful", "objectCount", len(state.desiredObjects))

	// 5. Apply generated resources using Server-Side Apply
	applyErr := r.applyResources(ctx, state)
	if applyErr != nil && state.firstError == nil {
		// Record the first apply error if no prior critical error occurred
		state.firstError = applyErr
	}
	// Note: Even if applyErr occurs, we continue to health checks to report current state.

	// 6. Check health and calculate overall status
	var allReady bool
	var needsRequeue bool
	var healthCheckErr error
	if state.firstError == nil { // Only run detailed health checks if build and apply were successful initially
		allReady, needsRequeue, healthCheckErr = r.checkHealthAndCalculateStatus(ctx, state)
		if healthCheckErr != nil {
			logger.Error(healthCheckErr, "Error occurred during health checking phase")
			if state.firstError == nil {
				state.firstError = healthCheckErr // Record health check process error
			}
			// If health check process failed, we likely need to requeue
			needsRequeue = true
		}
	} else {
		// If there was a build or apply error, skip detailed health checks
		logger.V(1).Info("Skipping detailed health check due to prior critical error", "firstError", state.firstError)
		// Mark as not ready and requeue needed because of the prior error
		allReady = false
		needsRequeue = true // Requeue because of the build/apply error
		// Update component statuses to reflect the build/apply error if not already done
		r.updateComponentStatusesForError(state, state.firstError)
	}

	// 7. Determine final phase and update status if needed
	r.determineFinalPhase(state, allReady)                                              // Update phase based on errors and readiness
	state.appDef.Status.Components = mapToSliceComponentStatus(state.componentStatuses) // Update components status list

	// Update LastChangeID in status if reconciliation was successful
	if allReady && state.firstError == nil {
		if changeID, exists := state.appDef.Annotations[appv1.AnnotationChangeID]; exists && changeID != "" {
			state.appDef.Status.LastChangeID = changeID
		}
	}

	_, statusUpdateErr := r.updateStatusIfNeeded(ctx, state.appDef, state.originalStatus)
	if statusUpdateErr != nil {
		// Distinguish between conflicts (expected, will retry) and other errors
		if apierrors.IsConflict(statusUpdateErr) {
			logger.V(1).Info("Status update conflict, will retry automatically")
		} else {
			logger.Error(statusUpdateErr, "Status update failed")
		}
		if state.firstError == nil {
			// Record status update error if no other error occurred
			state.firstError = statusUpdateErr
		}
		// If status update fails (e.g., conflict), we must requeue
		needsRequeue = true
	}

	// 8. Log result and return
	reconciliationDuration := time.Since(startTime)
	logger.Info("Reconciliation completed",
		"duration", reconciliationDuration.String(),
		"phase", state.appDef.Status.Phase,
		"requeue", needsRequeue,
	)

	// Record reconciliation completion event
	if state.firstError != nil {
		r.recordEventf(state.appDef, "Reconcile", webrecorder.StatusFailure, "SyncComponent",
			corev1.EventTypeWarning, "ReconcileFailed", "Reconciliation failed: %v", state.firstError)
	} else if allReady {
		r.recordEvent(state.appDef, "Reconcile", webrecorder.StatusSuccess, "SyncComponent",
			corev1.EventTypeNormal, "ReconcileCompleted", "Reconciliation completed successfully, all components ready")
	} else {
		r.recordEvent(state.appDef, "Reconcile", webrecorder.StatusInProgress, "SyncComponent",
			corev1.EventTypeNormal, "ReconcileProgressing", "Reconciliation in progress, waiting for components to be ready")
	}

	if needsRequeue {
		// Use a default requeue interval, or adjust based on error type later
		requeueInterval := 30 * time.Second
		if apierrors.IsConflict(statusUpdateErr) {
			requeueInterval = 5 * time.Second // Shorter retry for conflicts
		}
		logger.V(1).Info("Requeuing requested", "interval", requeueInterval.String())
		// Return the first critical error encountered. If statusUpdateErr occurred, it might hide the root cause.
		// Prioritizing the firstError seems reasonable.
		return ctrl.Result{RequeueAfter: requeueInterval}, state.firstError
	}

	// No requeue needed and no error occurred (or errors were handled and don't require immediate retry)
	return ctrl.Result{}, state.firstError // Return firstError (might be nil)
	// return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, state.firstError // Return firstError (might be nil)
}

// --- Helper Methods ---

// initializeComponentStatuses populates the initial status map.
func (r *ApplicationDefinitionReconciler) initializeComponentStatuses(state *reconcileState) error {
	names := make(map[string]bool)
	for _, comp := range state.appDef.Spec.Components {
		if comp.Name == "" {
			return fmt.Errorf("component name cannot be empty in spec")
		}
		if names[comp.Name] {
			return fmt.Errorf("duplicate component name found in spec: %s", comp.Name)
		}
		names[comp.Name] = true

		state.componentStatuses[comp.Name] = &appv1.ComponentStatusReference{
			Name:    comp.Name,
			Health:  false, // Default to unhealthy
			Message: "Initializing",
		}
	}
	// TODO: Optionally remove statuses for components that are no longer in the spec?
	// This might be better handled by K8s GC based on owner refs.
	return nil
}

// handleEmptyApp handles the case of no components.
func (r *ApplicationDefinitionReconciler) handleEmptyApp(ctx context.Context, state *reconcileState) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("ApplicationDefinition has no components defined.")
	state.appDef.Status.Phase = appv1.ApplicationPhaseRunning // Consider it Available if empty
	state.appDef.Status.Components = []appv1.ComponentStatusReference{}
	setCondition(state.appDef, metav1.Condition{Type: string(appv1.ConditionReady), Status: metav1.ConditionTrue, Reason: "NoComponentsDefined", Message: "Application has no components defined"})

	// TODO: Implement logic to garbage collect orphaned resources previously managed by this AppDef?
	// This is complex and relies on labels/ownerRefs. For now, assume GC handles it.

	if _, updateErr := r.updateStatusIfNeeded(ctx, state.appDef, state.originalStatus); updateErr != nil {
		logger.Error(updateErr, "Status update failed for empty app")
		return ctrl.Result{}, updateErr // Retry status update
	}
	return ctrl.Result{}, nil // Success, no requeue needed
}

// handleFinalizer adds or removes the finalizer.
func (r *ApplicationDefinitionReconciler) handleFinalizer(ctx context.Context, state *reconcileState) (isDeleted bool, err error) {
	logger := log.FromContext(ctx)
	appDef := state.appDef

	if appDef.ObjectMeta.DeletionTimestamp.IsZero() {
		// Object is NOT being deleted
		if !controllerutil.ContainsFinalizer(appDef, appDefFinalizer) {
			logger.Info("Adding Finalizer")
			controllerutil.AddFinalizer(appDef, appDefFinalizer)
			// Retry loop for adding finalizer
			for i := 0; i < 3; i++ {
				if err := r.Client.Update(ctx, appDef); err != nil {
					if apierrors.IsConflict(err) {
						logger.Info("Conflict adding finalizer, retrying...", "attempt", i+1)
						// Re-fetch the latest version
						if fetchErr := r.Client.Get(ctx, client.ObjectKeyFromObject(appDef), appDef); fetchErr != nil {
							logger.Error(fetchErr, "Failed to re-fetch appDef after conflict")
							return false, fetchErr
						}
						// Re-add the finalizer since appDef was re-fetched
						controllerutil.AddFinalizer(appDef, appDefFinalizer)
						continue
					}
					logger.Error(err, "Failed to add finalizer")
					return false, err
				}
				// Success, finalizer added
				return false, nil
			}
			// If we get here, all retries failed
			return false, fmt.Errorf("failed to add finalizer after 3 attempts")
		}
	} else {
		// Object IS being deleted
		if controllerutil.ContainsFinalizer(appDef, appDefFinalizer) {
			logger.Info("Performing cleanup before finalizer removal")

			// --- Add Application Cleanup Logic Here ---
			// Example: Delete external resources, notify external systems, etc.
			// If cleanup fails, return an error to retry cleanup.
			// if err := r.cleanupExternalResources(ctx, appDef); err != nil {
			//     logger.Error(err, "External resource cleanup failed")
			//     return true, err // isDeleted=true because deletion is in progress, return error to retry cleanup
			// }
			// -----------------------------------------

			logger.Info("Cleanup complete, removing Finalizer")
			controllerutil.RemoveFinalizer(appDef, appDefFinalizer)
			// Retry loop for removing finalizer
			for i := 0; i < 3; i++ {
				if err := r.Client.Update(ctx, appDef); err != nil {
					if apierrors.IsConflict(err) {
						logger.Info("Conflict removing finalizer, retrying...", "attempt", i+1)
						// Re-fetch the latest version
						if fetchErr := r.Client.Get(ctx, client.ObjectKeyFromObject(appDef), appDef); fetchErr != nil {
							logger.Error(fetchErr, "Failed to re-fetch appDef after conflict")
							return true, fetchErr
						}
						// Re-remove the finalizer since appDef was re-fetched
						controllerutil.RemoveFinalizer(appDef, appDefFinalizer)
						continue
					}
					logger.Error(err, "Failed to remove finalizer")
					return true, err
				}
				// Success, finalizer removed
				logger.Info("Finalizer removed successfully")
				break
			}
		}
		// Object is being deleted, stop further reconciliation
		return true, nil
	}

	// Object is not being deleted and finalizer exists (or was just added)
	return false, nil
}

// setInitialPhase updates status phase if it's empty or pending.
func (r *ApplicationDefinitionReconciler) setInitialPhase(ctx context.Context, state *reconcileState) (bool, error) {
	currentPhase := state.appDef.Status.Phase
	if currentPhase == "" || currentPhase == appv1.ApplicationPhasePending {
		state.appDef.Status.Phase = appv1.ApplicationPhaseCreating
		setCondition(state.appDef, metav1.Condition{
			Type:    string(appv1.ConditionReady),
			Status:  metav1.ConditionFalse,
			Reason:  "Processing",
			Message: "Starting component processing"})

		// 需要持久化更新到cr中，否则status一直不会更新，不会触发后续的apply
		// Retry loop for status update
		for i := 0; i < 3; i++ {
			err := r.Client.Status().Update(ctx, state.appDef)
			if err != nil {
				if apierrors.IsConflict(err) {
					logger := log.FromContext(ctx)
					logger.Info("Conflict updating status in setInitialPhase, retrying...", "attempt", i+1)
					// Re-fetch the latest status
					if fetchErr := r.Client.Get(ctx, client.ObjectKeyFromObject(state.appDef), state.appDef); fetchErr != nil {
						logger.Error(fetchErr, "Failed to re-fetch appDef after status update conflict")
						return false, fetchErr
					}
					// Reapply the status changes since appDef was re-fetched
					state.appDef.Status.Phase = appv1.ApplicationPhaseCreating
					setCondition(state.appDef, metav1.Condition{
						Type:    string(appv1.ConditionReady),
						Status:  metav1.ConditionFalse,
						Reason:  "Processing",
						Message: "Starting component processing"})
					continue
				}
				return false, err
			}
			// Success, status updated
			r.recordEvent(state.appDef, "Processing", webrecorder.StatusInProgress, "SyncComponent",
				corev1.EventTypeNormal, "Processing", "Starting component processing")
			// Status will be updated later if needed, just return true to signal requeue
			return true, nil // Signal that phase changed and might need status update + requeue
		}
		// If we get here, all retries failed
		return false, fmt.Errorf("failed to update status in setInitialPhase after 3 attempts")
	}
	return false, nil // Phase already set, no update needed now
}

// processComponentsAndBuildObjects iterates through components, gets strategies, builds objects.
func (r *ApplicationDefinitionReconciler) processComponentsAndBuildObjects(ctx context.Context, state *reconcileState) error {
	logger := log.FromContext(ctx)
	appDef := state.appDef
	state.desiredObjects = []client.Object{} // Ensure clean slate for this cycle

	for i := range appDef.Spec.Components {
		appComp := appDef.Spec.Components[i] // Use index to get mutable reference if needed, but copy is safer
		appComp.Type = "operator"

		compLogger := logger.WithValues("component", appComp.Name, "componentType", appComp.Type)
		compStatus := state.componentStatuses[appComp.Name] // Get status entry

		compStatus.Message = "Processing" // Update status message

		compLogger.V(1).Info("Calling builder strategy BuildObjects")

		// 1. Get Builder Strategy
		builder, found := strategy.GetAppBuilderStrategy(appComp.Type)
		if !found {
			err := fmt.Errorf("no builder strategy registered for component type: %s", appComp.Type)
			logger.Error(err, "Builder strategy not found")
			r.recordEventf(appDef, "BuildObjects", webrecorder.StatusFailure, "SyncComponent",
				corev1.EventTypeWarning, "BuilderStrategyNotFound", err.Error())
			return err
		}

		// 2. Unmarshal Specific Config
		config, err := commonutil.UnmarshalAppSpecificConfig(appComp.Type, appComp.Properties)
		if err != nil {
			err = fmt.Errorf("failed to unmarshal properties for component '%s': %w", appComp.Name, err)
			logger.Error(err, "Config unmarshal failed")
			return err
		}
		state.unmarshalledConfigs[appComp.Name] = config // Store for later use

		// 3. Build Objects
		objects, err := builder.BuildObjects(ctx, r.Client, r.Scheme, state.appDef, state.appDef, &appDef.Spec.Components[i], config)
		if err != nil {
			err = fmt.Errorf("builder strategy failed for component %s: %w", appComp.Name, err)
			logger.Error(err, "Builder strategy failed", "error", err)
			r.recordEventf(appDef, "BuildObjects", webrecorder.StatusFailure, "SyncComponent",
				corev1.EventTypeWarning, "BuilderFailed", err.Error())
			return err
		}

		// Process built objects
		for _, obj := range objects {
			if obj == nil {
				compLogger.Info("Warning: Builder strategy returned a nil object, skipping")
				continue
			}
			// Ensure metadata like Name and Namespace are set by builders
			if obj.GetName() == "" || obj.GetNamespace() == "" {
				gvkStr := "unknown GVK"
				if gvk := obj.GetObjectKind().GroupVersionKind(); gvk.Kind != "" {
					gvkStr = gvk.String()
				}
				err := fmt.Errorf("builder returned object of type %s without name or namespace for component %s", gvkStr, appComp.Name)
				compLogger.Error(err, "Invalid built object")
				r.updateComponentStatusWithError(compStatus, "InvalidBuiltObject", err.Error())
				return err // Critical build error
			}
			// Apply standard labels
			labels := obj.GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}
			labels[appNameLabel] = appDef.Name
			labels[compNameLabel] = appComp.Type
			labels[compInstanceLabel] = appComp.Name
			labels[common.ManagedByLabel] = common.OperatorName
			obj.SetLabels(labels)

			state.desiredObjects = append(state.desiredObjects, obj)

			// Update status with primary resource details if matching GVK from CompDef
			gvk := obj.GetObjectKind().GroupVersionKind()
			if gvk.Kind == compStatus.Kind && gvk.GroupVersion().String() == compStatus.APIVersion {
				compStatus.ResourceName = obj.GetName()
				compStatus.Namespace = obj.GetNamespace()
				compLogger.V(1).Info("Identified primary resource", "kind", gvk.Kind, "name", obj.GetName())
			}
		}

		compStatus.Message = "Built successfully" // Update status after successful build for this component
		compLogger.V(1).Info("Component processed successfully", "builtObjectCount", len(objects))
	}

	// [Added] Handle Pause/Resume Logic
	if err := r.handlePauseResume(ctx, state); err != nil {
		return fmt.Errorf("failed to handle pause/resume: %w", err)
	}

	return nil // All components processed without critical error
}

// applyResources applies the generated objects using SSA.
func (r *ApplicationDefinitionReconciler) applyResources(ctx context.Context, state *reconcileState) error {
	logger := log.FromContext(ctx)
	appDef := state.appDef
	if len(state.desiredObjects) == 0 {
		logger.V(1).Info("No desired objects to apply.")
		return nil
	}
	logger.V(1).Info("Applying generated resources", "count", len(state.desiredObjects))

	var firstApplyErr error

	for _, obj := range state.desiredObjects {
		gvk := obj.GetObjectKind().GroupVersionKind()
		objKey := client.ObjectKeyFromObject(obj)
		resultMapKey := kubeutil.BuildObjectResultMapKey(obj)

		// Before applying, check if the object is a Service and preserve its ClusterIP.
		// This is the core fix for the "field is immutable" error.
		if svc, ok := obj.(*corev1.Service); ok {
			// Create a placeholder for the current service state in the cluster
			currentSvc := &corev1.Service{}
			// Try to get the service from the cluster
			err := r.Client.Get(ctx, objKey, currentSvc)

			if err != nil && !apierrors.IsNotFound(err) {
				// If we failed to get the service for any reason other than it not existing,
				// this is a real error. We should stop and requeue.
				return fmt.Errorf("failed to get existing service %s: %w", objKey, err)
			}

			// If the service was found (err is nil)
			if err == nil {
				// Preserve the existing ClusterIP in our desired object.
				// This prevents the apply operation from trying to change an immutable field.
				svc.Spec.ClusterIP = currentSvc.Spec.ClusterIP
			}
			// If the service was not found (apierrors.IsNotFound(err) was true),
			// we do nothing. The 'svc' object with an empty ClusterIP will be used to create a new service,
			// and Kubernetes will assign a new IP, which is the correct behavior.
		}

		// Set Owner Reference before applying
		if err := controllerutil.SetControllerReference(appDef, obj, r.Scheme); err != nil {
			// This is critical, log and potentially stop/return error
			errMsg := fmt.Sprintf("Failed to set OwnerReference on %s %s: %v", gvk.Kind, objKey.String(), err)
			logger.Error(err, errMsg)
			r.recordEventf(appDef, "ApplyResources", webrecorder.StatusFailure, "SyncComponent",
				corev1.EventTypeWarning, "SetOwnerRefFailed", errMsg)
			if firstApplyErr == nil {
				firstApplyErr = fmt.Errorf(errMsg) // Wrap error
			}
			state.applyResults[resultMapKey] = kubeutil.ApplyResult{Error: firstApplyErr}
			r.updateComponentStatusForApplyError(state.componentStatuses, obj, firstApplyErr)
			return firstApplyErr // Stop applying on owner ref failure
		}

		// Apply using Server-Side Apply utility
		applyResult := kubeutil.ApplyObjectV2(ctx, r.Reconciler, obj, common.OperatorName) // Use constant for field manager
		state.applyResults[resultMapKey] = applyResult

		if applyResult.Error != nil {
			if apierrors.IsConflict(applyResult.Error) {
				logger.Info("Optimistic lock conflict detected, will requeue and retry.", "resource", objKey, "kind", gvk.Kind)
				if firstApplyErr == nil {
					firstApplyErr = applyResult.Error
				}
			} else {
				errMsg := fmt.Sprintf("Failed to apply resource %s %s: %v", gvk.Kind, objKey.String(), applyResult.Error)
				logger.Error(applyResult.Error, errMsg)
				r.recordEventf(appDef, "ApplyResources", webrecorder.StatusFailure, "SyncComponent",
					corev1.EventTypeWarning, "ResourceApplyFailed", errMsg)
				if firstApplyErr == nil {
					firstApplyErr = applyResult.Error
				}
				r.updateComponentStatusForApplyError(state.componentStatuses, obj, applyResult.Error)
			}
		} else {
			logger.V(1).Info("Successfully applied resource", "kind", gvk.Kind, "name", objKey.String(), "operation", applyResult.Operation)
		}
	}

	updateComponentStatusesFromApplyResults(state.componentStatuses, state.desiredObjects, state.applyResults)

	return firstApplyErr
}

// checkHealthAndCalculateStatus checks K8s and Application level health. [Modified]
func (r *ApplicationDefinitionReconciler) checkHealthAndCalculateStatus(ctx context.Context, state *reconcileState) (allComponentsReady bool, needsRequeue bool, firstCheckErr error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Checking health of applied resources")
	allComponentsReady = true // Assume ready initially
	needsRequeue = false      // Assume no requeue needed initially

	appDef := state.appDef // For convenience

	for compName, compStatus := range state.componentStatuses {
		if appDef.Spec.Components == nil || len(appDef.Spec.Components) == 0 {
			continue
		}
		spec := appDef.Spec.Components[0]
		compStatus.Namespace = appDef.Namespace
		compStatus.ResourceName = compName
		compStatus.APIVersion = spec.APIVersion
		compStatus.Kind = spec.Kind
		compLogger := logger.WithValues("component", compName, "kind", compStatus.Kind, "resourceName", compStatus.ResourceName)

		// Find the corresponding component spec (needed for app health check)
		var appComp *appv1.ApplicationComponent
		for i := range appDef.Spec.Components {
			if appDef.Spec.Components[i].Name == compName {
				appComp = &appDef.Spec.Components[i]
				break
			}
		}
		if appComp == nil {
			compLogger.Error(nil, "Internal error: Cannot find component spec in AppDef corresponding to status entry")
			allComponentsReady = false
			if firstCheckErr == nil {
				firstCheckErr = fmt.Errorf("component spec not found for %s", compName)
			}
			continue // Skip health check for this inconsistent entry
		}

		// Prerequisite checks for health checking
		isInfoMissing := compStatus.ResourceName == "" || compStatus.Kind == "" || compStatus.APIVersion == ""
		isPreviousError := strings.Contains(compStatus.Message, "Error:") || // Generic error marker
			strings.Contains(compStatus.Message, "Failed") || // Common failure keyword
			compStatus.Message == "BuilderStrategyNotFound" || // Specific error messages
			compStatus.Message == "CompDefNotFound" ||
			compStatus.Message == "InvalidCompDefSpec" ||
			compStatus.Message == "ConfigUnmarshalFailed" ||
			compStatus.Message == "BuildObjectsFailed" ||
			compStatus.Message == "InvalidBuiltObject"

		if isInfoMissing || isPreviousError {
			// If resource info is missing or a critical build/apply error occurred, mark as not ready.
			if !isPreviousError && compStatus.Message != "Resource Info Missing" { // Avoid redundant messages
				compStatus.Message = "Resource Info Missing" // Set a clear message
			}
			compStatus.Health = false
			allComponentsReady = false
			compLogger.V(1).Info("Skipping health check due to missing info or prior error", "currentMessage", compStatus.Message)
			continue // Skip actual health checks
		}

		// --- 1. Check K8s Resource Health ---
		k8sHealthy, k8sMessage, k8sCheckErr := kubeutil.CheckHealth(ctx, r.Client, r.Scheme, compStatus.Namespace, compStatus.ResourceName, compStatus.APIVersion, compStatus.Kind)
		if k8sCheckErr != nil {
			// Error during the K8s health check process itself
			compLogger.Error(k8sCheckErr, "Failed to execute K8s resource health check process")
			compStatus.Health = false
			compStatus.Message = fmt.Sprintf("K8sHealthCheckError: %v", k8sCheckErr)
			allComponentsReady = false
			needsRequeue = true // Requeue needed to retry the check
			if firstCheckErr == nil {
				firstCheckErr = k8sCheckErr
			}
			continue // Skip further checks for this component
		}

		if !k8sHealthy {
			// K8s resource is not ready (e.g., Pods not running, STS rollout incomplete)
			compStatus.Health = false
			compStatus.Message = k8sMessage // Use message from kubeutil.CheckHealth
			allComponentsReady = false
			needsRequeue = true // Requeue needed as resource is not ready yet
			compLogger.V(1).Info("K8s resource health check failed", "reason", k8sMessage)
			continue // Skip app-level check if K8s level isn't healthy
		}

		// --- 2. Check Application-Level Health (if K8s resource is healthy) ---
		compLogger.V(1).Info("K8s resource is healthy, proceeding to application-level health check")

		return true, false, firstCheckErr
	}

	return allComponentsReady, needsRequeue, firstCheckErr
}

// determineFinalPhase sets the overall AppDef phase based on errors and readiness.
func (r *ApplicationDefinitionReconciler) determineFinalPhase(state *reconcileState, allComponentsReady bool) {
	currentPhase := state.appDef.Status.Phase

	// If the application is suspended, don't override the Suspended phase
	if currentPhase == appv1.ApplicationPhaseSuspended {
		setCondition(state.appDef, metav1.Condition{Type: string(appv1.ConditionReady), Status: metav1.ConditionFalse, Reason: "Suspended", Message: "Application is intentionally suspended"})
		return
	}

	if state.firstError != nil {
		// If any critical error occurred during the reconcile cycle
		state.appDef.Status.Phase = appv1.ApplicationPhaseFailed
		reason := "ReconcileFailed"
		errMsg := state.firstError.Error()
		// Try to determine a more specific reason based on common error patterns
		if strings.Contains(errMsg, "apply") || strings.Contains(errMsg, "Apply") || strings.Contains(errMsg, "OwnerRef") {
			reason = "ApplyFailed"
		} else if strings.Contains(errMsg, "build") || strings.Contains(errMsg, "Build") || strings.Contains(errMsg, "Config") || strings.Contains(errMsg, "CompDef") || strings.Contains(errMsg, "strategy") || strings.Contains(errMsg, "Strategy") {
			reason = "ProcessingFailed"
		} else if strings.Contains(errMsg, "HealthCheck") || strings.Contains(errMsg, "health check") {
			reason = "HealthCheckFailed" // Covers both K8s and App health check process errors
		}
		setCondition(state.appDef, metav1.Condition{Type: string(appv1.ConditionReady), Status: metav1.ConditionFalse, Reason: reason, Message: errMsg})
		return
	}

	// No critical errors encountered in this cycle
	if allComponentsReady {
		state.appDef.Status.Phase = appv1.ApplicationPhaseRunning
		setCondition(state.appDef, metav1.Condition{Type: string(appv1.ConditionReady), Status: metav1.ConditionTrue, Reason: "ComponentsReady", Message: "All components reconciled and healthy"})
	} else {
		// No errors, but not all components are ready/healthy yet
		reason := "ComponentsNotReady"
		message := "One or more components are not ready or unhealthy"

		// Determine if it's applying or degraded
		if currentPhase == appv1.ApplicationPhaseRunning || currentPhase == appv1.ApplicationPhaseDegraded {
			// Was previously Available or Degraded, now not ready -> Degraded
			state.appDef.Status.Phase = appv1.ApplicationPhaseDegraded
			reason = "ComponentsDegraded"
			message = "One or more previously ready components are now unhealthy or not ready"
		} else {
			// Still in initial phases (Processing, Applying) or recovering from Failed
			state.appDef.Status.Phase = appv1.ApplicationPhaseUpdateing // Or keep Processing if appropriate? Applying seems better.
			reason = "ComponentsApplying"
			message = "Waiting for components to become ready and healthy"
		}
		setCondition(state.appDef, metav1.Condition{Type: string(appv1.ConditionReady), Status: metav1.ConditionFalse, Reason: reason, Message: message})
	}
}

// handleReconcileError updates status for critical errors and returns.
func (r *ApplicationDefinitionReconciler) handleReconcileError(ctx context.Context, state *reconcileState, reason string, err error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Error(err, "Reconciliation failed critically", "reason", reason)

	state.appDef.Status.Phase = appv1.ApplicationPhaseFailed
	setCondition(state.appDef, metav1.Condition{Type: string(appv1.ConditionReady), Status: metav1.ConditionFalse, Reason: reason, Message: err.Error()})

	// Update individual component statuses to reflect the failure if possible
	r.updateComponentStatusesForError(state, err)
	state.appDef.Status.Components = mapToSliceComponentStatus(state.componentStatuses)

	// Attempt to update status, but return the original error regardless
	_, updateErr := r.updateStatusIfNeeded(ctx, state.appDef, state.originalStatus)
	if updateErr != nil {
		logger.Error(updateErr, "Failed to update status during error handling for critical failure")
		// Log the status update error, but prioritize returning the original error
	}

	// Store the critical error if not already set
	if state.firstError == nil {
		state.firstError = err
	}
	// Return the original critical error to controller-runtime for potential backoff
	return ctrl.Result{}, state.firstError
}

// updateStatusIfNeeded compares current and original status and updates if necessary.
func (r *ApplicationDefinitionReconciler) updateStatusIfNeeded(ctx context.Context, currentApp *appv1.ApplicationDefinition, originalStatus *appv1.ApplicationDefinitionStatus) (bool, error) {
	logger := log.FromContext(ctx)
	// Ensure Generation matches ObservedGeneration after successful reconcile
	currentApp.Status.ObservedGeneration = currentApp.Generation

	// Compare relevant fields for changes
	if currentApp.Status.Phase == originalStatus.Phase &&
		conditionsEqual(currentApp.Status.Conditions, originalStatus.Conditions) &&
		componentStatusesEqual(currentApp.Status.Components, originalStatus.Components) &&
		currentApp.Status.ObservedGeneration == originalStatus.ObservedGeneration &&
		currentApp.Status.LastChangeID == originalStatus.LastChangeID {
		logger.V(1).Info("Status unchanged, skipping update.")
		return false, nil // No changes detected
	}

	// Status has changed, attempt update with retry
	logger.V(1).Info("Status has changed, attempting update.", "newPhase", currentApp.Status.Phase)

	// Create a copy of the desired status to reapply if needed
	desiredStatus := currentApp.Status.DeepCopy()

	for i := 0; i < 3; i++ {
		if err := r.Client.Status().Update(ctx, currentApp); err != nil {
			if apierrors.IsConflict(err) {
				logger.Info("Status update conflict detected, retrying...", "attempt", i+1, "error", err.Error())
				// Re-fetch the latest version
				if fetchErr := r.Client.Get(ctx, client.ObjectKeyFromObject(currentApp), currentApp); fetchErr != nil {
					logger.Error(fetchErr, "Failed to re-fetch appDef after status update conflict")
					return false, fetchErr
				}
				// Reapply the desired status changes since currentApp was re-fetched
				currentApp.Status = *desiredStatus.DeepCopy()
				// Update ObservedGeneration again since we're overwriting the status
				currentApp.Status.ObservedGeneration = currentApp.Generation
				continue
			}
			logger.Error(err, "Failed to update ApplicationDefinition status")
			return false, err // Return other status update errors
		}
		logger.V(1).Info("ApplicationDefinition status updated successfully")
		return true, nil // Status updated successfully
	}
	// If we get here, all retries failed
	return false, fmt.Errorf("failed to update status after 3 attempts")
}

// --- Status Comparison Helpers ---

func conditionsEqual(c1, c2 []metav1.Condition) bool {
	if len(c1) != len(c2) {
		return false
	}
	// Convert to map for easier comparison regardless of order
	map1 := make(map[string]metav1.Condition)
	for _, c := range c1 {
		map1[c.Type] = c
	}
	for _, c := range c2 {
		if existing, ok := map1[c.Type]; !ok ||
			existing.Status != c.Status ||
			existing.Reason != c.Reason ||
			existing.Message != c.Message {
			// Note: LastTransitionTime is ignored for equality comparison
			return false
		}
	}
	return true
}

func componentStatusesEqual(s1, s2 []appv1.ComponentStatusReference) bool {
	if len(s1) != len(s2) {
		return false
	}
	map1 := make(map[string]appv1.ComponentStatusReference)
	for _, s := range s1 {
		map1[s.Name] = s
	}
	for _, s := range s2 {
		if existing, ok := map1[s.Name]; !ok ||
			existing.Kind != s.Kind ||
			existing.APIVersion != s.APIVersion ||
			existing.ResourceName != s.ResourceName ||
			existing.Namespace != s.Namespace ||
			existing.Health != s.Health ||
			existing.Message != s.Message {
			return false
		}
	}
	return true
}

// --- Status Update Helpers ---

func setCondition(appDef *appv1.ApplicationDefinition, newCondition metav1.Condition) {
	// Use meta.SetStatusCondition to correctly handle updates and transitions.
	// It adds or updates the condition based on Type, setting LastTransitionTime.
	meta.SetStatusCondition(&appDef.Status.Conditions, newCondition)
}

func mapToSliceComponentStatus(statusMap map[string]*appv1.ComponentStatusReference) []appv1.ComponentStatusReference {
	statusSlice := make([]appv1.ComponentStatusReference, 0, len(statusMap))
	for _, statusPtr := range statusMap {
		if statusPtr != nil {
			statusSlice = append(statusSlice, *statusPtr)
		}
	}
	// Sort by name for consistent status output
	sort.Slice(statusSlice, func(i, j int) bool { return statusSlice[i].Name < statusSlice[j].Name })
	return statusSlice
}

// updateComponentStatusWithError sets the component's status message and health upon encountering a specific error.
func (r *ApplicationDefinitionReconciler) updateComponentStatusWithError(status *appv1.ComponentStatusReference, reason, errMsg string) {
	if status == nil {
		return // Should not happen if initialized correctly
	}
	status.Health = false
	status.Message = fmt.Sprintf("%s: %s", reason, errMsg) // Prefix message with reason
}

// updateComponentStatusForApplyError updates the relevant component status when an apply fails for one of its objects.
func (r *ApplicationDefinitionReconciler) updateComponentStatusForApplyError(statusMap map[string]*appv1.ComponentStatusReference, failedObj client.Object, applyErr error) {
	compNameLabel := failedObj.GetLabels()[compInstanceLabel]
	if compNameLabel == "" {
		return // Cannot map object to component
	}
	if compStatus, ok := statusMap[compNameLabel]; ok {
		// Only update if the current message isn't already reflecting a more critical prior error
		if !strings.Contains(compStatus.Message, "Error:") && !strings.Contains(compStatus.Message, "Failed") {
			gvk := failedObj.GetObjectKind().GroupVersionKind()
			objKey := client.ObjectKeyFromObject(failedObj)
			errMsg := fmt.Sprintf("ApplyError: Failed for %s %s: %v", gvk.Kind, objKey.String(), applyErr)
			r.updateComponentStatusWithError(compStatus, "ApplyFailed", errMsg)
		}
	}
}

// updateComponentStatusesFromApplyResults updates component statuses based *only* on the apply result.
// Health checks happen later.
func updateComponentStatusesFromApplyResults(statusMap map[string]*appv1.ComponentStatusReference, desiredObjectList []client.Object, applyResults map[string]kubeutil.ApplyResult) {
	logger := log.Log.WithName("status-updater") // Use a generic logger name

	// Map to track if *any* object belonging to a component failed to apply
	componentApplyFailed := make(map[string]bool)

	for _, obj := range desiredObjectList {
		compNameLabel := obj.GetLabels()[compInstanceLabel]
		if compNameLabel == "" {
			continue
		}

		resultMapKey := kubeutil.BuildObjectResultMapKey(obj)
		result, applyOk := applyResults[resultMapKey]

		if !applyOk || result.Error != nil {
			componentApplyFailed[compNameLabel] = true // Mark component as having at least one apply failure
			// Detailed error message is set by updateComponentStatusForApplyError
		}
	}

	// Now update the status message for components without apply errors
	for compName, compStatus := range statusMap {
		if _, failed := componentApplyFailed[compName]; !failed {
			// No apply errors for this component's objects.
			// Update message only if it's still in an initial state.
			if compStatus.Message == "Initializing" || compStatus.Message == "Processing" || compStatus.Message == "Built successfully" {
				compStatus.Message = "Applied successfully, awaiting health check"
				logger.V(1).Info("Updated component status after successful apply", "component", compName, "message", compStatus.Message)
			}
		}
		// If componentApplyFailed[compName] is true, the message was already set by updateComponentStatusForApplyError
	}
}

// updateComponentStatusesForError sets an error message on all component statuses.
func (r *ApplicationDefinitionReconciler) updateComponentStatusesForError(state *reconcileState, err error) {
	errMsg := err.Error()
	for _, compStatus := range state.componentStatuses {
		// Avoid overwriting specific errors with a generic one unless the current status is non-terminal
		if !strings.Contains(compStatus.Message, "Error:") && !strings.Contains(compStatus.Message, "Failed") {
			compStatus.Health = false
			compStatus.Message = fmt.Sprintf("OverallReconcileError: %s", errMsg)
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("applicationdefinition-controller")
	r.RESTMapper = mgr.GetRESTMapper() // Get RESTMapper from manager

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&appv1.ApplicationDefinition{})

	// Define owned types explicitly for better clarity and control
	ownedTypes := []client.Object{
		&appsv1.Deployment{},
		&appsv1.StatefulSet{},
		// &appsv1.DaemonSet{}, // Add if managed
		&corev1.Service{},
		&corev1.PersistentVolumeClaim{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&corev1.ServiceAccount{},
		// &policyv1.PodDisruptionBudget{},
		// &networkingv1.Ingress{}, // Add if managed
	}

	if commonutil.IsV1Supported {
		ownedTypes = append(ownedTypes, &policyv1.PodDisruptionBudget{}) // 使用 policy/v1
	} else {
		ownedTypes = append(ownedTypes, &policyv1beta1.PodDisruptionBudget{}) // 回退到 policy/v1beta1
	}

	for _, t := range ownedTypes {
		builder = builder.Owns(t)
	}

	return builder.Complete(r)
}

// getEventRecorder returns the appropriate event recorder based on ApplicationDefinition annotations.
// If webhook annotations are present, it returns a WebhookEventRecorder; otherwise, returns the standard recorder.
func (r *ApplicationDefinitionReconciler) getEventRecorder(app *appv1.ApplicationDefinition) record.EventRecorder {
	if app == nil || app.Annotations == nil {
		return r.Recorder
	}

	changeID := app.Annotations[appv1.AnnotationChangeID]
	clusterID := app.Annotations[appv1.AnnotationClusterID]
	webhookURL := app.Annotations[appv1.AnnotationChangeWebhookURL]

	if webhookURL == "" || changeID == "" {
		return r.Recorder
	}

	return webrecorder.NewWebhookEventRecorder(webhookURL, changeID, clusterID)
}

// recordEvent is a helper method to record events with optional webhook support
func (r *ApplicationDefinitionReconciler) recordEvent(app *appv1.ApplicationDefinition, phase, status, step, eventType, reason, message string) {
	recorder := r.getEventRecorder(app)

	if wr, ok := recorder.(*webrecorder.WebhookEventRecorder); ok {
		// Get current change ID from annotations
		currentChangeID := app.Annotations[appv1.AnnotationChangeID]

		// Check if this change ID has already been processed
		if app.Status.LastChangeID == currentChangeID {
			// Skip sending duplicate webhook event for the same change ID
			return
		}
		annotations := map[string]string{
			webrecorder.PhaseKey:  phase,
			webrecorder.StatusKey: status,
			webrecorder.StepKey:   step,
		}
		wr.AnnotatedEventf(app, annotations, eventType, reason, message)
	} else {
		recorder.Event(app, eventType, reason, message)
	}
}

// recordEventf is a helper method to record formatted events with optional webhook support
func (r *ApplicationDefinitionReconciler) recordEventf(app *appv1.ApplicationDefinition, phase, status, step, eventType, reason, messageFmt string, args ...interface{}) {
	recorder := r.getEventRecorder(app)

	if wr, ok := recorder.(*webrecorder.WebhookEventRecorder); ok {
		// Get current change ID from annotations
		currentChangeID := app.Annotations[appv1.AnnotationChangeID]

		// Check if this change ID has already been processed
		if app.Status.LastChangeID == currentChangeID {
			// Skip sending duplicate webhook event for the same change ID
			return
		}
		annotations := map[string]string{
			webrecorder.PhaseKey:  phase,
			webrecorder.StatusKey: status,
			webrecorder.StepKey:   step,
		}
		wr.AnnotatedEventf(app, annotations, eventType, reason, messageFmt, args...)
	} else {
		recorder.Eventf(app, eventType, reason, messageFmt, args...)
	}
}
