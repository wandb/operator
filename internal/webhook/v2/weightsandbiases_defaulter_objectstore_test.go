package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiasesCustomDefaulter - ObjectStore", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
	})

	It("defaults ObjectStore namespace to the parent namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec:       apiv2.WeightsAndBiasesSpec{ObjectStore: apiv2.ObjectStoreSpec{ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{}}},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.ObjectStore.ManagedObjectStore.Namespace).To(g.Equal("test-namespace"))
	})

	It("preserves a custom ObjectStore namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				ObjectStore: apiv2.ObjectStoreSpec{ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{Namespace: "custom-objectstore-namespace"}},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.ObjectStore.ManagedObjectStore.Namespace).To(g.Equal("custom-objectstore-namespace"))
	})

	It("does not mutate unrelated ObjectStore fields", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				ObjectStore: apiv2.ObjectStoreSpec{
					ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{
						StorageSize: "50Gi",
						Replicas:    4,
						Config:      apiv2.ObjectStoreConfig{AccessKey: "custom-admin"},
					},
				},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.ObjectStore.ManagedObjectStore.StorageSize).To(g.Equal("50Gi"))
		g.Expect(wandb.Spec.ObjectStore.ManagedObjectStore.Replicas).To(g.Equal(int32(4)))
		g.Expect(wandb.Spec.ObjectStore.ManagedObjectStore.Config.AccessKey).To(g.Equal("custom-admin"))
	})

	It("does not apply defaults when ExternalObjectStore is present", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				ObjectStore: apiv2.ObjectStoreSpec{
					ExternalObjectStore: &apiv2.ObjectStoreConnection{},
				},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.ObjectStore.ManagedObjectStore).To(g.BeNil())
	})
})
