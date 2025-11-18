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
	defaultSmallCHCpuRequest    = resource.MustParse(model.SmallClickHouseCpuRequest)
	defaultSmallCHCpuLimit      = resource.MustParse(model.SmallClickHouseCpuLimit)
	defaultSmallCHMemoryRequest = resource.MustParse(model.SmallClickHouseMemoryRequest)
	defaultSmallCHMemoryLimit   = resource.MustParse(model.SmallClickHouseMemoryLimit)

	overrideCHStorageSize   = "50Gi"
	overrideCHVersion       = "24.8"
	overrideCHReplicas      = int32(5)
	overrideCHNamespace     = "custom-namespace"
	overrideCHEnabled       = false
	overrideCHCpuRequest    = resource.MustParse("2")
	overrideCHCpuLimit      = resource.MustParse("4")
	overrideCHMemoryRequest = resource.MustParse("4Gi")
	overrideCHMemoryLimit   = resource.MustParse("8Gi")
)

var _ = Describe("BuildClickHouseSpec", func() {
	Describe("Config merging", func() {
		Context("when both Config values are nil", func() {
			It("should result in nil Config", func() {
				actual := apiv2.WBClickHouseSpec{Config: nil}
				defaults := apiv2.WBClickHouseSpec{Config: nil}

				result, err := BuildClickHouseSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).To(BeNil())
			})
		})

		Context("when actual Config is nil", func() {
			It("should use default Config", func() {
				actual := apiv2.WBClickHouseSpec{Config: nil}
				defaults := apiv2.WBClickHouseSpec{
					Config: &apiv2.WBClickHouseConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: defaultSmallCHCpuRequest,
							},
						},
					},
				}

				result, err := BuildClickHouseSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallCHCpuRequest))
			})
		})

		Context("when both Config values exist", func() {
			It("should merge resources with actual taking precedence", func() {
				actual := apiv2.WBClickHouseSpec{
					Config: &apiv2.WBClickHouseConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: overrideCHCpuRequest},
						},
					},
				}
				defaults := apiv2.WBClickHouseSpec{
					Config: &apiv2.WBClickHouseConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallCHCpuRequest,
								corev1.ResourceMemory: defaultSmallCHMemoryRequest,
							},
						},
					},
				}

				result, err := BuildClickHouseSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideCHCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallCHMemoryRequest))
			})
		})
	})

	Describe("Version merging", func() {
		Context("when actual Version is empty", func() {
			It("should use default Version", func() {
				actual := apiv2.WBClickHouseSpec{Version: ""}
				defaults := apiv2.WBClickHouseSpec{Version: model.ClickHouseVersion}

				result, err := BuildClickHouseSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Version).To(Equal(model.ClickHouseVersion))
			})
		})

		Context("when actual Version is set", func() {
			It("should use actual Version", func() {
				actual := apiv2.WBClickHouseSpec{Version: overrideCHVersion}
				defaults := apiv2.WBClickHouseSpec{Version: model.ClickHouseVersion}

				result, err := BuildClickHouseSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Version).To(Equal(overrideCHVersion))
			})
		})
	})

	Describe("Replicas field", func() {
		It("should always use actual Replicas", func() {
			actual := apiv2.WBClickHouseSpec{Replicas: overrideCHReplicas}
			defaults := apiv2.WBClickHouseSpec{Replicas: 3}

			result, err := BuildClickHouseSpec(actual, defaults)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Replicas).To(Equal(overrideCHReplicas))
		})
	})

	Describe("Enabled field", func() {
		It("should always use actual Enabled", func() {
			actual := apiv2.WBClickHouseSpec{Enabled: false}
			defaults := apiv2.WBClickHouseSpec{Enabled: true}

			result, err := BuildClickHouseSpec(actual, defaults)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Enabled).To(BeFalse())
		})
	})
})

