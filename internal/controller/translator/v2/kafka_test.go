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
	defaultSmallKafkaCpuRequest    = resource.MustParse(model.SmallKafkaCpuRequest)
	defaultSmallKafkaCpuLimit      = resource.MustParse(model.SmallKafkaCpuLimit)
	defaultSmallKafkaMemoryRequest = resource.MustParse(model.SmallKafkaMemoryRequest)
	defaultSmallKafkaMemoryLimit   = resource.MustParse(model.SmallKafkaMemoryLimit)

	overrideKafkaStorageSize   = "20Gi"
	overrideKafkaNamespace     = "custom-namespace"
	overrideKafkaEnabled       = false
	overrideKafkaCpuRequest    = resource.MustParse("750m")
	overrideKafkaCpuLimit      = resource.MustParse("1500m")
	overrideKafkaMemoryRequest = resource.MustParse("2Gi")
	overrideKafkaMemoryLimit   = resource.MustParse("4Gi")
)

var _ = Describe("BuildKafkaSpec", func() {
	Describe("Config merging", func() {
		Context("when both Config values are nil", func() {
			It("should result in nil Config", func() {
				actual := apiv2.WBKafkaSpec{Config: nil}
				defaults := apiv2.WBKafkaSpec{Config: nil}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).To(BeNil())
			})
		})

		Context("when actual Config is nil", func() {
			It("should use default Config", func() {
				actual := apiv2.WBKafkaSpec{Config: nil}
				defaults := apiv2.WBKafkaSpec{
					Config: &apiv2.WBKafkaConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallKafkaCpuRequest,
								corev1.ResourceMemory: defaultSmallKafkaMemoryRequest,
							},
						},
					},
				}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallKafkaCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallKafkaMemoryRequest))
			})
		})

		Context("when default Config is nil", func() {
			It("should use actual Config", func() {
				actual := apiv2.WBKafkaSpec{
					Config: &apiv2.WBKafkaConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: overrideKafkaCpuRequest,
							},
						},
					},
				}
				defaults := apiv2.WBKafkaSpec{Config: nil}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuRequest))
			})
		})

		Context("when both Config values exist", func() {
			It("should merge resources with actual taking precedence", func() {
				actual := apiv2.WBKafkaSpec{
					Config: &apiv2.WBKafkaConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: overrideKafkaCpuRequest,
							},
						},
					},
				}
				defaults := apiv2.WBKafkaSpec{
					Config: &apiv2.WBKafkaConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallKafkaCpuRequest,
								corev1.ResourceMemory: defaultSmallKafkaMemoryRequest,
							},
						},
					},
				}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallKafkaMemoryRequest))
			})
		})
	})

	Describe("StorageSize merging", func() {
		Context("when actual StorageSize is empty", func() {
			It("should use default StorageSize", func() {
				actual := apiv2.WBKafkaSpec{StorageSize: ""}
				defaults := apiv2.WBKafkaSpec{StorageSize: model.SmallKafkaStorageSize}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(model.SmallKafkaStorageSize))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actual := apiv2.WBKafkaSpec{StorageSize: overrideKafkaStorageSize}
				defaults := apiv2.WBKafkaSpec{StorageSize: model.SmallKafkaStorageSize}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(overrideKafkaStorageSize))
			})
		})

		Context("when both StorageSize values are empty", func() {
			It("should result in empty StorageSize", func() {
				actual := apiv2.WBKafkaSpec{StorageSize: ""}
				defaults := apiv2.WBKafkaSpec{StorageSize: ""}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(""))
			})
		})
	})

	Describe("Namespace merging", func() {
		Context("when actual Namespace is empty", func() {
			It("should use default Namespace", func() {
				actual := apiv2.WBKafkaSpec{Namespace: ""}
				defaults := apiv2.WBKafkaSpec{Namespace: overrideKafkaNamespace}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideKafkaNamespace))
			})
		})

		Context("when actual Namespace is set", func() {
			It("should use actual Namespace", func() {
				actual := apiv2.WBKafkaSpec{Namespace: overrideKafkaNamespace}
				defaults := apiv2.WBKafkaSpec{Namespace: "default-namespace"}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideKafkaNamespace))
			})
		})
	})

	Describe("Enabled field", func() {
		Context("when actual Enabled is true", func() {
			It("should use actual value regardless of default", func() {
				actual := apiv2.WBKafkaSpec{Enabled: true}
				defaults := apiv2.WBKafkaSpec{Enabled: false}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
			})
		})

		Context("when actual Enabled is false", func() {
			It("should use actual value regardless of default", func() {
				actual := apiv2.WBKafkaSpec{Enabled: false}
				defaults := apiv2.WBKafkaSpec{Enabled: true}

				result, err := BuildKafkaSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
			})
		})
	})
})

