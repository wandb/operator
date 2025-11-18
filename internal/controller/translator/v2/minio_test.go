package v2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	defaultSmallMinioCpuRequest    = resource.MustParse(model.SmallMinioCpuRequest)
	defaultSmallMinioCpuLimit      = resource.MustParse(model.SmallMinioCpuLimit)
	defaultSmallMinioMemoryRequest = resource.MustParse(model.SmallMinioMemoryRequest)
	defaultSmallMinioMemoryLimit   = resource.MustParse(model.SmallMinioMemoryLimit)

	overrideMinioStorageSize   = "50Gi"
	overrideMinioNamespace     = "custom-namespace"
	overrideMinioReplicas      = int32(5)
	overrideMinioEnabled       = false
	overrideMinioCpuRequest    = resource.MustParse("1")
	overrideMinioCpuLimit      = resource.MustParse("2")
	overrideMinioMemoryRequest = resource.MustParse("2Gi")
	overrideMinioMemoryLimit   = resource.MustParse("4Gi")
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
				actual := apiv2.WBMinioSpec{Config: nil}
				defaults := apiv2.WBMinioSpec{
					Config: &apiv2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: defaultSmallMinioCpuRequest,
							},
						},
					},
				}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallMinioCpuRequest))
			})
		})

		Context("when default Config is nil", func() {
			It("should use actual Config", func() {
				actual := apiv2.WBMinioSpec{
					Config: &apiv2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: overrideMinioCpuRequest,
							},
						},
					},
				}
				defaults := apiv2.WBMinioSpec{Config: nil}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMinioCpuRequest))
			})
		})

		Context("when both Config values exist", func() {
			It("should merge resources with actual taking precedence", func() {
				actual := apiv2.WBMinioSpec{
					Config: &apiv2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: overrideMinioCpuRequest,
							},
						},
					},
				}
				defaults := apiv2.WBMinioSpec{
					Config: &apiv2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallMinioCpuRequest,
								corev1.ResourceMemory: defaultSmallMinioMemoryRequest,
							},
						},
					},
				}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMinioCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallMinioMemoryRequest))
			})
		})
	})

	Describe("StorageSize merging", func() {
		Context("when actual StorageSize is empty", func() {
			It("should use default StorageSize", func() {
				actual := apiv2.WBMinioSpec{StorageSize: ""}
				defaults := apiv2.WBMinioSpec{StorageSize: model.SmallMinioStorageSize}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(model.SmallMinioStorageSize))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actual := apiv2.WBMinioSpec{StorageSize: overrideMinioStorageSize}
				defaults := apiv2.WBMinioSpec{StorageSize: model.SmallMinioStorageSize}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(overrideMinioStorageSize))
			})
		})
	})

	Describe("Namespace merging", func() {
		Context("when actual Namespace is empty", func() {
			It("should use default Namespace", func() {
				actual := apiv2.WBMinioSpec{Namespace: ""}
				defaults := apiv2.WBMinioSpec{Namespace: overrideMinioNamespace}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideMinioNamespace))
			})
		})

		Context("when actual Namespace is set", func() {
			It("should use actual Namespace", func() {
				actual := apiv2.WBMinioSpec{Namespace: overrideMinioNamespace}
				defaults := apiv2.WBMinioSpec{Namespace: "default-namespace"}

				result, err := BuildMinioSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideMinioNamespace))
			})
		})
	})

	Describe("Replicas field", func() {
		It("should always use actual Replicas value", func() {
			actual := apiv2.WBMinioSpec{Replicas: overrideMinioReplicas}
			defaults := apiv2.WBMinioSpec{Replicas: 3}

			result, err := BuildMinioSpec(actual, defaults)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Replicas).To(Equal(overrideMinioReplicas))
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

var _ = Describe("InfraConfigBuilder.AddMinioSpec", func() {
	const testOwnerNamespace = "test-namespace"

	Context("when adding dev size spec", func() {
		It("should merge actual with dev defaults from model", func() {
			actual := apiv2.WBMinioSpec{
				Enabled: true,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeDev)
			result := builder.AddMinioSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMinio).ToNot(BeNil())
			Expect(builder.mergedMinio.Enabled).To(BeTrue())
			Expect(builder.mergedMinio.Namespace).To(Equal(testOwnerNamespace))
			Expect(builder.mergedMinio.StorageSize).To(Equal(model.DevMinioStorageSize))
		})
	})

	Context("when adding small size spec with all overrides", func() {
		It("should use all overrides and verify they differ from defaults", func() {
			actual := apiv2.WBMinioSpec{
				Enabled:     overrideMinioEnabled,
				Namespace:   overrideMinioNamespace,
				StorageSize: overrideMinioStorageSize,
				Replicas:    overrideMinioReplicas,
				Config: &apiv2.WBMinioConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideMinioCpuRequest,
							corev1.ResourceMemory: overrideMinioMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideMinioCpuLimit,
							corev1.ResourceMemory: overrideMinioMemoryLimit,
						},
					},
				},
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMinioSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMinio).ToNot(BeNil())

			Expect(builder.mergedMinio.Enabled).To(Equal(overrideMinioEnabled))
			Expect(builder.mergedMinio.Enabled).ToNot(Equal(true))

			Expect(builder.mergedMinio.Namespace).To(Equal(overrideMinioNamespace))
			Expect(builder.mergedMinio.Namespace).ToNot(Equal(testOwnerNamespace))

			Expect(builder.mergedMinio.StorageSize).To(Equal(overrideMinioStorageSize))
			Expect(builder.mergedMinio.StorageSize).ToNot(Equal(model.SmallMinioStorageSize))

			Expect(builder.mergedMinio.Replicas).To(Equal(overrideMinioReplicas))
			Expect(builder.mergedMinio.Replicas).ToNot(Equal(int32(3)))

			Expect(builder.mergedMinio.Config).ToNot(BeNil())
			Expect(builder.mergedMinio.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMinioCpuRequest))
			Expect(builder.mergedMinio.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMinioCpuRequest))

			Expect(builder.mergedMinio.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideMinioMemoryRequest))
			Expect(builder.mergedMinio.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMinioMemoryRequest))

			Expect(builder.mergedMinio.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideMinioCpuLimit))
			Expect(builder.mergedMinio.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMinioCpuLimit))

			Expect(builder.mergedMinio.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMinioMemoryLimit))
			Expect(builder.mergedMinio.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMinioMemoryLimit))
		})
	})

	Context("when adding small size spec with storage override only", func() {
		It("should use override storage and verify it differs from default", func() {
			actual := apiv2.WBMinioSpec{
				Enabled:     true,
				StorageSize: overrideMinioStorageSize,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMinioSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMinio.StorageSize).To(Equal(overrideMinioStorageSize))
			Expect(builder.mergedMinio.StorageSize).ToNot(Equal(model.SmallMinioStorageSize))
			Expect(builder.mergedMinio.Config).ToNot(BeNil())
		})
	})

	Context("when adding small size spec with namespace override only", func() {
		It("should use override namespace and verify it differs from default", func() {
			actual := apiv2.WBMinioSpec{
				Enabled:   true,
				Namespace: overrideMinioNamespace,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMinioSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMinio.Namespace).To(Equal(overrideMinioNamespace))
			Expect(builder.mergedMinio.Namespace).ToNot(Equal(testOwnerNamespace))
		})
	})

	Context("when adding small size spec with resource overrides only", func() {
		It("should use override resources and verify they differ from defaults", func() {
			actual := apiv2.WBMinioSpec{
				Enabled: true,
				Config: &apiv2.WBMinioConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideMinioCpuRequest,
							corev1.ResourceMemory: overrideMinioMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideMinioCpuLimit,
							corev1.ResourceMemory: overrideMinioMemoryLimit,
						},
					},
				},
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMinioSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMinio.Config).ToNot(BeNil())

			Expect(builder.mergedMinio.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMinioCpuRequest))
			Expect(builder.mergedMinio.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMinioCpuRequest))

			Expect(builder.mergedMinio.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideMinioMemoryRequest))
			Expect(builder.mergedMinio.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMinioMemoryRequest))

			Expect(builder.mergedMinio.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideMinioCpuLimit))
			Expect(builder.mergedMinio.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMinioCpuLimit))

			Expect(builder.mergedMinio.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMinioMemoryLimit))
			Expect(builder.mergedMinio.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMinioMemoryLimit))
		})
	})

	Context("when size is invalid", func() {
		It("should append error to builder", func() {
			actual := apiv2.WBMinioSpec{Enabled: true}
			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSize("invalid"))
			result := builder.AddMinioSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).ToNot(BeEmpty())
		})
	})
})

var _ = Describe("TranslateMinioConfig", func() {
	Context("when translating a complete Minio config", func() {
		It("should correctly map all fields to WBMinioSpec", func() {
			config := model.MinioConfig{
				Enabled:     true,
				Namespace:   overrideMinioNamespace,
				StorageSize: overrideMinioStorageSize,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    overrideMinioCpuRequest,
						corev1.ResourceMemory: overrideMinioMemoryRequest,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    overrideMinioCpuLimit,
						corev1.ResourceMemory: overrideMinioMemoryLimit,
					},
				},
			}

			spec := TranslateMinioConfig(config)

			Expect(spec.Enabled).To(Equal(config.Enabled))
			Expect(spec.Namespace).To(Equal(config.Namespace))
			Expect(spec.StorageSize).To(Equal(config.StorageSize))
			Expect(spec.Config).ToNot(BeNil())
			Expect(spec.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMinioCpuRequest))
			Expect(spec.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMinioMemoryLimit))
		})
	})
})
