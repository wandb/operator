package v2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("BuildMinioSpec", func() {
	Describe("Config merging", func() {
		Context("when both Config values are nil", func() {
			It("should result in nil Config", func() {
				actual := v2.WBMinioSpec{Config: nil}
				defaults := v2.WBMinioSpec{Config: nil}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).To(BeNil())
			})
		})

		Context("when actual Config is nil", func() {
			It("should use default Config", func() {
				actual := v2.WBMinioSpec{Config: nil}
				defaults := v2.WBMinioSpec{
					Config: &v2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("500m"),
							},
						},
					},
				}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
			})
		})

		Context("when both Config values exist", func() {
			It("should merge resources with actual taking precedence", func() {
				actual := v2.WBMinioSpec{
					Config: &v2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
						},
					},
				}
				defaults := v2.WBMinioSpec{
					Config: &v2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("1")))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))
			})
		})
	})

	Describe("Replicas field", func() {
		It("should always use actual Replicas", func() {
			actual := v2.WBMinioSpec{Replicas: 5}
			defaults := v2.WBMinioSpec{Replicas: 3}

			result, err := BuildMinioSpec(actual, defaults)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Replicas).To(Equal(int32(5)))
		})
	})

	Describe("Enabled field", func() {
		It("should always use actual Enabled", func() {
			actual := v2.WBMinioSpec{Enabled: false}
			defaults := v2.WBMinioSpec{Enabled: true}

			result, err := BuildMinioSpec(actual, defaults)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Enabled).To(BeFalse())
		})
	})
})

var _ = Describe("BuildMinioDefaults", func() {
	Context("when profile is Dev", func() {
		It("should return dev defaults", func() {
			spec, err := BuildMinioDefaults(v2.WBSizeDev)
			Expect(err).ToNot(HaveOccurred())
			Expect(spec.Enabled).To(BeTrue())
			Expect(spec.StorageSize).To(Equal(devMinioStorageSize))
			Expect(spec.Config).To(BeNil())
		})
	})

	Context("when profile is Small", func() {
		It("should return small defaults with resources", func() {
			spec, err := BuildMinioDefaults(v2.WBSizeSmall)
			Expect(err).ToNot(HaveOccurred())
			Expect(spec.Enabled).To(BeTrue())
			Expect(spec.StorageSize).To(Equal(smallMinioStorageSize))
			Expect(spec.Config).ToNot(BeNil())
			Expect(spec.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(smallMinioCpuRequest)))
		})
	})

	Context("when profile is invalid", func() {
		It("should return error", func() {
			_, err := BuildMinioDefaults(v2.WBSize("invalid"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported size for Minio"))
		})
	})
})