var _ = Describe("InfraConfigBuilder.AddKafkaSpec", func() {
	const testOwnerNamespace = "test-namespace"

	Context("when adding dev size spec", func() {
		It("should merge actual with dev defaults from model", func() {
			actual := apiv2.WBKafkaSpec{
				Enabled: true,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeDev)
			result := builder.AddKafkaSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedKafka).ToNot(BeNil())
			Expect(builder.mergedKafka.Enabled).To(BeTrue())
			Expect(builder.mergedKafka.Namespace).To(Equal(testOwnerNamespace))
			Expect(builder.mergedKafka.StorageSize).To(Equal(model.DevKafkaStorageSize))
		})
	})

	Context("when adding small size spec with all overrides", func() {
		It("should use all overrides and verify they differ from defaults", func() {
			actual := apiv2.WBKafkaSpec{
				Enabled:     overrideKafkaEnabled,
				Namespace:   overrideKafkaNamespace,
				StorageSize: overrideKafkaStorageSize,
				Config: &apiv2.WBKafkaConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideKafkaCpuRequest,
							corev1.ResourceMemory: overrideKafkaMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideKafkaCpuLimit,
							corev1.ResourceMemory: overrideKafkaMemoryLimit,
						},
					},
				},
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddKafkaSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedKafka).ToNot(BeNil())

			Expect(builder.mergedKafka.Enabled).To(Equal(overrideKafkaEnabled))
			Expect(builder.mergedKafka.Enabled).ToNot(Equal(true))

			Expect(builder.mergedKafka.Namespace).To(Equal(overrideKafkaNamespace))
			Expect(builder.mergedKafka.Namespace).ToNot(Equal(testOwnerNamespace))

			Expect(builder.mergedKafka.StorageSize).To(Equal(overrideKafkaStorageSize))
			Expect(builder.mergedKafka.StorageSize).ToNot(Equal(model.SmallKafkaStorageSize))

			Expect(builder.mergedKafka.Config).ToNot(BeNil())
			Expect(builder.mergedKafka.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuRequest))
			Expect(builder.mergedKafka.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallKafkaCpuRequest))

			Expect(builder.mergedKafka.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideKafkaMemoryRequest))
			Expect(builder.mergedKafka.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallKafkaMemoryRequest))

			Expect(builder.mergedKafka.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuLimit))
			Expect(builder.mergedKafka.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallKafkaCpuLimit))

			Expect(builder.mergedKafka.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideKafkaMemoryLimit))
			Expect(builder.mergedKafka.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallKafkaMemoryLimit))
		})
	})

	Context("when adding small size spec with storage override only", func() {
		It("should use override storage and verify it differs from default", func() {
			actual := apiv2.WBKafkaSpec{
				Enabled:     true,
				StorageSize: overrideKafkaStorageSize,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddKafkaSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedKafka.StorageSize).To(Equal(overrideKafkaStorageSize))
			Expect(builder.mergedKafka.StorageSize).ToNot(Equal(model.SmallKafkaStorageSize))
			Expect(builder.mergedKafka.Config).ToNot(BeNil())
		})
	})

	Context("when adding small size spec with namespace override only", func() {
		It("should use override namespace and verify it differs from default", func() {
			actual := apiv2.WBKafkaSpec{
				Enabled:   true,
				Namespace: overrideKafkaNamespace,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddKafkaSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedKafka.Namespace).To(Equal(overrideKafkaNamespace))
			Expect(builder.mergedKafka.Namespace).ToNot(Equal(testOwnerNamespace))
		})
	})

	Context("when adding small size spec with resource overrides only", func() {
		It("should use override resources and verify they differ from defaults", func() {
			actual := apiv2.WBKafkaSpec{
				Enabled: true,
				Config: &apiv2.WBKafkaConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideKafkaCpuRequest,
							corev1.ResourceMemory: overrideKafkaMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideKafkaCpuLimit,
							corev1.ResourceMemory: overrideKafkaMemoryLimit,
						},
					},
				},
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddKafkaSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedKafka.Config).ToNot(BeNil())

			Expect(builder.mergedKafka.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuRequest))
			Expect(builder.mergedKafka.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallKafkaCpuRequest))

			Expect(builder.mergedKafka.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideKafkaMemoryRequest))
			Expect(builder.mergedKafka.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallKafkaMemoryRequest))

			Expect(builder.mergedKafka.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuLimit))
			Expect(builder.mergedKafka.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallKafkaCpuLimit))

			Expect(builder.mergedKafka.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideKafkaMemoryLimit))
			Expect(builder.mergedKafka.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallKafkaMemoryLimit))
		})
	})

	Context("when size is invalid", func() {
		It("should append error to builder", func() {
			actual := apiv2.WBKafkaSpec{Enabled: true}
			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSize("invalid"))
			result := builder.AddKafkaSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).ToNot(BeEmpty())
		})
	})
})

var _ = Describe("TranslateKafkaConfig", func() {
	Context("when translating a complete Kafka config", func() {
		It("should correctly map all fields to WBKafkaSpec", func() {
			config := model.KafkaConfig{
				Enabled:     true,
				Namespace:   overrideKafkaNamespace,
				StorageSize: overrideKafkaStorageSize,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    overrideKafkaCpuRequest,
						corev1.ResourceMemory: overrideKafkaMemoryRequest,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    overrideKafkaCpuLimit,
						corev1.ResourceMemory: overrideKafkaMemoryLimit,
					},
				},
			}

			spec := TranslateKafkaConfig(config)

			Expect(spec.Enabled).To(Equal(config.Enabled))
			Expect(spec.Namespace).To(Equal(config.Namespace))
			Expect(spec.StorageSize).To(Equal(config.StorageSize))
			Expect(spec.Config).ToNot(BeNil())
			Expect(spec.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuRequest))
			Expect(spec.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuLimit))
		})
	})
})
