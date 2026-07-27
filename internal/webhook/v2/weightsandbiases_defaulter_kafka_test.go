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
			Spec:       apiv2.WeightsAndBiasesSpec{Kafka: apiv2.KafkaSpec{ManagedKafka: &apiv2.ManagedKafkaSpec{}}},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Kafka.ManagedKafka.Namespace).To(g.Equal("test-namespace"))
	})

	It("preserves a custom Kafka namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Kafka: apiv2.KafkaSpec{ManagedKafka: &apiv2.ManagedKafkaSpec{Namespace: "custom-kafka-namespace"}},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Kafka.ManagedKafka.Namespace).To(g.Equal("custom-kafka-namespace"))
	})

	It("does not mutate unrelated Kafka fields", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Kafka: apiv2.KafkaSpec{ManagedKafka: &apiv2.ManagedKafkaSpec{StorageSize: "20Gi", Replicas: 5}},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Kafka.ManagedKafka.StorageSize).To(g.Equal("20Gi"))
		g.Expect(wandb.Spec.Kafka.ManagedKafka.Replicas).To(g.Equal(int32(5)))
	})

	It("creates managed Kafka defaults when Kafka is unset", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec:       apiv2.WeightsAndBiasesSpec{},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Kafka.ManagedKafka).ToNot(g.BeNil())
		g.Expect(wandb.Spec.Kafka.ManagedKafka.Name).To(g.Equal("test-wandb-kafka"))
		g.Expect(wandb.Spec.Kafka.ManagedKafka.Namespace).To(g.Equal("test-namespace"))
		g.Expect(wandb.Spec.Kafka.ManagedKafka.ServiceAccount.Create).ToNot(g.BeNil())
		g.Expect(*wandb.Spec.Kafka.ManagedKafka.ServiceAccount.Create).To(g.BeTrue())
		g.Expect(wandb.Spec.Kafka.ManagedKafka.ServiceAccount.ServiceAccountName).To(g.Equal("test-wandb-kafka"))
	})
})
