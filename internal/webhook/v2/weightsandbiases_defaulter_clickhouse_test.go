package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiasesCustomDefaulter - ClickHouse", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
	})

	It("defaults ClickHouse namespace to the parent namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec:       apiv2.WeightsAndBiasesSpec{ClickHouse: map[string]apiv2.ClickHouseSpec{apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}}}},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.Namespace).To(g.Equal("test-namespace"))
	})

	It("preserves a custom ClickHouse namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				ClickHouse: map[string]apiv2.ClickHouseSpec{apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{Namespace: "custom-clickhouse-namespace"}}},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.Namespace).To(g.Equal("custom-clickhouse-namespace"))
	})

	It("does not mutate unrelated ClickHouse fields", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				ClickHouse: map[string]apiv2.ClickHouseSpec{
					apiv2.DefaultInstanceName: {
						ManagedClickHouse: &apiv2.ManagedClickHouseSpec{
							StorageSize: "100Gi",
							Replicas:    2,
							Version:     "24.1",
						},
					},
				},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.StorageSize).To(g.Equal("100Gi"))
		g.Expect(wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.Replicas).To(g.Equal(int32(2)))
		g.Expect(wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.Version).To(g.Equal("24.1"))
	})

	It("defaults the plain '<cr>-chi' name for CR names that fit", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec:       apiv2.WeightsAndBiasesSpec{ClickHouse: map[string]apiv2.ClickHouseSpec{apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}}}},
		}

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		g.Expect(wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.Name).To(g.Equal("test-wandb-chi"))
		g.Expect(wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.ServiceAccount.Create).ToNot(g.BeNil())
		g.Expect(*wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.ServiceAccount.Create).To(g.BeTrue())
		g.Expect(wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.ServiceAccount.ServiceAccountName).To(g.Equal("test-wandb-chi"))
	})

	It("keys non-default instance names before the suffix", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{ClickHouse: map[string]apiv2.ClickHouseSpec{
				apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}},
				"analytics":               {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}},
			}},
		}

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		g.Expect(wandb.Spec.ClickHouse["analytics"].ManagedClickHouse.Name).To(g.Equal("test-wandb-analytics-chi"))
	})

	It("defaults a deployable name for CR names the plain default would wedge", func() {
		// 32 chars: "<cr>-chi" would overflow the derived per-host volume names
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb-integration-environments-2", Namespace: "test-namespace"},
			Spec:       apiv2.WeightsAndBiasesSpec{ClickHouse: map[string]apiv2.ClickHouseSpec{apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}}}},
		}

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())

		managed := wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse
		g.Expect(managed.Name).To(g.HaveSuffix("-chi"))
		g.Expect(len(managed.Name)).To(g.BeNumerically("<=", altinity.MaxSpecNameLength()))
		g.Expect(altinity.ValidateDerivedNames(managed)).To(g.Succeed())

		// persisted in the spec, so it must be deterministic
		again := &apiv2.WeightsAndBiases{
			ObjectMeta: wandb.ObjectMeta,
			Spec:       apiv2.WeightsAndBiasesSpec{ClickHouse: map[string]apiv2.ClickHouseSpec{apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}}}},
		}
		g.Expect(defaulter.Default(ctx, again)).To(g.Succeed())
		g.Expect(again.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.Name).To(g.Equal(managed.Name))
	})

	It("keeps plain default names for the other infra at CR lengths only ClickHouse would break", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb-legacy-overrides-v1", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				MySQL:       map[string]apiv2.MySQLSpec{apiv2.DefaultInstanceName: {ManagedMysql: &apiv2.ManagedMysqlSpec{}}},
				Redis:       map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {ManagedRedis: &apiv2.ManagedRedisSpec{}}},
				Kafka:       apiv2.KafkaSpec{ManagedKafka: &apiv2.ManagedKafkaSpec{}},
				ObjectStore: map[string]apiv2.ObjectStoreSpec{apiv2.DefaultInstanceName: {ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{}}},
			},
		}

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		g.Expect(wandb.Spec.MySQL[apiv2.DefaultInstanceName].ManagedMysql.Name).To(g.Equal("wandb-legacy-overrides-v1-mysql"))
		g.Expect(wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis.Name).To(g.Equal("wandb-legacy-overrides-v1-redis"))
		g.Expect(wandb.Spec.Kafka.ManagedKafka.Name).To(g.Equal("wandb-legacy-overrides-v1-kafka"))
		g.Expect(wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore.Name).To(g.Equal("wandb-legacy-overrides-v1-seaweedfs"))
	})

	It("does not apply defaults when ExternalClickhouse is present", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				ClickHouse: map[string]apiv2.ClickHouseSpec{
					apiv2.DefaultInstanceName: {ExternalClickHouse: &apiv2.ClickHouseConnection{}},
				},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse).To(g.BeNil())
	})
})
