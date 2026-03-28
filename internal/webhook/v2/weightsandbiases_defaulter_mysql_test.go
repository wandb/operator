package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiasesCustomDefaulter - MySQL", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
	})

	It("defaults MySQL namespace and deployment type", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec:       apiv2.WeightsAndBiasesSpec{MySQL: apiv2.MySQLSpec{ManagedMysql: &apiv2.ManagedMysqlSpec{}}},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.MySQL.ManagedMysql.Namespace).To(g.Equal("test-namespace"))
		g.Expect(wandb.Spec.MySQL.ManagedMysql.DeploymentType).To(g.Equal(apiv2.MySQLTypeMysql))
	})

	It("preserves custom MySQL namespace and deployment type", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				MySQL: apiv2.MySQLSpec{
					ManagedMysql: &apiv2.ManagedMysqlSpec{
						Namespace:      "custom-mysql-namespace",
						DeploymentType: apiv2.MySQLTypePercona,
					},
				},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.MySQL.ManagedMysql.Namespace).To(g.Equal("custom-mysql-namespace"))
		g.Expect(wandb.Spec.MySQL.ManagedMysql.DeploymentType).To(g.Equal(apiv2.MySQLTypePercona))
	})

	It("does not mutate unrelated MySQL fields", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				MySQL: apiv2.MySQLSpec{ManagedMysql: &apiv2.ManagedMysqlSpec{StorageSize: "50Gi"}},
			},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.MySQL.ManagedMysql.StorageSize).To(g.Equal("50Gi"))
	})

	It("does not apply defaults when ManagedMysql is nil", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec:       apiv2.WeightsAndBiasesSpec{},
		}

		err := defaulter.Default(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
		g.Expect(wandb.Spec.MySQL.ManagedMysql).To(g.BeNil())
	})
})
