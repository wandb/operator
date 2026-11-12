package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv2 "github.com/wandb/operator/api/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiases retentionPolicy", func() {
	var (
		ctx       context.Context
		obj       *appsv2.WeightsAndBiases
		oldObj    *appsv2.WeightsAndBiases
		validator WeightsAndBiasesCustomValidator
	)

	newWandb := func(name string) *appsv2.WeightsAndBiases {
		wandb := &appsv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
		wandb.Spec.Wandb.Hostname = "https://wandb.example.com"
		wandb.Spec.Wandb.Version = testPinnedVersion
		return wandb
	}
	policy := func(p appsv2.OnDeletePolicy) *appsv2.RetentionPolicy {
		return &appsv2.RetentionPolicy{OnDelete: p}
	}

	BeforeEach(func() {
		ctx = context.Background()
		obj = newWandb("wandb")
		oldObj = newWandb("wandb")
		validator = WeightsAndBiasesCustomValidator{}
	})

	Context("validating webhook", func() {
		It("rejects an unknown top-level value", func() {
			obj.Spec.RetentionPolicy.OnDelete = "Detach"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`spec.retentionPolicy.onDelete: Unsupported value: "Detach": supported values: "detach", "purge"`))
		})

		DescribeTable("rejects an unknown per-instance value",
			func(set func(*appsv2.WeightsAndBiases), path string) {
				set(obj)

				_, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(path + `: Unsupported value: "retain"`))
			},
			Entry("mysql", func(w *appsv2.WeightsAndBiases) {
				w.Spec.MySQL = map[string]appsv2.MySQLSpec{appsv2.DefaultInstanceName: {ManagedMysql: &appsv2.ManagedMysqlSpec{
					ManagedInfraSpec: appsv2.ManagedInfraSpec{RetentionPolicy: policy("retain")},
				}}}
			}, "spec.mysql[default].managedMysql.retentionPolicy.onDelete"),
			Entry("redis", func(w *appsv2.WeightsAndBiases) {
				w.Spec.Redis = map[string]appsv2.RedisSpec{appsv2.DefaultInstanceName: {ManagedRedis: &appsv2.ManagedRedisSpec{
					ManagedInfraSpec: appsv2.ManagedInfraSpec{RetentionPolicy: policy("retain")},
				}}}
			}, "spec.redis[default].managedRedis.retentionPolicy.onDelete"),
			Entry("object store", func(w *appsv2.WeightsAndBiases) {
				w.Spec.ObjectStore = map[string]appsv2.ObjectStoreSpec{appsv2.DefaultInstanceName: {ManagedObjectStore: &appsv2.ManagedObjectStoreSpec{
					ManagedInfraSpec: appsv2.ManagedInfraSpec{RetentionPolicy: policy("retain")},
				}}}
			}, "spec.objectStore[default].managedObjectStore.retentionPolicy.onDelete"),
			Entry("clickhouse", func(w *appsv2.WeightsAndBiases) {
				w.Spec.ClickHouse = map[string]appsv2.ClickHouseSpec{appsv2.DefaultInstanceName: {ManagedClickHouse: &appsv2.ManagedClickHouseSpec{
					ManagedInfraSpec: appsv2.ManagedInfraSpec{RetentionPolicy: policy("retain")},
				}}}
			}, "spec.clickhouse[default].managedClickhouse.retentionPolicy.onDelete"),
			Entry("kafka", func(w *appsv2.WeightsAndBiases) {
				w.Spec.Kafka.ManagedKafka = &appsv2.ManagedKafkaSpec{
					ManagedInfraSpec: appsv2.ManagedInfraSpec{RetentionPolicy: policy("retain")},
				}
			}, "spec.kafka.managedKafka.retentionPolicy.onDelete"),
		)

		It("accepts detach and purge at both levels", func() {
			obj.Spec.RetentionPolicy.OnDelete = appsv2.PurgeOnDelete
			obj.Spec.MySQL = map[string]appsv2.MySQLSpec{appsv2.DefaultInstanceName: {ManagedMysql: &appsv2.ManagedMysqlSpec{
				ManagedInfraSpec: appsv2.ManagedInfraSpec{RetentionPolicy: policy(appsv2.DetachOnDelete)},
			}}}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("accepts an unknown value left unchanged on update", func() {
			oldObj.Spec.RetentionPolicy.OnDelete = "retain"
			obj.Spec.RetentionPolicy.OnDelete = "retain"

			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("rejects changing to an unknown value on update", func() {
			oldObj.Spec.RetentionPolicy.OnDelete = appsv2.DetachOnDelete
			obj.Spec.RetentionPolicy.OnDelete = "retain"

			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.retentionPolicy.onDelete"))
		})
	})

	Context("CRD schema", func() {
		It("rejects an unknown top-level value", func() {
			wandb := newWandb("retention-bad-top-level")
			wandb.Spec.RetentionPolicy.OnDelete = "retain"

			err := k8sClient.Create(ctx, wandb)
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid, got %v", err)
			Expect(err.Error()).To(ContainSubstring(`spec.retentionPolicy.onDelete: Unsupported value: "retain"`))
			Expect(err.Error()).NotTo(ContainSubstring("admission webhook"))
		})

		It("rejects an unknown per-instance value", func() {
			wandb := newWandb("retention-bad-instance")
			wandb.Spec.MySQL = map[string]appsv2.MySQLSpec{appsv2.DefaultInstanceName: {ManagedMysql: &appsv2.ManagedMysqlSpec{
				ManagedInfraSpec: appsv2.ManagedInfraSpec{RetentionPolicy: policy("Detach")},
			}}}

			err := k8sClient.Create(ctx, wandb)
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid, got %v", err)
			Expect(err.Error()).To(ContainSubstring(`spec.mysql.default.managedMysql.retentionPolicy.onDelete: Unsupported value: "Detach"`))
			Expect(err.Error()).NotTo(ContainSubstring("admission webhook"))
		})

		It("accepts purge", func() {
			wandb := newWandb("retention-purge")
			wandb.Spec.RetentionPolicy.OnDelete = appsv2.PurgeOnDelete

			Expect(k8sClient.Create(ctx, wandb)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, wandb)
		})
	})
})
