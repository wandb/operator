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
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/pkg/vendored/argo-rollouts/argoproj.io.rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// serviceUpdateCounter counts Service Update calls passing through the client.
type serviceUpdateCounter struct {
	client.Client
	updates int
}

func (c *serviceUpdateCounter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*corev1.Service); ok {
		c.updates++
	}
	return c.Client.Update(ctx, obj, opts...)
}

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
					HpaTemplate: &autoscalingv2.HorizontalPodAutoscalerSpec{
						MinReplicas: &minReplicas,
						MaxReplicas: maxReplicas,
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
			foundHPA := &autoscalingv2.HorizontalPodAutoscaler{}
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

		It("should propagate VolumeClaimTemplates to the StatefulSet", func() {
			resourceName := "test-statefulset-pvc"
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
									Name:  "etcd",
									Image: "etcd",
									VolumeMounts: []corev1.VolumeMount{
										{Name: "data", MountPath: "/data"},
									},
								},
							},
						},
					},
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "data"},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")},
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

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			found := &appsv1.StatefulSet{}
			err = k8sClient.Get(ctx, typeNamespacedName, found)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Spec.VolumeClaimTemplates).To(HaveLen(1))
			Expect(found.Spec.VolumeClaimTemplates[0].Name).To(Equal("data"))
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
			_, reconcileErr := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(reconcileErr).NotTo(HaveOccurred())
			// 2. Create Deployment and Service
			_, reconcileErr = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(reconcileErr).NotTo(HaveOccurred())

			found := &corev1.Service{}
			err := k8sClient.Get(ctx, typeNamespacedName, found)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Spec.Ports[0].Port).To(Equal(int32(80)))
		})

		It("round-trips a normalized ServiceTemplate without drift", func() {
			// The Application CRD schema defaults serviceTemplate.ports[].protocol,
			// so an un-normalized template reads back different from what was
			// written — the drift that made the parent's update gate fire on every
			// reconcile and kept Service-bearing Applications churning.
			raw := &corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 8080}}}

			rawApp := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roundtrip-raw", Namespace: "default"},
				Spec: apiv2.ApplicationSpec{
					Kind: "Deployment",
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "nginx"}}},
					},
					ServiceTemplate: raw.DeepCopy(),
				},
			}
			Expect(k8sClient.Create(ctx, rawApp)).To(Succeed())

			fetched := &apiv2.Application{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-roundtrip-raw", Namespace: "default"}, fetched)).To(Succeed())
			Expect(fetched.Spec.ServiceTemplate.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP),
				"CRD schema defaulting fills ports[].protocol")
			Expect(apiequality.Semantic.DeepEqual(fetched.Spec.ServiceTemplate, raw)).To(BeFalse(),
				"un-normalized templates do not round-trip; reconcilers must not build them")

			// The normalized form (what reconcileApplications writes now) is stable.
			normalized := raw.DeepCopy()
			common.NormalizeServicePorts(normalized.Ports)
			normApp := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roundtrip-norm", Namespace: "default"},
				Spec: apiv2.ApplicationSpec{
					Kind: "Deployment",
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "nginx"}}},
					},
					ServiceTemplate: normalized.DeepCopy(),
				},
			}
			Expect(k8sClient.Create(ctx, normApp)).To(Succeed())
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-roundtrip-norm", Namespace: "default"}, fetched)).To(Succeed())
			Expect(apiequality.Semantic.DeepEqual(fetched.Spec.ServiceTemplate, normalized)).To(BeTrue(),
				"normalized templates round-trip unchanged, so the update gate settles")
		})

		It("does not update the Service on a steady-state reconcile", func() {
			resourceName := "test-service-steady"
			typeNamespacedName := types.NamespacedName{Name: resourceName, Namespace: "default"}

			resource := &apiv2.Application{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: "default"},
				Spec: apiv2.ApplicationSpec{
					Kind: "Deployment",
					PodTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "nginx"}}},
					},
					// No protocol/targetPort: mirrors templates written before
					// normalization existed.
					ServiceTemplate: &corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			// Count Service Update calls: the API server absorbs writes of
			// defaulted-back fields as no-ops (resourceVersion holds), but each
			// call still fires the Owns(Service) watch and re-queues the
			// Application — the hot loop this test pins.
			counter := &serviceUpdateCounter{Client: k8sClient}
			controllerReconciler := &ApplicationReconciler{Client: counter, Scheme: k8sClient.Scheme()}

			// 1. Add finalizer; 2. create Deployment and Service.
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			svc := &corev1.Service{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, svc)).To(Succeed())

			// Steady state: the API server has defaulted fields the template leaves
			// unset (protocol, targetPort, sessionAffinity, type, ...); reconciling
			// again must not write the Service at all.
			counter.updates = 0
			for i := 0; i < 3; i++ {
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(counter.updates).To(BeZero(),
				"steady-state reconciles must not update the Service")
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
			_, reconcileErr := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(reconcileErr).NotTo(HaveOccurred())
			// 2. Create Resources
			_, reconcileErr = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(reconcileErr).NotTo(HaveOccurred())

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
			_, reconcileErr := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(reconcileErr).NotTo(HaveOccurred())
			// 2. Create Resources
			_, reconcileErr = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(reconcileErr).NotTo(HaveOccurred())

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
