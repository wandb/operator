package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	wandbcomv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/channel/deployer/deployerfakes"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
	"github.com/wandb/operator/pkg/wandb/spec/state"
	"github.com/wandb/operator/pkg/wandb/spec/state/secrets"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

var deployerSpec = spec.Spec{
	Metadata: &spec.Metadata{
		"channelId":        "b56e1972-3c78-4de0-af90-e3597bb0785a",
		"channelName":      "Stable",
		"releaseId":        "74a7f750-de86-43fe-a945-5350646cf415",
		"releaseName":      "v202407-2.11",
		"releaseCreatedAt": "2024-07-02T18:35:42.479Z",
	},
	Chart: &charts.RepoRelease{
		URL:     "https://charts.wandb.ai",
		Name:    "operator-wandb",
		Version: "0.14.3",
		Debug:   false,
	},
	Values: spec.Values{
		"app": map[string]interface{}{
			"image": map[string]interface{}{
				"tag":        "0.56.0",
				"repository": "wandb/local",
			},
		},
		"weave": map[string]interface{}{
			"image": map[string]interface{}{
				"tag":        "0.56.0",
				"repository": "wandb/local",
			},
		},
		"console": map[string]interface{}{
			"image": map[string]interface{}{
				"tag":        "2.6.0",
				"repository": "wandb/console",
			},
		},
		"parquet": map[string]interface{}{
			"image": map[string]interface{}{
				"tag":        "0.56.0",
				"repository": "wandb/local",
			},
		},
	},
}

var recorder *record.FakeRecorder
var reconciler *WeightsAndBiasesReconciler

