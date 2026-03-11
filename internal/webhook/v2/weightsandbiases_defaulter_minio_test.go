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
			Spec:       apiv2.WeightsAndBiasesSpec{Minio: apiv2.MinioSpec{InfraSpec: apiv2.InfraSpec{Enabled: true}}},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Minio.Namespace).To(g.Equal("test-namespace"))
	})

	It("preserves a custom Minio namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Minio: apiv2.MinioSpec{InfraSpec: apiv2.InfraSpec{Enabled: true}, Namespace: "custom-minio-namespace"},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Minio.Namespace).To(g.Equal("custom-minio-namespace"))
	})

	It("does not mutate unrelated Minio fields", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Minio: apiv2.MinioSpec{
					InfraSpec:   apiv2.InfraSpec{Enabled: true},
					StorageSize: "50Gi",
					Replicas:    4,
					Config:      apiv2.MinioConfig{MinioBrowserSetting: "off", RootUser: "custom-admin"},
				},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.Minio.StorageSize).To(g.Equal("50Gi"))
		g.Expect(wandb.Spec.Minio.Replicas).To(g.Equal(int32(4)))
		g.Expect(wandb.Spec.Minio.Config.MinioBrowserSetting).To(g.Equal("off"))
		g.Expect(wandb.Spec.Minio.Config.RootUser).To(g.Equal("custom-admin"))
	})
})
