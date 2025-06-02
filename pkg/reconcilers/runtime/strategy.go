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

package runtime

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	k8sbuilders "github.com/infinilabs/runtime-operator/pkg/builders/k8s"

	appv1 "github.com/infinilabs/runtime-operator/api/app/v1"
	"github.com/infinilabs/runtime-operator/internal/controller/common/kubeutil"
	"github.com/infinilabs/runtime-operator/pkg/apis/common"
	commonreconcilers "github.com/infinilabs/runtime-operator/pkg/reconcilers/common"
	"github.com/infinilabs/runtime-operator/pkg/strategy"

	"k8s.io/client-go/tools/record"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Ensure implementation complies
var _ strategy.AppReconcileStrategy = &ReconcileStrategy{}

// ReconcileStrategy orchestrates the reconciliation flow for the "gateway" application type.
type ReconcileStrategy struct{}

// Register the strategy
func init() {
	strategy.RegisterAppReconcileStrategy("gateway", &ReconcileStrategy{})
}

// Reconcile implements AppReconcileStrategy interface.
// Defines the sequence of tasks to run for a Gateway component instance.
// It utilizes the common Task Runner.
func (s *ReconcileStrategy) Reconcile(
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	appDef *appv1.ApplicationDefinition,
	appComp *appv1.ApplicationComponent,
	componentStatus *appv1.ComponentStatusReference,
	mergedConfig interface{},
	desiredObjects []client.Object,
	applyResults map[string]kubeutil.ApplyResult,
	recorder record.EventRecorder,
) (bool, error) {
	logger := log.FromContext(ctx).WithValues("component", componentStatus.Name, "reconcileStrategy", "Gateway")

	// --- Define the list of tasks for Gateway reconciliation workflow ---
	taskList := []commonreconcilers.Task{
		commonreconcilers.NewCheckK8sHealthTask(),
	}

	// --- Run tasks using Task Runner ---
	taskRunner := commonreconcilers.NewTaskRunner(k8sClient, scheme, recorder)
	overallResult, runErr := taskRunner.RunTasks(
		ctx,
		appDef,
		appComp,
		componentStatus,
		mergedConfig,
		buildObjectMap(desiredObjects),
		applyResults,
		taskList,
	)

	// --- Handle overall result and error ---
	if runErr != nil {
		logger.Error(runErr, "Gateway reconciliation task execution failed")
		return false, runErr // Signal overall reconcile failure
	}

	if overallResult == commonreconcilers.TaskResultPending {
		logger.V(1).Info("Gateway reconciliation task execution pending")
		return true, nil // Signal needs requeue
	}

	logger.V(1).Info("Gateway reconciliation task execution complete")
	return false, nil
}

// CheckAppHealth implements AppReconcileStrategy interface for Gateway.
// Performs application-level health check for Gateway.
func (s *ReconcileStrategy) CheckAppHealth(ctx context.Context, k8sClient client.Client,
	scheme *runtime.Scheme, appDef *appv1.ApplicationDefinition,
	appComp *appv1.ApplicationComponent, appSpecificConfig interface{}) (bool, string, error) {
	logger := log.FromContext(ctx).WithValues("component", appComp.Name, "type", appComp.Type)
	logger.V(1).Info("Executing Gateway application health check (Strategy CheckAppHealth method)")

	// Type assert the specific config
	gatewayConfig, ok := appSpecificConfig.(*common.RuntimeConfig)
	if !ok || gatewayConfig == nil {
		return false, "Invalid or missing Gateway config for health check", fmt.Errorf("invalid config type %T", appSpecificConfig)
	}

	// 1. Find the primary client service name (convention or config)
	instanceName := appComp.Name
	serviceName := k8sbuilders.DeriveResourceName(instanceName) // Assume service name matches instance name
	namespace := appDef.Namespace

	// 2. Get the service object
	svc := &corev1.Service{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: serviceName}, svc); err != nil {
		if apierrors.IsNotFound(err) { // Use apierrors alias
			return false, fmt.Sprintf("Client service %s not found", serviceName), nil // Not healthy if service missing
		}
		return false, fmt.Sprintf("Failed to get client service %s: %v", serviceName, err), err
	}

	// 3. Check if it's headless (if so, app health might rely on other checks)
	if svc.Spec.ClusterIP == corev1.ClusterIPNone {
		// Maybe return true here, assuming basic K8s readiness passed earlier?
		// Or perform a check specific to headless discovery.
		return true, "Headless service found (basic check)", nil
	}

	// 4. For non-headless, check endpoints
	endpoints := &corev1.Endpoints{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: serviceName}, endpoints); err != nil {
		if apierrors.IsNotFound(err) { // Use apierrors alias
			return false, "Gateway service endpoints not found", nil // No endpoints means no healthy pods serving
		}
		return false, fmt.Sprintf("Failed to get Gateway endpoints %s: %v", serviceName, err), err
	}

	// Check if endpoints has ready addresses in subsets
	hasReadyEndpoints := false
	readyCount := 0
	totalCount := 0
	if endpoints.Subsets != nil {
		for _, subset := range endpoints.Subsets {
			if subset.Addresses != nil {
				readyCount += len(subset.Addresses)
				hasReadyEndpoints = hasReadyEndpoints || len(subset.Addresses) > 0
			}
			if subset.NotReadyAddresses != nil {
				totalCount += len(subset.NotReadyAddresses)
			}
		}
		totalCount += readyCount
	}

	if !hasReadyEndpoints {
		return false, fmt.Sprintf("No ready endpoints found for Gateway service (%d/%d ready/total)", readyCount, totalCount), nil
	}

	return true, fmt.Sprintf("Gateway application service has ready endpoints (%d/%d ready/total)", readyCount, totalCount), nil
}

// Helper to build map from object slice (duplicate from OS strategy, move to common util?)
func buildObjectMap(objList []client.Object) map[string]client.Object {
	objMap := make(map[string]client.Object, len(objList))
	for _, obj := range objList {
		gvk := obj.GetObjectKind().GroupVersionKind()
		objKey := client.ObjectKeyFromObject(obj)
		resultMapKey := gvk.String() + "/" + objKey.String() // Use consistent key
		objMap[resultMapKey] = obj
	}
	return objMap
}
