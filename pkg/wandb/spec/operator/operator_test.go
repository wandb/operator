package operator_test

import (
	"github.com/wandb/operator/pkg/wandb/spec/operator"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Operator", func() {
	Describe("Defaults", func() {
		It("should return a Spec with default values", func() {
			wandb := &v1.WeightsAndBiases{}
			scheme := runtime.NewScheme()
			_ = v1.AddToScheme(scheme)
			expectedSpec := &spec.Spec{
				Chart: nil,
				Values: map[string]interface{}{
					"global": map[string]interface{}{
						"operator": map[string]interface{}{
							"apiVersion": "apps.wandb.com/v1",
							"namespace":  "wandb",
						},
					},
				},
			}

			Expect(operator.Defaults(wandb, scheme)).To(Equal(expectedSpec))
		})

		It("should use OPERATOR_NAMESPACE environment variable if set", func() {
			wandb := &v1.WeightsAndBiases{}
			scheme := runtime.NewScheme()
			_ = v1.AddToScheme(scheme)
			_ = os.Setenv("OPERATOR_NAMESPACE", "custom-namespace")
			defer func() {
				_ = os.Unsetenv("OPERATOR_NAMESPACE")
			}()
			expectedSpec := &spec.Spec{
				Chart: nil,
				Values: map[string]interface{}{
					"global": map[string]interface{}{
						"operator": map[string]interface{}{
							"apiVersion": "apps.wandb.com/v1",
							"namespace":  "custom-namespace",
						},
					},
				},
			}

			Expect(operator.Defaults(wandb, scheme)).To(Equal(expectedSpec))
		})
	})

	Describe("Spec", func() {
		It("should return the spec for the given CRD", func() {
			wandb := &v1.WeightsAndBiases{
				Spec: v1.WeightsAndBiasesSpec{
					Chart:  v1.Object{Object: map[string]interface{}{"Path": "test-chart-value"}},
					Values: v1.Object{Object: map[string]interface{}{"test-values-key": "test-values-value"}},
				},
			}
			expectedSpec := &spec.Spec{
				Chart:  &charts.LocalRelease{Path: "test-chart-value"},
				Values: map[string]interface{}{"test-values-key": "test-values-value"},
			}

			Expect(operator.Spec(wandb)).To(Equal(expectedSpec))
		})
	})
})
