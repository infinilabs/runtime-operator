package gateway

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	k8s_builders "github.com/infinilabs/runtime-operator/pkg/builders/k8s"

	appv1 "github.com/infinilabs/runtime-operator/api/app/v1"
	"github.com/infinilabs/runtime-operator/internal/controller/common/kubeutil"
	"github.com/infinilabs/runtime-operator/pkg/apis/common"
	common_reconcilers "github.com/infinilabs/runtime-operator/pkg/reconcilers/common"
	"github.com/infinilabs/runtime-operator/pkg/strategy"

	"k8s.io/client-go/tools/record"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Ensure implementation complies
var _ strategy.AppReconcileStrategy = &GatewayReconcileStrategy{}

// GatewayReconcileStrategy orchestrates the reconciliation flow for the "gateway" application type.
type GatewayReconcileStrategy struct{}

// Register the strategy
func init() {
	strategy.RegisterAppReconcileStrategy("gateway", &GatewayReconcileStrategy{})
}

// Reconcile implements AppReconcileStrategy interface.
// Defines the sequence of tasks to run for a Gateway component instance.
// It utilizes the common Task Runner.
func (s *GatewayReconcileStrategy) Reconcile(
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	appDef *appv1.ApplicationDefinition,
	appComp *appv1.ApplicationComponent,             // <--- 这个参数在函数签名中已经有了
	componentStatus *appv1.ComponentStatusReference, // <--- 这个参数在函数签名中已经有了
	mergedConfig interface{},                        // <--- 这个参数在函数签名中已经有了
	desiredObjects []client.Object,                  // <--- 原始的 desiredObjects 列表
	applyResults map[string]kubeutil.ApplyResult,    // <--- 这个参数在函数签名中已经有了
	recorder record.EventRecorder,
) (bool, error) { // Returns needsRequeue, error
	logger := log.FromContext(ctx).WithValues("component", componentStatus.Name, "reconcileStrategy", "Gateway")

	// --- Define the list of tasks for Gateway reconciliation workflow ---
	taskList := []common_reconcilers.Task{
		common_reconcilers.NewCheckK8sHealthTask(),
		// common_reconcilers.NewCheckServiceReadyTask(), // Example
	}

	// --- Task Context is prepared INTERNALLY by TaskRunner or passed to individual tasks ---
	// --- You DON'T pass the taskContext struct directly to RunTasks ---

	// --- Run tasks using Task Runner ---
	taskRunner := common_reconcilers.NewTaskRunner(k8sClient, scheme, recorder)

	// --- 正确的调用方式 ---
	// 将 Reconcile 函数接收到的参数直接传递给 RunTasks
	overallResult, runErr := taskRunner.RunTasks(
		ctx,
		appDef,                         // owner (client.Object)
		appComp,                        // *appv1.ApplicationComponent
		componentStatus,                // *appv1.ComponentStatusReference
		mergedConfig,                   // interface{}
		buildObjectMap(desiredObjects), // map[string]client.Object (需要转换)
		applyResults,                   // map[string]kubeutil.ApplyResult
		taskList,                       // []common_reconcilers.Task
	)

	// --- Handle overall result and error ---
	if runErr != nil {
		logger.Error(runErr, "Gateway reconciliation task execution failed")
		return false, runErr // Signal overall reconcile failure
	}

	if overallResult == common_reconcilers.TaskResultPending {
		logger.V(1).Info("Gateway reconciliation task execution pending")
		return true, nil // Signal needs requeue
	}

	logger.V(1).Info("Gateway reconciliation task execution complete")
	return false, nil // Signal completion
}

// CheckAppHealth implements AppReconcileStrategy interface for Gateway.
// Performs application-level health check for Gateway.
func (s *GatewayReconcileStrategy) CheckAppHealth(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme, appDef *appv1.ApplicationDefinition, appComp *appv1.ApplicationComponent, appSpecificConfig interface{}) (bool, string, error) {
	logger := log.FromContext(ctx).WithValues("component", appComp.Name, "type", appComp.Type)
	logger.V(1).Info("Executing Gateway application health check (Strategy CheckAppHealth method)")

	// Type assert the specific config
	gatewayConfig, ok := appSpecificConfig.(*common.ResourceConfig)
	if !ok || gatewayConfig == nil {
		return false, "Invalid or missing Gateway config for health check", fmt.Errorf("invalid config type %T", appSpecificConfig)
	}

	// --- Implement Gateway Specific Health Check Logic ---
	// Example: Check Client Service Endpoints as a basic app-level check

	// 1. Find the primary client service name (convention or config)
	instanceName := appComp.Name
	serviceName := k8s_builders.DeriveResourceName(instanceName) // Assume service name matches instance name
	namespace := appDef.Namespace

	// 2. Get the service object
	svc := &corev1.Service{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: serviceName}, svc); err != nil {
		if apierrors.IsNotFound(err) { // Use apierrors alias
			return false, fmt.Sprintf("Client service %s not found", serviceName), nil // Not healthy if service missing
		}
		// Error during the check process itself
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

	// --- Add deeper HTTP check if needed ---
	// clusterIP := svc.Spec.ClusterIP
	// httpPort := findHttpPort(...) // Find relevant port
	// healthURL := fmt.Sprintf("http://%s:%d/healthz", clusterIP, httpPort)
	// Perform http.Get(healthURL) ...

	// If only endpoint check is done:
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
