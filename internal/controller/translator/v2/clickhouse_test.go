package v2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("BuildClickHouseSpec", func() {
	Describe("Config merging", func() {
		Context("when both Config values are nil", func() {
			It("should result in nil Config", func() {
				actual := v2.WBClickHouseSpec{Config: nil}
				defaults := v2.WBClickHouseSpec{Config: nil}

				result, err := BuildClickHouseSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).To(BeNil())
			})
		})

		Context("when actual Config is nil", func() {
			It("should use default Config", func() {
				actual := v2.WBClickHouseSpec{Config: nil}
				defaults := v2.WBClickHouseSpec{
					Config: &v2.WBClickHouseConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("500m"),
							},
						},
					},
				}

				result, err := BuildClickHouseSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
			})
		})

		Context("when both Config values exist", func() {
			It("should merge resources with actual taking precedence", func() {
				actual := v2.WBClickHouseSpec{
					Config: &v2.WBClickHouseConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
						},
					},
				}
				defaults := v2.WBClickHouseSpec{
					Config: &v2.WBClickHouseConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				}

				result, err := BuildClickHouseSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("2")))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))
			})
		})
	})

	Describe("Version merging", func() {
		Context("when actual Version is empty", func() {
			It("should use default Version", func() {
				actual := v2.WBClickHouseSpec{Version: ""}
				defaults := v2.WBClickHouseSpec{Version: "23.8"}

				result, err := BuildClickHouseSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Version).To(Equal("23.8"))
			})
		})

		Context("when actual Version is set", func() {
			It("should use actual Version", func() {
				actual := v2.WBClickHouseSpec{Version: "24.1"}
				defaults := v2.WBClickHouseSpec{Version: "23.8"}

				result, err := BuildClickHouseSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Version).To(Equal("24.1"))
			})
		})
	})

	Describe("Replicas field", func() {
		It("should always use actual Replicas", func() {
			actual := v2.WBClickHouseSpec{Replicas: 5}
			defaults := v2.WBClickHouseSpec{Replicas: 3}

			result, err := BuildClickHouseSpec(actual, defaults)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Replicas).To(Equal(int32(5)))
		})
	})

	Describe("Enabled field", func() {
		It("should always use actual Enabled", func() {
			actual := v2.WBClickHouseSpec{Enabled: false}
			defaults := v2.WBClickHouseSpec{Enabled: true}

			result, err := BuildClickHouseSpec(actual, defaults)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Enabled).To(BeFalse())
		})
	})
})

var _ = Describe("BuildClickHouseDefaults", func() {
	Context("when profile is Dev", func() {
		It("should return dev defaults with 1 replica", func() {
			spec, err := BuildClickHouseDefaults(v2.WBSizeDev, testingOwnerNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(spec.Enabled).To(BeTrue())
			Expect(spec.Version).To(Equal(ClickHouseVersion))
			Expect(spec.StorageSize).To(Equal(DevClickHouseStorageSize))
			Expect(spec.Replicas).To(Equal(int32(1)))
			Expect(spec.Config).To(BeNil())
		})
	})

	Context("when profile is Small", func() {
		It("should return small defaults with 3 replicas and resources", func() {
			spec, err := BuildClickHouseDefaults(v2.WBSizeSmall, testingOwnerNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(spec.Enabled).To(BeTrue())
			Expect(spec.Version).To(Equal(ClickHouseVersion))
			Expect(spec.StorageSize).To(Equal(SmallClickHouseStorageSize))
			Expect(spec.Replicas).To(Equal(int32(3)))
			Expect(spec.Config).ToNot(BeNil())
			Expect(spec.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(SmallClickHouseCpuRequest)))
		})
	})

	Context("when profile is invalid", func() {
		It("should return error", func() {
			_, err := BuildClickHouseDefaults(v2.WBSize("invalid"), testingOwnerNamespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported size for ClickHouse"))
		})
	})
})
