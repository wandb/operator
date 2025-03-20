package utils_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("License", func() {
	Describe("GetLicense", func() {
		Context("when the license is set via values", func() {
			It("should return the license", func() {
				testSpec := &spec.Spec{
					Values: map[string]interface{}{
						"global": map[string]interface{}{
							"license": "test-license",
						},
					},
				}
				testWandb := &v1.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "test-name",
					},
				}
				Expect(utils.GetLicense(context.Background(), k8sClient, testWandb, testSpec)).To(Equal("test-license"))
			})
		})
		Context("when the license secret is specified but not present", func() {
			It("should return an empty string", func() {
				testSpec := &spec.Spec{
					Values: map[string]interface{}{
						"global": map[string]interface{}{
							"licenseSecret": map[string]interface{}{
								"name": "test-secret-name",
								"key":  "license",
							},
						},
					},
				}
				testWandb := &v1.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "test-name",
					},
				}
				Expect(utils.GetLicense(context.Background(), k8sClient, testWandb, testSpec)).To(Equal(""))
			})
		})
		Context("when the license is set via secret", func() {
			It("should return the license", func() {
				licenseSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "test-secret-name",
					},
					Data: map[string][]byte{
						"license": []byte("test-license"),
					},
				}
				err := k8sClient.Create(context.Background(), licenseSecret)
				Expect(err).NotTo(HaveOccurred())
				testSpec := &spec.Spec{
					Values: map[string]interface{}{
						"global": map[string]interface{}{
							"licenseSecret": map[string]interface{}{
								"name": "test-secret-name",
								"key":  "license",
							},
						},
					},
				}
				testWandb := &v1.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "test-name",
					},
				}
				Expect(utils.GetLicense(context.Background(), k8sClient, testWandb, testSpec)).To(Equal("test-license"))
			})
		})
		Context("when the license is not set", func() {
			It("should return an empty string", func() {
				testSpec := &spec.Spec{
					Values: map[string]interface{}{
						"global": map[string]interface{}{},
					},
				}
				testWandb := &v1.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "test-name",
					},
				}
				Expect(utils.GetLicense(context.Background(), k8sClient, testWandb, testSpec)).To(Equal(""))
			})
		})
	})
})