var _ = Describe("WeightsandbiasesController", func() {
	Describe("DryRun Reconcile", func() {
		BeforeEach(func() {
			ctx := context.Background()
			recorder = record.NewFakeRecorder(10)
			deployerClient := &deployerfakes.FakeDeployerInterface{}
			deployerClient.GetSpecReturns(&deployerSpec, nil)
			reconciler = &WeightsAndBiasesReconciler{
				Client:         k8sClient,
				IsAirgapped:    false,
				DeployerClient: deployerClient,
				Scheme:         scheme.Scheme,
				Recorder:       recorder,
				DryRun:         true,
			}
			wandb := wandbcomv1.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: wandbcomv1.WeightsAndBiasesSpec{
					Chart: wandbcomv1.Object{Object: map[string]interface{}{}},
					Values: wandbcomv1.Object{Object: map[string]interface{}{
						"global": map[string]interface{}{
							"host": "https://qa-google.wandb.io",
						},
						"ingress": map[string]interface{}{
							"annotations": map[string]interface{}{
								"ingress.gcp.kubernetes.io/pre-shared-cert":   "wandb-qa-local-cert-content-hawk",
								"kubernetes.io/ingress.class":                 "gce",
								"kubernetes.io/ingress.global-static-ip-name": "wandb-qa-local-operator-address",
							},
							"nameOverride": "wandb-qa-local",
						},
					}},
				},
			}
			err := k8sClient.Create(ctx, &wandb)
			Expect(err).ToNot(HaveOccurred())
			res, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}})
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{RequeueAfter: time.Duration(1 * time.Hour)}))
		})
		AfterEach(func() {
			ctx := context.Background()
			wandb := wandbcomv1.WeightsAndBiases{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "test", Namespace: "default"}, &wandb)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, &wandb)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-spec-active", Namespace: "default"}})
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}})
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "test", Namespace: "default"}, &wandb)
			Expect(err).To(HaveOccurred())
		})
		Context("When a WeightsAndBiases instance is created", func() {
			It("Should add the finalizer to the instance", func() {
				ctx := context.Background()
				wandb := wandbcomv1.WeightsAndBiases{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test", Namespace: "default"}, &wandb)
				Expect(err).ToNot(HaveOccurred())
				Expect(wandb.ObjectMeta.Finalizers).To(ContainElement(resFinalizer))
			})
			It("Should record a sequence of events", func() {
				Expect(recorder.Events).To(HaveLen(3))
				event := <-recorder.Events
				Expect(event).To(ContainSubstring("Normal Reconciling Reconciling"))
				event = <-recorder.Events
				Expect(event).To(ContainSubstring("Normal LoadingConfig Loading desired configuration"))
				event = <-recorder.Events
				Expect(event).To(ContainSubstring("Completed reconcile successfully"))
			})
			It("Should create an empty User Spec", func() {
				ctx := context.Background()
				wandb := wandbcomv1.WeightsAndBiases{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test", Namespace: "default"}, &wandb)
				Expect(err).ToNot(HaveOccurred())
				specManager := state.New(ctx, k8sClient, &wandb, scheme.Scheme, secrets.New(ctx, k8sClient, &wandb, scheme.Scheme))
				userSpec, err := specManager.GetUserInput()
				Expect(err).ToNot(HaveOccurred())
				Expect(userSpec.Values).To(BeEmpty())
				Expect(userSpec.Chart).To(BeNil())
				Expect(userSpec.Metadata).To(BeNil())
			})
		})
	})
	Describe("Reconcile with _releaseId set", func() {
		BeforeEach(func() {
			ctx := context.Background()
			recorder = record.NewFakeRecorder(10)
			deployerClient := &deployerfakes.FakeDeployerInterface{}
			deployerClient.GetSpecReturns(&deployerSpec, nil)
			reconciler = &WeightsAndBiasesReconciler{
				Client:         k8sClient,
				IsAirgapped:    false,
				DeployerClient: deployerClient,
				Scheme:         scheme.Scheme,
				Recorder:       recorder,
				DryRun:         true,
			}
			wandb := wandbcomv1.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-release-id",
					Namespace: "default",
				},
				Spec: wandbcomv1.WeightsAndBiasesSpec{
					Chart: wandbcomv1.Object{Object: map[string]interface{}{}},
					Values: wandbcomv1.Object{Object: map[string]interface{}{
						"global": map[string]interface{}{
							"host": "https://qa-google.wandb.io",
						},
					}},
				},
			}
			err := k8sClient.Create(ctx, &wandb)
			Expect(err).ToNot(HaveOccurred())

			// Create UserSpec with _releaseId
			userSpec := &spec.Spec{
				Values: map[string]interface{}{
					"_releaseId": "0b901113-8135-48ae-bdaf-6fa82b4b2d28",
				},
			}
			err = state.New(ctx, k8sClient, &wandb, scheme.Scheme, secrets.New(ctx, k8sClient, &wandb, scheme.Scheme)).SetUserInput(userSpec)
			Expect(err).ToNot(HaveOccurred())

			res, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}})
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{RequeueAfter: time.Duration(1 * time.Hour)}))
		})

		AfterEach(func() {
			ctx := context.Background()
			wandb := wandbcomv1.WeightsAndBiases{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-release-id", Namespace: "default"}, &wandb)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, &wandb)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-release-id-spec-active", Namespace: "default"}})
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}})
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-release-id", Namespace: "default"}, &wandb)
			Expect(err).To(HaveOccurred())
		})

		It("Should use the specified _releaseId from UserSpec in the final spec", func() {
			ctx := context.Background()
			wandb := wandbcomv1.WeightsAndBiases{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-release-id", Namespace: "default"}, &wandb)
			Expect(err).ToNot(HaveOccurred())

			specManager := state.New(ctx, k8sClient, &wandb, scheme.Scheme, secrets.New(ctx, k8sClient, &wandb, scheme.Scheme))
			activeSpec, err := specManager.GetActive()
			Expect(err).ToNot(HaveOccurred())
			Expect(activeSpec.Values["_releaseId"]).To(Equal("0b901113-8135-48ae-bdaf-6fa82b4b2d28"))
		})
	})
	//TODO(dpanzella): Uncomment after fixing the helm kubernetes client to not be the default
	//Describe("Reconcile and Apply", func() {
	//	BeforeEach(func() {
	//		ctx := context.Background()
	//		recorder = record.NewFakeRecorder(10)
	//		deployerClient := &deployerfakes.FakeDeployerInterface{}
	//		deployerClient.GetSpecReturns(&deployerSpec, nil)
	//		reconciler = &WeightsAndBiasesReconciler{
	//			Client:         k8sClient,
	//			IsAirgapped:    false,
	//			DeployerClient: deployerClient,
	//			Scheme:         scheme.Scheme,
	//			Recorder:       recorder,
	//			DryRun:         false,
	//		}
	//		wandb := wandbcomv1.WeightsAndBiases{
	//			ObjectMeta: metav1.ObjectMeta{
	//				Name:      "test",
	//				Namespace: "default",
	//			},
	//			Spec: wandbcomv1.WeightsAndBiasesSpec{
	//				Chart: wandbcomv1.Object{Object: map[string]interface{}{}},
	//				Values: wandbcomv1.Object{Object: map[string]interface{}{
	//					"global": map[string]interface{}{
	//						"host": "https://qa-google.wandb.io",
	//					},
	//					"ingress": map[string]interface{}{
	//						"annotations": map[string]interface{}{
	//							"ingress.gcp.kubernetes.io/pre-shared-cert":   "wandb-qa-local-cert-content-hawk",
	//							"kubernetes.io/ingress.class":                 "gce",
	//							"kubernetes.io/ingress.global-static-ip-name": "wandb-qa-local-operator-address",
	//						},
	//						"nameOverride": "wandb-qa-local",
	//					},
	//				}},
	//			},
	//		}
	//		err := k8sClient.Create(ctx, &wandb)
	//		Expect(err).ToNot(HaveOccurred())
	//		res, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}})
	//		Expect(err).ToNot(HaveOccurred())
	//		Expect(res).To(Equal(ctrl.Result{RequeueAfter: time.Duration(1 * time.Hour)}))
	//	})
	//	AfterEach(func() {
	//		ctx := context.Background()
	//		wandb := wandbcomv1.WeightsAndBiases{}
	//		err := k8sClient.Get(ctx, types.NamespacedName{Name: "test", Namespace: "default"}, &wandb)
	//		Expect(err).ToNot(HaveOccurred())
	//		err = k8sClient.Delete(ctx, &wandb)
	//		Expect(err).ToNot(HaveOccurred())
	//		err = k8sClient.Delete(ctx, &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-spec-active", Namespace: "default"}})
	//		Expect(err).ToNot(HaveOccurred())
	//		err = k8sClient.Get(ctx, types.NamespacedName{Name: "test", Namespace: "default"}, &wandb)
	//		Expect(err).ToNot(HaveOccurred())
	//		controllerutil.RemoveFinalizer(&wandb, resFinalizer)
	//		err = k8sClient.Update(ctx, &wandb)
	//		Expect(err).ToNot(HaveOccurred())
	//		err = k8sClient.Get(ctx, types.NamespacedName{Name: "test", Namespace: "default"}, &wandb)
	//		Expect(err).To(HaveOccurred())
	//	})
	//	Context("When a WeightsAndBiases instance is created", func() {
	//		It("Should add the finalizer to the instance", func() {
	//			ctx := context.Background()
	//			wandb := wandbcomv1.WeightsAndBiases{}
	//			err := k8sClient.Get(ctx, types.NamespacedName{Name: "test", Namespace: "default"}, &wandb)
	//			Expect(err).ToNot(HaveOccurred())
	//			Expect(wandb.ObjectMeta.Finalizers).To(ContainElement(resFinalizer))
	//		})
	//		It("Should record a sequence of events", func() {
	//			Expect(recorder.Events).To(HaveLen(3))
	//			event := <-recorder.Events
	//			Expect(event).To(ContainSubstring("Normal Reconciling Reconciling"))
	//			event = <-recorder.Events
	//			Expect(event).To(ContainSubstring("Normal LoadingConfig Loading desired configuration"))
	//			event = <-recorder.Events
	//			Expect(event).To(ContainSubstring("Completed reconcile successfully"))
	//		})
	//		It("Should add ownerrefs to the resources", func() {
	//			ctx := context.Background()
	//			labels := client.MatchingLabels{
	//				"app.kubernetes.io/managed-by": "Helm",
	//				"app.kubernetes.io/instance":   "test",
	//			}
	//			deploymentList := appsv1.DeploymentList{}
	//			err := k8sClient.List(ctx, &deploymentList, labels)
	//			Expect(err).ToNot(HaveOccurred())
	//			for _, deployment := range deploymentList.Items {
	//				ownerRefs := deployment.GetOwnerReferences()
	//				Expect(ownerRefs).To(HaveLen(1))
	//				Expect(ownerRefs[0].Name).To(Equal("test"))
	//			}
	//		})
	//	})
	//})
})
