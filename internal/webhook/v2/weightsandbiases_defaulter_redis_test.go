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
			Spec:       apiv2.WeightsAndBiasesSpec{Redis: map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {ManagedRedis: &apiv2.ManagedRedisSpec{}}}},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis.Namespace).To(g.Equal("test-namespace"))
	})

	It("preserves a custom Redis namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Redis: map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {ManagedRedis: &apiv2.ManagedRedisSpec{Namespace: "custom-redis-namespace"}}},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis.Namespace).To(g.Equal("custom-redis-namespace"))
	})

	It("defaults Sentinel on for managed Redis regardless of size", func() {
		for _, size := range []apiv2.Size{apiv2.SizeDev, apiv2.SizeSmall} {
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size:  size,
					Redis: map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {ManagedRedis: &apiv2.ManagedRedisSpec{}}},
				},
			}

			err := defaulter.Default(ctx, wandb)
			g.Expect(err).ToNot(g.HaveOccurred())
			g.Expect(wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis.Sentinel.Enabled).To(g.HaveValue(g.BeTrue()))
		}
	})

	It("preserves explicitly disabled Sentinel", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Size:  apiv2.SizeSmall,
				Redis: map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {ManagedRedis: &apiv2.ManagedRedisSpec{Sentinel: apiv2.RedisSentinelSpec{Enabled: boolPtr(false)}}}},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis.Sentinel.Enabled).To(g.HaveValue(g.BeFalse()))
	})

	It("does not apply defaults when External is present", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Redis: map[string]apiv2.RedisSpec{
					apiv2.DefaultInstanceName: {ExternalRedis: &apiv2.RedisConnection{}},
				},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis).To(g.BeNil())
	})
})
