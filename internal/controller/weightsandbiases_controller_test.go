package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
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

			fmt.Print(createdWandb)
			Expect(utils.ContainsString(createdWandb.GetFinalizers(), "wandb.apps.wandb.com/cleanup")).Should(BeTrue())

			// Cleanup
			Expect(k8sClient.Delete(ctx, createdWandb)).Should(Succeed())
		})
	})
})
