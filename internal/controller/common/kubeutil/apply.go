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

// internal/controller/common/kubeutil/apply.go
// Package kubeutil provides utility functions for interacting with Kubernetes resources.
package kubeutil

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil" // For OperationResult
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ApplyResult contains the result of an apply operation (SSA).
// Note: With Server-Side Apply, accurately determining Created vs Updated vs Unchanged
// often requires comparing the object before and after, which adds complexity.
// OperationResultNone is often returned for successful SSA calls if no mutations occurred or detection is hard.
type ApplyResult struct {
	// Operation indicates the result (Created, Updated, Configured, None, Deleted - less relevant for Apply).
	// Using OperationResultNone for SSA success unless K8s version/libraries provide more detail.
	Operation controllerutil.OperationResult
	// Error holds any error that occurred during the apply operation.
	Error error
}

// ApplyObject idempotently applies the desired state of a Kubernetes object
// using Server-Side Apply. It creates the object if it doesn't exist, or
// patches it based on the desired state, managing fields via the specified fieldManager.
// The input 'obj' should be a *pointer* to a valid client.Object (e.g., &appsv1.Deployment{}).
// It should contain the complete desired state *with* Kind, APIVersion, Name, Namespace set.
func ApplyObject(ctx context.Context, k8sClient client.Client, obj client.Object, fieldManager string) ApplyResult {
	// Ensure essential fields for Apply are present (Defensive check)
	gvk := obj.GetObjectKind().GroupVersionKind()
	objKey := client.ObjectKeyFromObject(obj)

	// 内置资源没有 Group
	if gvk.Kind == "" || gvk.Version == "" || objKey.Name == "" || objKey.Namespace == "" {
		// Log a critical error if basic info is missing for apply.
		err := fmt.Errorf("object is missing essential GVK or Name/Namespace for apply (GVK: %s, NsName: %s)", gvk.String(), objKey.String())
		log.FromContext(ctx).Error(err, "Cannot apply object without complete metadata")
		return ApplyResult{Error: err}
	}

	logger := log.FromContext(ctx).WithValues(
		"kind", gvk.Kind,
		"version", gvk.Version,
		"name", objKey.Name,
		"namespace", objKey.Namespace,
		"fieldManager", fieldManager,
	)
	logger.V(1).Info("Attempting to apply object using Server-Side Apply")

	// Prepare the patch data using client.Apply.
	// Pass the object itself which contains the desired state.
	patchData := client.Apply // This signifies the patch *content* implies Apply strategy

	// Prepare the patch options.
	// FieldOwner: Identifies who is managing the fields in this object. Should be unique (e.g., your operator name).
	// Force: Needed to acquire ownership of fields currently managed by others (e.g., if a Helm chart or manual edit previously set fields).
	patchOpts := []client.PatchOption{
		client.FieldOwner(fieldManager),
		client.ForceOwnership,
	}

	// Call the Patch API method with the object and apply options.
	// This is the core Server-Side Apply call.
	err := k8sClient.Patch(ctx, obj, patchData, patchOpts...)
	if err != nil {
		// Wrap the error for more context if desired
		// Example: fmt.Errorf("failed to apply %s %s/%s: %w", gvk.Kind, objKey.Namespace, objKey.Name, err)
		// Or return original error
		logger.V(1).Error(err, "Patch call failed")
		return ApplyResult{Error: err} // Return the API error
	}

	logger.V(1).Info("Patch call succeeded")

	// --- Determine Operation Result (Optional and Tricky with SSA) ---
	// K8s Server-Side Apply API call itself doesn't clearly return Created vs Updated vs Unchanged.
	// It might set f:managedFields on the object upon return.
	// To accurately determine: Get object BEFORE patch, GET object AFTER patch, compare spec/metadata/resourceVersion.
	// This adds API calls and complexity.
	// A simpler approach is to return OperationResultNone for success, and let health checks verify result.
	// For this framework, let's indicate success with None.
	// If the K8s libraries provided better results for SSA in the future, we could leverage that.

	return ApplyResult{Operation: controllerutil.OperationResultNone, Error: nil}
}

// BuildObjectResultMapKey creates a unique string key for the applyResults map.
// It uses the object's GVK string, Namespace, and Name.
func BuildObjectResultMapKey(obj client.Object) string {
	if obj == nil {
		return ""
	}
	gvk := obj.GetObjectKind().GroupVersionKind()
	objKey := client.ObjectKeyFromObject(obj)
	// Use the full GVK string from the GVK struct (includes Group, Version, Kind)
	return gvk.String() + "/" + objKey.String()
}
