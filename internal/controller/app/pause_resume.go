package app

import (
	"context"

	appv1 "github.com/infinilabs/runtime-operator/api/app/v1"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// handlePauseResume handles the logic for suspending and resuming the application.
func (r *ApplicationDefinitionReconciler) handlePauseResume(ctx context.Context, state *reconcileState) error {
	logger := log.FromContext(ctx)
	appDef := state.appDef
	isSuspended := appDef.Spec.Suspend != nil && *appDef.Spec.Suspend

	// Initialize SuspendedReplicas map if nil
	if appDef.Status.SuspendedReplicas == nil {
		appDef.Status.SuspendedReplicas = make(map[string]int32)
	}

	for _, obj := range state.desiredObjects {
		// We only care about Deployments and StatefulSets for scaling
		var currentReplicas *int32
		var objName string
		var objKind string

		switch o := obj.(type) {
		case *appsv1.Deployment:
			currentReplicas = o.Spec.Replicas
			objName = o.Name
			objKind = "Deployment"
		case *appsv1.StatefulSet:
			currentReplicas = o.Spec.Replicas
			objName = o.Name
			objKind = "StatefulSet"
		default:
			continue // Skip other objects
		}

		// Find the component name from labels
		compName := obj.GetLabels()[compInstanceLabel]
		if compName == "" {
			continue
		}

		if isSuspended {
			// --- SUSPEND LOGIC ---
			// Check if we already have a recorded replica count
			if _, ok := appDef.Status.SuspendedReplicas[compName]; !ok {
				replicasToRecord := int32(1) // Default
				if currentReplicas != nil {
					replicasToRecord = *currentReplicas
				}
				appDef.Status.SuspendedReplicas[compName] = replicasToRecord
				logger.V(1).Info("Recording replicas for suspend", "component", compName, "resource", objName, "replicas", replicasToRecord)
			}

			// Set replicas to 0
			zero := int32(0)
			switch o := obj.(type) {
			case *appsv1.Deployment:
				o.Spec.Replicas = &zero
			case *appsv1.StatefulSet:
				o.Spec.Replicas = &zero
			}
			logger.V(1).Info("Suspending component", "component", compName, "resource", objName, "kind", objKind)

		} else {
			// --- RESUME LOGIC ---
			// Check if we have a recorded replica count
			if recordedReplicas, ok := appDef.Status.SuspendedReplicas[compName]; ok {
				// Restore the recorded value
				switch o := obj.(type) {
				case *appsv1.Deployment:
					o.Spec.Replicas = &recordedReplicas
				case *appsv1.StatefulSet:
					o.Spec.Replicas = &recordedReplicas
				}
				logger.Info("Resuming component", "component", compName, "resource", objName, "kind", objKind, "replicas", recordedReplicas)

				// Remove the entry from the map so that future reconciles respect the Spec.
				delete(appDef.Status.SuspendedReplicas, compName)
			}
		}
	}

	// Set the phase to Suspended if the application is suspended
	// Clear the Suspended phase if the application is being resumed
	if isSuspended {
		state.appDef.Status.Phase = appv1.ApplicationPhaseSuspended
	} else if state.appDef.Status.Phase == appv1.ApplicationPhaseSuspended {
		// Application is being resumed, clear the Suspended phase
		// Set to Updating so normal reconciliation can determine the final phase
		state.appDef.Status.Phase = appv1.ApplicationPhaseUpdateing
	}

	return nil
}
