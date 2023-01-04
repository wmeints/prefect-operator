/*
Copyright 2023 Willem Meints.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mlopsv1alpha1 "github.com/wmeints/prefect-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("PrefectEnvironment controller", func() {
	Context("PrefectEnvironment controller test", func() {
		environmentName := "test-environment"
		environmentNamespace := "prefect-operator-test"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: environmentNamespace,
			},
		}

		BeforeEach(func() {
			By("Creating a namespace for the test")
			Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
		})

		AfterEach(func() {
			By("Deleting the namespace for the test")
			Expect(k8sClient.Delete(ctx, namespace)).Should(Succeed())
		})

		It("Should succesfully reconcile a non-existing environment", func() {
			environment := &mlopsv1alpha1.PrefectEnvironment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: environmentNamespace,
				},
				Spec: mlopsv1alpha1.PrefectEnvironmentSpec{
					AgentReplicas:    1,
					DatabaseReplicas: 1,
					OrionReplicas:    1,
				},
			}

			By("Creating a new PrefectEnvironment")
			Expect(k8sClient.Create(ctx, environment)).Should(Succeed())

			By("Checking if the environment was created")
			Eventually(func() error {
				found := &mlopsv1alpha1.PrefectEnvironment{}
				return k8sClient.Get(ctx, types.NamespacedName{Name: environmentName, Namespace: environmentNamespace}, found)
			}, time.Minute, time.Second).Should(Succeed())

			reconciler := PrefectEnvironmentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: environmentName, Namespace: environmentNamespace},
			})

			Expect(err).To(Not(HaveOccurred()))

			By("Checking if the database deployment was created")
			Eventually(func() error {
				databaseDeployment := &appsv1.Deployment{}
				deploymentName := types.NamespacedName{
					Name:      fmt.Sprintf("%s-orion-database", environmentName),
					Namespace: environmentNamespace,
				}

				return k8sClient.Get(ctx, deploymentName, databaseDeployment)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if the database service was created")
			Eventually(func() error {
				service := &corev1.Service{}
				serviceName := types.NamespacedName{
					Name:      fmt.Sprintf("%s-orion-database", environmentName),
					Namespace: environmentNamespace,
				}

				return k8sClient.Get(ctx, serviceName, service)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if the orion deployment was created")
			Eventually(func() error {
				deployment := &appsv1.Deployment{}
				deploymentName := types.NamespacedName{
					Name:      fmt.Sprintf("%s-orion-server", environmentName),
					Namespace: environmentNamespace,
				}

				return k8sClient.Get(ctx, deploymentName, deployment)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if the orion service was created")
			Eventually(func() error {
				service := &corev1.Service{}
				serviceName := types.NamespacedName{
					Name:      fmt.Sprintf("%s-orion-server", environmentName),
					Namespace: environmentNamespace,
				}

				return k8sClient.Get(ctx, serviceName, service)
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
