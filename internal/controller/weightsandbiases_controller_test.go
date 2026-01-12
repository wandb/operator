package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	v2 "github.com/wandb/operator/internal/controller/v2"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/manifest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("WeightsAndBiases Controller V2", func() {
	const (
		WandbName      = "test-wandb-v2"
		WandbNamespace = "default"
		timeout        = time.Second * 10
		duration       = time.Second * 10
		interval       = time.Millisecond * 250
	)

	Context("When reconciling a v2 WeightsAndBiases object", func() {
		It("Should successfully reconcile and update status", func() {
			By("Creating a new WeightsAndBiases v2 object")
			ctx := context.Background()
			manifest.Path = "../../hack/testing-manifests/server-manifest/0.76.1.yaml"
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName,
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.WBSizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname: "http://localhost",
						Features: map[string]bool{
							"proxy": true,
						},
					},
					MySQL: apiv2.WBMySQLSpec{
						Enabled:     true,
						StorageSize: "10Gi",
					},
					Redis: apiv2.WBRedisSpec{
						Enabled:     true,
						StorageSize: "10Gi",
					},
					Kafka: apiv2.WBKafkaSpec{
						Enabled:     true,
						StorageSize: "10Gi",
					},
					Minio: apiv2.WBMinioSpec{
						Enabled:     true,
						StorageSize: "10Gi",
					},
					ClickHouse: apiv2.WBClickHouseSpec{
						Enabled: true,
					},
				},
				Status: apiv2.WeightsAndBiasesStatus{},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: WandbName, Namespace: WandbNamespace}
			createdWandb := &apiv2.WeightsAndBiases{}

			By("Running the reconciler")
			reconciler := &WeightsAndBiasesReconciler{
				Client:   k8sClient,
				Scheme:   scheme.Scheme,
				Recorder: record.NewFakeRecorder(10),
				EnableV2: true,
			}

			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: wandbLookupKey,
			})

			Expect(err).Should(Succeed())

			By("Checking if finalizers were added")
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			Expect(utils.ContainsString(createdWandb.GetFinalizers(), "wandb.apps.wandb.com/cleanup")).Should(BeTrue())

			// Cleanup
			Expect(k8sClient.Delete(ctx, createdWandb)).Should(Succeed())
		})

		It("Should create application components when infrastructure is ready", func() {
			By("Creating a new WeightsAndBiases v2 object with ready infrastructure")
			ctx := context.Background()
			manifest.Path = "../../hack/testing-manifests/server-manifest/0.76.1.yaml"
			wandbVersion := "0.76.1"
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName + "-ready",
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.WBSizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname: "http://localhost",
						Version:  wandbVersion,
						Features: map[string]bool{},
					},
					MySQL: apiv2.WBMySQLSpec{
						Enabled: true,
					},
					Redis: apiv2.WBRedisSpec{
						Enabled: true,
					},
					Kafka: apiv2.WBKafkaSpec{
						Enabled: true,
					},
					Minio: apiv2.WBMinioSpec{
						Enabled: true,
					},
					ClickHouse: apiv2.WBClickHouseSpec{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}
			createdWandb := &apiv2.WeightsAndBiases{}
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			By("Running the reconciler")
			reconciler := &WeightsAndBiasesReconciler{
				Client:   k8sClient,
				Scheme:   scheme.Scheme,
				Recorder: record.NewFakeRecorder(10),
				EnableV2: true,
			}

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: wandbLookupKey,
			})
			Expect(err).Should(Succeed())

			By("Setting infrastructure status to ready")
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			createdWandb.Status.MySQLStatus.Ready = true
			createdWandb.Status.RedisStatus.Ready = true
			createdWandb.Status.KafkaStatus.Ready = true
			createdWandb.Status.MinioStatus.Ready = true
			createdWandb.Status.ClickHouseStatus.Ready = true

			Expect(k8sClient.Status().Update(ctx, createdWandb)).Should(Succeed())

			// For now test by calling ReconcileWandbManifest directly, but this will get refactored into the reconciler later
			wandbManifest, err := manifest.GetServerManifest(wandbVersion)
			Expect(err).Should(Succeed())
			ctrlResult, err := v2.ReconcileWandbManifest(ctx, k8sClient, createdWandb, wandbManifest)
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeZero())

			By("Checking if Applications were created")
			appList := &apiv2.ApplicationList{}
			Expect(k8sClient.List(ctx, appList, client.InNamespace(WandbNamespace))).Should(Succeed())

			// The 0.76.1.yaml manifest should have some applications defined.
			// We expect them to be created as Application CRs.
			Expect(len(appList.Items)).Should(BeNumerically("==", len(wandbManifest.Applications)-2), "Expected all non-feature flagged applications to be created")

			// Cleanup
			Expect(k8sClient.Delete(ctx, createdWandb)).Should(Succeed())
		})
	})
})
