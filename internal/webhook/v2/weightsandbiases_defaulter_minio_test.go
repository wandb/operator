package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiasesCustomDefaulter - Minio", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
	})

	It("defaults Minio namespace to the parent namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec:       apiv2.WeightsAndBiasesSpec{Minio: apiv2.MinioSpec{ManagedMinio: &apiv2.ManagedMinioSpec{}}},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Minio.ManagedMinio.Namespace).To(g.Equal("test-namespace"))
	})

	It("preserves a custom Minio namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Minio: apiv2.MinioSpec{ManagedMinio: &apiv2.ManagedMinioSpec{Namespace: "custom-minio-namespace"}},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Minio.ManagedMinio.Namespace).To(g.Equal("custom-minio-namespace"))
	})

	It("does not mutate unrelated Minio fields", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Minio: apiv2.MinioSpec{
					ManagedMinio: &apiv2.ManagedMinioSpec{
						StorageSize: "50Gi",
						Replicas:    4,
						Config:      apiv2.MinioConfig{MinioBrowserSetting: "off", RootUser: "custom-admin"},
					},
				},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Minio.ManagedMinio.StorageSize).To(g.Equal("50Gi"))
		g.Expect(wandb.Spec.Minio.ManagedMinio.Replicas).To(g.Equal(int32(4)))
		g.Expect(wandb.Spec.Minio.ManagedMinio.Config.MinioBrowserSetting).To(g.Equal("off"))
		g.Expect(wandb.Spec.Minio.ManagedMinio.Config.RootUser).To(g.Equal("custom-admin"))
	})

	It("does not apply defaults when ManagedMinio is nil", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec:       apiv2.WeightsAndBiasesSpec{},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Minio.ManagedMinio).To(g.BeNil())
	})
})
