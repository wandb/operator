package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiases multi-instance infra", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
		validator WeightsAndBiasesCustomValidator
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
		validator = WeightsAndBiasesCustomValidator{}
	})

	It("seeds a managed default instance when the map is empty", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wb", Namespace: "ns"},
		}

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())

		mysql, ok := wandb.Spec.MySQL[apiv2.DefaultInstanceName]
		g.Expect(ok).To(g.BeTrue())
		g.Expect(mysql.ManagedMysql).ToNot(g.BeNil())
		g.Expect(mysql.ManagedMysql.Name).To(g.Equal("wb-mysql"))
	})

	It("names the default instance plainly and keys other instances before the suffix", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wb", Namespace: "ns"},
			Spec: apiv2.WeightsAndBiasesSpec{
				MySQL: map[string]apiv2.MySQLSpec{
					apiv2.DefaultInstanceName: {ManagedMysql: &apiv2.ManagedMysqlSpec{}},
					"analytics":               {ManagedMysql: &apiv2.ManagedMysqlSpec{}},
				},
			},
		}

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())

		g.Expect(wandb.Spec.MySQL[apiv2.DefaultInstanceName].ManagedMysql.Name).To(g.Equal("wb-mysql"))
		g.Expect(wandb.Spec.MySQL["analytics"].ManagedMysql.Name).To(g.Equal("wb-analytics-mysql"))
		g.Expect(wandb.Spec.MySQL["analytics"].ManagedMysql.Namespace).To(g.Equal("ns"))
	})

	It("rejects instances defined without a default instance", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wb", Namespace: "ns"},
			Spec: apiv2.WeightsAndBiasesSpec{
				MySQL: map[string]apiv2.MySQLSpec{
					"analytics": {ManagedMysql: &apiv2.ManagedMysqlSpec{Name: "wb-mysql-analytics", Namespace: "ns"}},
				},
			},
		}

		_, err := validator.ValidateCreate(ctx, wandb)
		g.Expect(err).To(g.HaveOccurred())
		g.Expect(err.Error()).To(g.ContainSubstring("default"))
	})

	It("accepts multiple instances when a default is present", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wb", Namespace: "ns"},
			Spec: apiv2.WeightsAndBiasesSpec{
				MySQL: map[string]apiv2.MySQLSpec{
					apiv2.DefaultInstanceName: {ManagedMysql: &apiv2.ManagedMysqlSpec{Name: "wb-mysql", Namespace: "ns"}},
					"analytics":               {ManagedMysql: &apiv2.ManagedMysqlSpec{Name: "wb-mysql-analytics", Namespace: "ns"}},
				},
			},
		}

		_, err := validator.ValidateCreate(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
	})
})
