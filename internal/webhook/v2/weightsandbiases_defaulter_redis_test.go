package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiasesCustomDefaulter - Redis", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
	})

	It("defaults Redis namespace to the parent namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec:       apiv2.WeightsAndBiasesSpec{Redis: apiv2.RedisSpec{ManagedInfraSpec: apiv2.ManagedInfraSpec{Enabled: true}}},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Redis.Namespace).To(g.Equal("test-namespace"))
	})

	It("preserves a custom Redis namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Redis: apiv2.RedisSpec{ManagedInfraSpec: apiv2.ManagedInfraSpec{Enabled: true}, Namespace: "custom-redis-namespace"},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Redis.Namespace).To(g.Equal("custom-redis-namespace"))
	})

	It("does not mutate unrelated Redis fields", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Redis: apiv2.RedisSpec{ManagedInfraSpec: apiv2.ManagedInfraSpec{Enabled: true}, StorageSize: "20Gi", Sentinel: apiv2.RedisSentinelSpec{Enabled: true}},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Redis.StorageSize).To(g.Equal("20Gi"))
		g.Expect(wandb.Spec.Redis.Sentinel.Enabled).To(g.BeTrue())
	})
})
