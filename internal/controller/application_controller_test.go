/*
Copyright 2025.

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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/vendored/argo-rollouts/argoproj.io.rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Application Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		AfterEach(func() {
			// Cleanup all Applications in the namespace
			appList := &apiv2.ApplicationList{}
			Expect(k8sClient.List(ctx, appList, client.InNamespace("default"))).To(Succeed())
			for _, app := range appList.Items {
				// Remove finalizer to allow deletion
				if len(app.Finalizers) > 0 {
					app.Finalizers = nil
					Expect(k8sClient.Update(ctx, &app)).To(Succeed())
				}
				Expect(k8sClient.Delete(ctx, &app)).To(Succeed())
			}

			// Cleanup other resources that might have been created
			// We can add more cleanup here if needed, but deleting Applications should be enough
			// if owner references are set (though they might not be in these manual tests).
		})

		It("should successfully reconcile a Deployment", func() {
			resourceName := "test-deployment"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			By("creating the custom resource for the Kind Application")
			resource := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: apiv2.ApplicationSpec{
					Kind: "Deployment",
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &ApplicationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling the created resource to add finalizer")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling again to create the Deployment")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the Deployment was created")
			foundDeployment := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, typeNamespacedName, foundDeployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(foundDeployment.Name).To(Equal(resourceName))
			Expect(foundDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal("nginx"))
		})

		It("should successfully scale a Deployment using Replicas field", func() {
			resourceName := "test-replicas"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			var replicas int32 = 3

			resource := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: apiv2.ApplicationSpec{
					Kind:     "Deployment",
					Replicas: &replicas,
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &ApplicationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// Reconcile to add finalizer
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Reconcile to create Deployment
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			foundDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, foundDeployment)).To(Succeed())
			Expect(*foundDeployment.Spec.Replicas).To(Equal(replicas))

			// Update replicas
			var newReplicas int32 = 5
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
			resource.Spec.Replicas = &newReplicas
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, typeNamespacedName, foundDeployment)).To(Succeed())
			Expect(*foundDeployment.Spec.Replicas).To(Equal(newReplicas))
		})

		It("should successfully create and manage an HPA", func() {
			resourceName := "test-hpa"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			var minReplicas int32 = 2
			var maxReplicas int32 = 10

			resource := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: apiv2.ApplicationSpec{
					Kind: "Deployment",
					HpaTemplate: &autoscalingv1.HorizontalPodAutoscalerSpec{
						MinReplicas:                    &minReplicas,
						MaxReplicas:                    maxReplicas,
						TargetCPUUtilizationPercentage: func(i int32) *int32 { return &i }(50),
					},
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &ApplicationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// Reconcile to add finalizer
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Reconcile to create Deployment and HPA
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Check Deployment replicas (should be set to MinReplicas on creation)
			foundDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, foundDeployment)).To(Succeed())
			Expect(*foundDeployment.Spec.Replicas).To(Equal(minReplicas))

			// Check HPA
			foundHPA := &autoscalingv1.HorizontalPodAutoscaler{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, foundHPA)).To(Succeed())
			Expect(*foundHPA.Spec.MinReplicas).To(Equal(minReplicas))
			Expect(foundHPA.Spec.MaxReplicas).To(Equal(maxReplicas))
			Expect(foundHPA.Spec.ScaleTargetRef.Kind).To(Equal("Deployment"))
			Expect(foundHPA.Spec.ScaleTargetRef.Name).To(Equal(resourceName))

			// Remove HPA template and set manual replicas
			var replicas int32 = 4
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
			resource.Spec.HpaTemplate = nil
			resource.Spec.Replicas = &replicas
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())

			// Reconcile again
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// HPA should be deleted
			err = k8sClient.Get(ctx, typeNamespacedName, foundHPA)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())

			// Deployment replicas should be updated
			Expect(k8sClient.Get(ctx, typeNamespacedName, foundDeployment)).To(Succeed())
			Expect(*foundDeployment.Spec.Replicas).To(Equal(replicas))
		})

		It("should successfully reconcile a StatefulSet", func() {
			resourceName := "test-statefulset"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			resource := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: apiv2.ApplicationSpec{
					Kind: "StatefulSet",
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "redis",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &ApplicationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// 1. Add finalizer
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// 2. Create StatefulSet
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			found := &appsv1.StatefulSet{}
			err = k8sClient.Get(ctx, typeNamespacedName, found)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Spec.Template.Spec.Containers[0].Image).To(Equal("redis"))
		})

		It("should successfully reconcile a Rollout", func() {
			resourceName := "test-rollout"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			resource := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: apiv2.ApplicationSpec{
					Kind: "Rollout",
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &ApplicationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// 1. Add finalizer
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// 2. Create Rollout
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			found := &v1alpha1.Rollout{}
			err = k8sClient.Get(ctx, typeNamespacedName, found)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Spec.Template.Spec.Containers[0].Image).To(Equal("nginx:latest"))
		})

		It("should successfully reconcile a DaemonSet", func() {
			resourceName := "test-daemonset"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			resource := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: apiv2.ApplicationSpec{
					Kind: "DaemonSet",
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "fluentd",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &ApplicationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// 1. Add finalizer
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// 2. Reconcile (DaemonSet is currently a no-op in the controller)
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("DaemonSet is currently not implemented, so we just verify reconciliation succeeds")
		})

		It("should successfully reconcile a Service", func() {
			resourceName := "test-service"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			resource := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: apiv2.ApplicationSpec{
					Kind: "Deployment",
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "web", Image: "nginx"}},
						},
					},
					ServiceTemplate: &corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Port: 80,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &ApplicationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// 1. Add finalizer
			controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			// 2. Create Deployment and Service
			controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})

			found := &corev1.Service{}
			err := k8sClient.Get(ctx, typeNamespacedName, found)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Spec.Ports[0].Port).To(Equal(int32(80)))
		})

		It("should successfully reconcile Jobs and CronJobs", func() {
			resourceName := "test-jobs"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			resource := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: apiv2.ApplicationSpec{
					Kind: "Deployment",
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "web", Image: "nginx"}},
						},
					},
					Jobs: []batchv1.Job{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-job"},
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										RestartPolicy: corev1.RestartPolicyNever,
										Containers:    []corev1.Container{{Name: "job", Image: "busybox"}},
									},
								},
							},
						},
					},
					CronJobs: []batchv1.CronJob{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-cronjob"},
							Spec: batchv1.CronJobSpec{
								Schedule: "* * * * *",
								JobTemplate: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												RestartPolicy: corev1.RestartPolicyNever,
												Containers:    []corev1.Container{{Name: "cronjob", Image: "busybox"}},
											},
										},
									},
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &ApplicationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// 1. Add finalizer
			controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			// 2. Create Resources
			controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})

			By("Checking the Job")
			foundJob := &batchv1.Job{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-job", Namespace: "default"}, foundJob)
			Expect(err).NotTo(HaveOccurred())

			By("Checking the CronJob")
			foundCronJob := &batchv1.CronJob{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-cronjob", Namespace: "default"}, foundCronJob)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should successfully delete resources when Application is deleted", func() {
			resourceName := "test-deletion"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			resource := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: apiv2.ApplicationSpec{
					Kind: "Deployment",
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "web", Image: "nginx"}},
						},
					},
					ServiceTemplate: &corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 80}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &ApplicationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// 1. Add finalizer
			controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			// 2. Create Resources
			controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})

			// Verify they exist
			Expect(k8sClient.Get(ctx, typeNamespacedName, &appsv1.Deployment{})).To(Succeed())
			Expect(k8sClient.Get(ctx, typeNamespacedName, &corev1.Service{})).To(Succeed())

			// 3. Delete Application
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			// 4. Reconcile deletion (Finalizer handling)
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Verify resources are gone
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, typeNamespacedName, &appsv1.Deployment{}))).To(BeTrue())
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, typeNamespacedName, &corev1.Service{}))).To(BeTrue())

			// Verify Application is gone (finalizer removed)
			err = k8sClient.Get(ctx, typeNamespacedName, &apiv2.Application{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	})
})
