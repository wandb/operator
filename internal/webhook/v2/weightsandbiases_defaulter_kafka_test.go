package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiasesCustomDefaulter - Kafka", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
	})

	It("defaults Kafka namespace to the parent namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec:       apiv2.WeightsAndBiasesSpec{Kafka: apiv2.KafkaSpec{InfraSpec: apiv2.InfraSpec{Enabled: true}}},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Kafka.Namespace).To(g.Equal("test-namespace"))
	})

	It("preserves a custom Kafka namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Kafka: apiv2.KafkaSpec{InfraSpec: apiv2.InfraSpec{Enabled: true}, Namespace: "custom-kafka-namespace"},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Kafka.Namespace).To(g.Equal("custom-kafka-namespace"))
	})

	It("does not mutate unrelated Kafka fields", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Kafka: apiv2.KafkaSpec{InfraSpec: apiv2.InfraSpec{Enabled: true}, StorageSize: "20Gi", Replicas: 5},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Kafka.StorageSize).To(g.Equal("20Gi"))
		g.Expect(wandb.Spec.Kafka.Replicas).To(g.Equal(int32(5)))
	})
})
