package v2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("BuildMinioSpec", func() {
	Describe("Config merging", func() {
		Context("when both Config values are nil", func() {
			It("should result in nil Config", func() {
				actual := apiv2.WBMinioSpec{Config: nil}
				defaults := apiv2.WBMinioSpec{Config: nil}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).To(BeNil())
			})
		})

		Context("when actual Config is nil", func() {
			It("should use default Config", func() {
				defaultCpuRequest := resource.MustParse("500m")

				actual := apiv2.WBMinioSpec{Config: nil}
				defaults := apiv2.WBMinioSpec{
					Config: &apiv2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: defaultCpuRequest,
							},
						},
					},
				}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultCpuRequest))
			})
		})

		Context("when default Config is nil", func() {
			It("should use actual Config", func() {
				actualCpuRequest := resource.MustParse("1")

				actual := apiv2.WBMinioSpec{
					Config: &apiv2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: actualCpuRequest,
							},
						},
					},
				}
				defaults := apiv2.WBMinioSpec{Config: nil}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(actualCpuRequest))
			})
		})

		Context("when both Config values exist", func() {
			It("should merge resources with actual taking precedence", func() {
				actualCpuRequest := resource.MustParse("1")
				defaultCpuRequest := resource.MustParse("500m")
				defaultMemoryRequest := resource.MustParse("1Gi")

				actual := apiv2.WBMinioSpec{
					Config: &apiv2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: actualCpuRequest,
							},
						},
					},
				}
				defaults := apiv2.WBMinioSpec{
					Config: &apiv2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    defaultCpuRequest,
								corev1.ResourceMemory: defaultMemoryRequest,
							},
						},
					},
				}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(actualCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultMemoryRequest))
			})
		})
	})

	Describe("StorageSize merging", func() {
		Context("when actual StorageSize is empty", func() {
			It("should use default StorageSize", func() {
				defaultStorageSize := "10Gi"

				actual := apiv2.WBMinioSpec{StorageSize: ""}
				defaults := apiv2.WBMinioSpec{StorageSize: defaultStorageSize}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(defaultStorageSize))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actualStorageSize := "50Gi"
				defaultStorageSize := "10Gi"

				actual := apiv2.WBMinioSpec{StorageSize: actualStorageSize}
				defaults := apiv2.WBMinioSpec{StorageSize: defaultStorageSize}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(actualStorageSize))
			})
		})
	})

	Describe("Namespace merging", func() {
		Context("when actual Namespace is empty", func() {
			It("should use default Namespace", func() {
				defaultNamespace := "default-namespace"

				actual := apiv2.WBMinioSpec{Namespace: ""}
				defaults := apiv2.WBMinioSpec{Namespace: defaultNamespace}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(defaultNamespace))
			})
		})

		Context("when actual Namespace is set", func() {
			It("should use actual Namespace", func() {
				actualNamespace := "custom-namespace"
				defaultNamespace := "default-namespace"

				actual := apiv2.WBMinioSpec{Namespace: actualNamespace}
				defaults := apiv2.WBMinioSpec{Namespace: defaultNamespace}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(actualNamespace))
			})
		})
	})

	Describe("Replicas field", func() {
		It("should always use actual Replicas value", func() {
			actualReplicas := int32(5)
			defaultReplicas := int32(3)

			actual := apiv2.WBMinioSpec{Replicas: actualReplicas}
			defaults := apiv2.WBMinioSpec{Replicas: defaultReplicas}

			result, err := BuildMinioSpec(actual, defaults)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Replicas).To(Equal(actualReplicas))
		})
	})

	Describe("Enabled field", func() {
		Context("when actual Enabled is true", func() {
			It("should use actual value regardless of default", func() {
				actual := apiv2.WBMinioSpec{Enabled: true}
				defaults := apiv2.WBMinioSpec{Enabled: false}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
			})
		})

		Context("when actual Enabled is false", func() {
			It("should use actual value regardless of default", func() {
				actual := apiv2.WBMinioSpec{Enabled: false}
				defaults := apiv2.WBMinioSpec{Enabled: true}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
			})
		})
	})
})

var _ = Describe("BuildMinioDefaults", func() {
	const testOwnerNamespace = "test-namespace"

	Context("when profile is Dev", func() {
		It("should return complete dev defaults", func() {
			spec, err := BuildMinioDefaults(apiv2.WBSizeDev, testOwnerNamespace)
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.Enabled).To(BeTrue())
			Expect(spec.Namespace).To(Equal(testOwnerNamespace))
			Expect(spec.StorageSize).To(Equal(DevMinioStorageSize))
			Expect(spec.Config).To(BeNil())
		})
	})

	Context("when profile is Small", func() {
		It("should return complete small defaults with all resource fields", func() {
			spec, err := BuildMinioDefaults(apiv2.WBSizeSmall, testOwnerNamespace)
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.Enabled).To(BeTrue())
			Expect(spec.Namespace).To(Equal(testOwnerNamespace))
			Expect(spec.StorageSize).To(Equal(SmallMinioStorageSize))
			Expect(spec.Config).ToNot(BeNil())

			expectedCpuRequest, _ := resource.ParseQuantity(SmallMinioCpuRequest)
			expectedCpuLimit, _ := resource.ParseQuantity(SmallMinioCpuLimit)
			expectedMemoryRequest, _ := resource.ParseQuantity(SmallMinioMemoryRequest)
			expectedMemoryLimit, _ := resource.ParseQuantity(SmallMinioMemoryLimit)

			Expect(spec.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(expectedCpuRequest))
			Expect(spec.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(expectedCpuLimit))
			Expect(spec.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(expectedMemoryRequest))
			Expect(spec.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(expectedMemoryLimit))
		})
	})

	Context("when profile is invalid", func() {
		It("should return error", func() {
			_, err := BuildMinioDefaults(apiv2.WBSize("invalid"), testOwnerNamespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported size for Minio"))
		})
	})
})
