package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/utils"
)

var _ = Describe("License", func() {
	Describe("GetLicense", func() {
		Context("when the license is set", func() {
			It("should return the license", func() {
				testSpec := &spec.Spec{
					Values: map[string]interface{}{
						"global": map[string]interface{}{
							"license": "test-license",
						},
					},
				}
				Expect(utils.GetLicense(testSpec)).To(Equal("test-license"))
			})
		})
		Context("when the license is not set", func() {
			It("should return an empty string", func() {
				testSpec := &spec.Spec{
					Values: map[string]interface{}{
						"global": map[string]interface{}{},
					},
				}
				Expect(utils.GetLicense(testSpec)).To(Equal(""))
			})
		})
	})
})
