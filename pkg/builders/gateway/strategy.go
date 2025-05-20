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

// pkg/strategy/strategy.go
// Defines generic interfaces and registry for application build and reconcile strategies.
package gateway

import (
	"context"
	// Import types used ONLY in the interface definitions
	appv1 "github.com/infinilabs/operator/api/app/v1" // For ApplicationComponent etc. in signatures
	"github.com/infinilabs/operator/internal/controller/common/kubeutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema" // For GVK
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// --- Builder Strategy ---

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
