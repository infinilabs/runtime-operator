package runtime

import (
	"context"

	appv1 "github.com/infinilabs/runtime-operator/api/app/v1"
	"github.com/infinilabs/runtime-operator/internal/controller/common/kubeutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AppBuilderStrategy defines the contract for application-specific logic during object building.
type AppBuilderStrategy interface {
	// BuildObjects builds K8s objects for a specific component instance.
	BuildObjects(
		ctx context.Context,
		k8sClient client.Client,
		scheme *runtime.Scheme,
		owner client.Object, // Owning AppDef
		appDef *appv1.ApplicationDefinition, // Full AppDef
		appComp *appv1.ApplicationComponent, // Component being processed
		appSpecificConfig interface{}, // Unmarshalled specific config
	) ([]client.Object, error)

	// GetWorkloadGVK returns the expected primary K8s workload GVK managed by this strategy.
	GetWorkloadGVK() schema.GroupVersionKind
}

// AppReconcileStrategy defines the contract for orchestrating reconciliation tasks and health checks.
type AppReconcileStrategy interface {
	// Reconcile orchestrates post-build reconciliation tasks.
	Reconcile(
		ctx context.Context,
		k8sClient client.Client,
		scheme *runtime.Scheme,
		appDef *appv1.ApplicationDefinition,
		appComp *appv1.ApplicationComponent,
		componentStatus *appv1.ComponentStatusReference, // Mutable status
		mergedConfig interface{}, // Unmarshalled specific config
		desiredObjects []client.Object, // Built objects (consider passing map?)
		applyResults map[string]kubeutil.ApplyResult, // Results from apply phase
		recorder record.EventRecorder,
	) (needsRequeue bool, err error)

	// CheckAppHealth performs application-level health checks.
	CheckAppHealth(
		ctx context.Context,
		k8sClient client.Client,
		scheme *runtime.Scheme,
		appDef *appv1.ApplicationDefinition,
		appComp *appv1.ApplicationComponent,
		appSpecificConfig interface{}, // Unmarshalled specific config
	) (isHealthy bool, message string, err error)
}