var _ = Describe("InfraConfigBuilder.AddClickHouseSpec", func() {
	const testOwnerNamespace = "test-namespace"

	Context("when adding dev size spec", func() {
		It("should merge actual with dev defaults from model", func() {
			actual := apiv2.WBClickHouseSpec{
				Enabled:  true,
				Replicas: 1,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeDev)
			result := builder.AddClickHouseSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedClickHouse).ToNot(BeNil())
			Expect(builder.mergedClickHouse.Enabled).To(BeTrue())
			Expect(builder.mergedClickHouse.Namespace).To(Equal(testOwnerNamespace))
			Expect(builder.mergedClickHouse.StorageSize).To(Equal(model.DevClickHouseStorageSize))
			Expect(builder.mergedClickHouse.Replicas).To(Equal(int32(1)))
			Expect(builder.mergedClickHouse.Version).To(Equal(model.ClickHouseVersion))
		})
	})

	Context("when adding small size spec with empty actual", func() {
		It("should use all small defaults including resources", func() {
			actual := apiv2.WBClickHouseSpec{
				Enabled:  true,
				Replicas: 3,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddClickHouseSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedClickHouse.Enabled).To(BeTrue())
			Expect(builder.mergedClickHouse.Replicas).To(Equal(int32(3)))
			Expect(builder.mergedClickHouse.StorageSize).To(Equal(model.SmallClickHouseStorageSize))
			Expect(builder.mergedClickHouse.Version).To(Equal(model.ClickHouseVersion))
			Expect(builder.mergedClickHouse.Config).ToNot(BeNil())
			Expect(builder.mergedClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(model.SmallClickHouseCpuRequest)))
		})
	})

	Context("when adding small size spec with all overrides", func() {
		It("should use all overrides and verify they differ from defaults", func() {
			actual := apiv2.WBClickHouseSpec{
				Enabled:     overrideCHEnabled,
				Namespace:   overrideCHNamespace,
				StorageSize: overrideCHStorageSize,
				Version:     overrideCHVersion,
				Replicas:    overrideCHReplicas,
				Config: &apiv2.WBClickHouseConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideCHCpuRequest,
							corev1.ResourceMemory: overrideCHMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideCHCpuLimit,
							corev1.ResourceMemory: overrideCHMemoryLimit,
						},
					},
				},
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddClickHouseSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedClickHouse).ToNot(BeNil())

			Expect(builder.mergedClickHouse.Enabled).To(Equal(overrideCHEnabled))
			Expect(builder.mergedClickHouse.Enabled).ToNot(Equal(true))

			Expect(builder.mergedClickHouse.Namespace).To(Equal(overrideCHNamespace))
			Expect(builder.mergedClickHouse.Namespace).ToNot(Equal(testOwnerNamespace))

			Expect(builder.mergedClickHouse.StorageSize).To(Equal(overrideCHStorageSize))
			Expect(builder.mergedClickHouse.StorageSize).ToNot(Equal(model.SmallClickHouseStorageSize))

			Expect(builder.mergedClickHouse.Version).To(Equal(overrideCHVersion))
			Expect(builder.mergedClickHouse.Version).ToNot(Equal(model.ClickHouseVersion))

			Expect(builder.mergedClickHouse.Replicas).To(Equal(overrideCHReplicas))
			Expect(builder.mergedClickHouse.Replicas).ToNot(Equal(int32(3)))

			Expect(builder.mergedClickHouse.Config).ToNot(BeNil())
			Expect(builder.mergedClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideCHCpuRequest))
			Expect(builder.mergedClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallCHCpuRequest))

			Expect(builder.mergedClickHouse.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideCHMemoryRequest))
			Expect(builder.mergedClickHouse.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallCHMemoryRequest))

			Expect(builder.mergedClickHouse.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideCHCpuLimit))
			Expect(builder.mergedClickHouse.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallCHCpuLimit))

			Expect(builder.mergedClickHouse.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideCHMemoryLimit))
			Expect(builder.mergedClickHouse.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallCHMemoryLimit))
		})
	})

	Context("when adding small size spec with storage override only", func() {
		It("should use override storage and verify it differs from default", func() {
			actual := apiv2.WBClickHouseSpec{
				Enabled:     true,
				Replicas:    3,
				StorageSize: overrideCHStorageSize,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddClickHouseSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedClickHouse.StorageSize).To(Equal(overrideCHStorageSize))
			Expect(builder.mergedClickHouse.StorageSize).ToNot(Equal(model.SmallClickHouseStorageSize))
			Expect(builder.mergedClickHouse.Config).ToNot(BeNil())
			Expect(builder.mergedClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallCHCpuRequest))
		})
	})

	Context("when adding small size spec with version override only", func() {
		It("should use override version and verify it differs from default", func() {
			actual := apiv2.WBClickHouseSpec{
				Enabled:  true,
				Replicas: 3,
				Version:  overrideCHVersion,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddClickHouseSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedClickHouse.Version).To(Equal(overrideCHVersion))
			Expect(builder.mergedClickHouse.Version).ToNot(Equal(model.ClickHouseVersion))
			Expect(builder.mergedClickHouse.Config).ToNot(BeNil())
		})
	})

	Context("when adding small size spec with namespace override only", func() {
		It("should use override namespace and verify it differs from default", func() {
			actual := apiv2.WBClickHouseSpec{
				Enabled:   true,
				Replicas:  3,
				Namespace: overrideCHNamespace,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddClickHouseSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedClickHouse.Namespace).To(Equal(overrideCHNamespace))
			Expect(builder.mergedClickHouse.Namespace).ToNot(Equal(testOwnerNamespace))
		})
	})

	Context("when adding small size spec with resource overrides only", func() {
		It("should use override resources and verify they differ from defaults", func() {
			actual := apiv2.WBClickHouseSpec{
				Enabled:  true,
				Replicas: 3,
				Config: &apiv2.WBClickHouseConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideCHCpuRequest,
							corev1.ResourceMemory: overrideCHMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideCHCpuLimit,
							corev1.ResourceMemory: overrideCHMemoryLimit,
						},
					},
				},
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddClickHouseSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedClickHouse.Config).ToNot(BeNil())

			Expect(builder.mergedClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideCHCpuRequest))
			Expect(builder.mergedClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallCHCpuRequest))

			Expect(builder.mergedClickHouse.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideCHMemoryRequest))
			Expect(builder.mergedClickHouse.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallCHMemoryRequest))

			Expect(builder.mergedClickHouse.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideCHCpuLimit))
			Expect(builder.mergedClickHouse.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallCHCpuLimit))

			Expect(builder.mergedClickHouse.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideCHMemoryLimit))
			Expect(builder.mergedClickHouse.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallCHMemoryLimit))
		})
	})

	Context("when size is invalid", func() {
		It("should append error to builder", func() {
			actual := apiv2.WBClickHouseSpec{Enabled: true}
			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSize("invalid"))
			result := builder.AddClickHouseSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).ToNot(BeEmpty())
		})
	})
})

