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

package app

import (
	"context"
	"fmt"
	"time"

	"github.com/cisco-open/operator-tools/pkg/reconciler"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1 "github.com/infinilabs/runtime-operator/api/app/v1"
	_ "github.com/infinilabs/runtime-operator/pkg/builders/runtime"
)

var _ = Describe("ApplicationDefinition Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		applicationdefinition := &appv1.ApplicationDefinition{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ApplicationDefinition")
			// Ensure clean state
			err := k8sClient.Get(ctx, typeNamespacedName, applicationdefinition)
			if err == nil {
				// Object exists, delete it
				// Force remove finalizers to ensure it goes away
				applicationdefinition.SetFinalizers(nil)
				_ = k8sClient.Update(ctx, applicationdefinition)
				_ = k8sClient.Delete(ctx, applicationdefinition)

				// Wait for it to be gone
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, typeNamespacedName, applicationdefinition))
				}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
			}

			resource := &appv1.ApplicationDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: appv1.ApplicationDefinitionSpec{
					Components: []appv1.ApplicationComponent{
						{
							Name:       "test-comp",
							Kind:       "StatefulSet",
							APIVersion: "apps/v1",
							Type:       "operator",
							Properties: runtime.RawExtension{Raw: []byte(`{"image":{"repository":"nginx","tag":"latest"},"replicas":3,"storage":{"enabled":true,"size":"1Gi","mountPath":"/data"},"ports":[{"containerPort":80,"name":"http"}]}`)},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &appv1.ApplicationDefinition{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ApplicationDefinition")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ApplicationDefinitionReconciler{
				Client:     k8sClient,
				Scheme:     k8sClient.Scheme(),
				Recorder:   record.NewFakeRecorder(10),
				Reconciler: reconciler.NewReconcilerWith(k8sClient),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should suspend and resume the application", func() {
			controllerReconciler := &ApplicationDefinitionReconciler{
				Client:     k8sClient,
				Scheme:     k8sClient.Scheme(),
				Recorder:   record.NewFakeRecorder(10),
				Reconciler: reconciler.NewReconcilerWith(k8sClient),
			}

			// 1. Initial Reconcile (Create)
			By("Initial reconcile")

			// 1. Initial Reconcile (Create)
			By("Initial reconcile")

			// Debug: Check object state
			debugAppDef := &appv1.ApplicationDefinition{}
			if err := k8sClient.Get(ctx, typeNamespacedName, debugAppDef); err == nil {
				fmt.Printf("DEBUG: AppDef before reconcile: UID=%s, DeletionTimestamp=%v\n", debugAppDef.UID, debugAppDef.DeletionTimestamp)
			} else {
				fmt.Printf("DEBUG: Failed to get AppDef before reconcile: %v\n", err)
			}

			// Loop reconcile until no requeue
			for i := 0; i < 5; i++ {
				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())
				if !result.Requeue && result.RequeueAfter == 0 {
					break
				}
			}

			// Verify StatefulSet created with 3 replicas
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				// List all StatefulSets to debug
				stsList := &appsv1.StatefulSetList{}
				if err := k8sClient.List(ctx, stsList, client.InNamespace("default")); err == nil {
					fmt.Printf("Found %d StatefulSets in default namespace:\n", len(stsList.Items))
					for _, s := range stsList.Items {
						fmt.Printf(" - %s\n", s.Name)
					}
				} else {
					fmt.Printf("Failed to list StatefulSets: %v\n", err)
				}

				// The resource name for StatefulSet is usually the component name + suffix or just component name.
				// In builder.go: resourceName := builders.DeriveResourceName(instanceName)
				// DeriveResourceName usually returns the name as is or sanitized.
				// Let's try "test-comp".
				return k8sClient.Get(ctx, types.NamespacedName{Name: "test-comp", Namespace: "default"}, sts)
			}, time.Second*10, time.Millisecond*500).Should(Succeed())
			Expect(*sts.Spec.Replicas).To(Equal(int32(3)))

			// 2. Suspend
			fmt.Println("DEBUG: Suspending the application...")
			By("Suspending the application")
			err := k8sClient.Get(ctx, typeNamespacedName, applicationdefinition)
			Expect(err).NotTo(HaveOccurred())
			suspend := true
			applicationdefinition.Spec.Suspend = &suspend
			fmt.Println("DEBUG: Updating application definition...")
			Expect(k8sClient.Update(ctx, applicationdefinition)).To(Succeed())
			fmt.Println("DEBUG: Update successful.")

			// Reconcile again
			fmt.Println("DEBUG: Reconciling for suspend...")
			for i := 0; i < 10; i++ {
				fmt.Printf("DEBUG: Reconcile iteration %d\n", i)
				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())
				if !result.Requeue && result.RequeueAfter == 0 {
					fmt.Println("DEBUG: Reconcile finished (no requeue).")
					break
				}
			}

			// Verify StatefulSet scaled to 0
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-comp", Namespace: "default"}, sts)
				if err != nil {
					return -1
				}
				return *sts.Spec.Replicas
			}, time.Second*30, time.Millisecond*500).Should(Equal(int32(0)))

			// Verify Status.SuspendedReplicas
			err = k8sClient.Get(ctx, typeNamespacedName, applicationdefinition)
			Expect(err).NotTo(HaveOccurred())
			Expect(applicationdefinition.Status.SuspendedReplicas).To(HaveKeyWithValue("test-comp", int32(3)))

			// 3. Resume
			By("Resuming the application")
			suspend = false
			applicationdefinition.Spec.Suspend = &suspend
			Expect(k8sClient.Update(ctx, applicationdefinition)).To(Succeed())

			// Reconcile again
			for i := 0; i < 5; i++ {
				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())
				if !result.Requeue && result.RequeueAfter == 0 {
					break
				}
			}

			// Verify StatefulSet scaled back to 3
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-comp", Namespace: "default"}, sts)
				if err != nil {
					return -1
				}
				return *sts.Spec.Replicas
			}, time.Second*10, time.Millisecond*500).Should(Equal(int32(3)))
		})
	})
})
