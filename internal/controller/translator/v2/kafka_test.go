package v2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("BuildKafkaSpec", func() {
	Describe("Config merging", func() {
		Context("when both Config values are nil", func() {
			It("should result in nil Config", func() {
				actual := v2.WBKafkaSpec{Config: nil}
				defaults := v2.WBKafkaSpec{Config: nil}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).To(BeNil())
			})
		})

		Context("when actual Config is nil", func() {
			It("should use default Config", func() {
				actual := v2.WBKafkaSpec{Config: nil}
				defaults := v2.WBKafkaSpec{
					Config: &v2.WBKafkaConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
			})
		})

		Context("when both Config values exist", func() {
			It("should merge resources with actual taking precedence", func() {
				actual := v2.WBKafkaSpec{
					Config: &v2.WBKafkaConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("750m")},
						},
					},
				}
				defaults := v2.WBKafkaSpec{
					Config: &v2.WBKafkaConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("750m")))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))
			})
		})
	})

	Describe("StorageSize and Namespace merging", func() {
		It("should merge correctly", func() {
			actual := v2.WBKafkaSpec{StorageSize: "20Gi"}
			defaults := v2.WBKafkaSpec{StorageSize: "10Gi", Namespace: "default"}

			result, err := BuildKafkaSpec(actual, defaults)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.StorageSize).To(Equal("20Gi"))
			Expect(result.Namespace).To(Equal("default"))
		})
	})

	Describe("Enabled field", func() {
		It("should always use actual Enabled", func() {
			actual := v2.WBKafkaSpec{Enabled: false}
			defaults := v2.WBKafkaSpec{Enabled: true}

			result, err := BuildKafkaSpec(actual, defaults)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Enabled).To(BeFalse())
		})
	})
})

var _ = Describe("BuildKafkaDefaults", func() {
	Context("when profile is Dev", func() {
		It("should return dev defaults", func() {
			spec, err := BuildKafkaDefaults(v2.WBSizeDev, testingOwnerNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(spec.Enabled).To(BeTrue())
			Expect(spec.StorageSize).To(Equal(DevKafkaStorageSize))
			Expect(spec.Config).To(BeNil())
		})
	})

	Context("when profile is Small", func() {
		It("should return small defaults with resources", func() {
			spec, err := BuildKafkaDefaults(v2.WBSizeSmall, testingOwnerNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(spec.Enabled).To(BeTrue())
			Expect(spec.StorageSize).To(Equal(SmallKafkaStorageSize))
			Expect(spec.Config).ToNot(BeNil())
			Expect(spec.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(SmallKafkaCpuRequest)))
		})
	})

	Context("when profile is invalid", func() {
		It("should return error", func() {
			_, err := BuildKafkaDefaults(v2.WBSize("invalid"), testingOwnerNamespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported size for Kafka"))
		})
	})
})