var _ = Describe("TranslateClickHouseConfig", func() {
	Context("when translating a complete ClickHouse config", func() {
		It("should correctly map all fields to WBClickHouseSpec", func() {
			config := model.ClickHouseConfig{
				Enabled:     true,
				Namespace:   overrideCHNamespace,
				StorageSize: overrideCHStorageSize,
				Replicas:    overrideCHReplicas,
				Version:     overrideCHVersion,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    overrideCHCpuRequest,
						corev1.ResourceMemory: overrideCHMemoryRequest,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    overrideCHCpuLimit,
						corev1.ResourceMemory: overrideCHMemoryLimit,
					},
				},
			}

			spec := TranslateClickHouseConfig(config)

			Expect(spec.Enabled).To(Equal(config.Enabled))
			Expect(spec.Namespace).To(Equal(config.Namespace))
			Expect(spec.StorageSize).To(Equal(config.StorageSize))
			Expect(spec.Replicas).To(Equal(config.Replicas))
			Expect(spec.Version).To(Equal(config.Version))
			Expect(spec.Config).ToNot(BeNil())
			Expect(spec.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideCHCpuRequest))
			Expect(spec.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideCHCpuLimit))
			Expect(spec.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideCHMemoryRequest))
			Expect(spec.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideCHMemoryLimit))
		})
	})

	Context("when translating a minimal ClickHouse config", func() {
		It("should handle empty resources", func() {
			config := model.ClickHouseConfig{
				Enabled:     overrideCHEnabled,
				Namespace:   overrideCHNamespace,
				StorageSize: model.DevClickHouseStorageSize,
				Replicas:    1,
				Version:     model.ClickHouseVersion,
				Resources:   corev1.ResourceRequirements{},
			}

			spec := TranslateClickHouseConfig(config)

			Expect(spec.Enabled).To(Equal(config.Enabled))
			Expect(spec.Namespace).To(Equal(config.Namespace))
			Expect(spec.StorageSize).To(Equal(config.StorageSize))
			Expect(spec.Replicas).To(Equal(config.Replicas))
			Expect(spec.Version).To(Equal(config.Version))
			Expect(spec.Config).ToNot(BeNil())
		})
	})
})
