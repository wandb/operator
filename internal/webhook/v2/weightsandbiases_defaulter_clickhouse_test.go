package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
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
