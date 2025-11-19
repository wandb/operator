package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/common"
	"github.com/wandb/operator/internal/defaults"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	defaultSmallKafkaCpuRequest    = resource.MustParse(defaults.SmallKafkaCpuRequest)
	defaultSmallKafkaCpuLimit      = resource.MustParse(defaults.SmallKafkaCpuLimit)
	defaultSmallKafkaMemoryRequest = resource.MustParse(defaults.SmallKafkaMemoryRequest)
	defaultSmallKafkaMemoryLimit   = resource.MustParse(defaults.SmallKafkaMemoryLimit)

	overrideKafkaStorageSize   = "20Gi"
	overrideKafkaNamespace     = "custom-namespace"
	overrideKafkaEnabled       = false
	overrideKafkaCpuRequest    = resource.MustParse("750m")
	overrideKafkaCpuLimit      = resource.MustParse("1500m")
	overrideKafkaMemoryRequest = resource.MustParse("2Gi")
	overrideKafkaMemoryLimit   = resource.MustParse("4Gi")
)

var _ = Describe("BuildKafkaConfig", func() {
	Describe("Config merging", func() {
		Context("when actual Config is nil", func() {
			It("should use default Config resources", func() {
				actual := apiv2.WBKafkaSpec{Config: nil}
				defaultConfig := common.KafkaConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    defaultSmallKafkaCpuRequest,
							corev1.ResourceMemory: defaultSmallKafkaMemoryRequest,
						},
					},
				}

				result, err := BuildKafkaConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallKafkaCpuRequest))
				Expect(result.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallKafkaMemoryRequest))
			})
		})

		Context("when actual Config exists", func() {
			It("should use actual Config resources and merge with defaults", func() {
				actual := apiv2.WBKafkaSpec{
					Config: &apiv2.WBKafkaConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: overrideKafkaCpuRequest,
							},
						},
					},
				}
				defaultConfig := common.KafkaConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    defaultSmallKafkaCpuRequest,
							corev1.ResourceMemory: defaultSmallKafkaMemoryRequest,
						},
					},
				}

				result, err := BuildKafkaConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuRequest))
				Expect(result.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallKafkaMemoryRequest))
			})
		})
	})

	Describe("StorageSize merging", func() {
		Context("when actual StorageSize is empty", func() {
			It("should use default StorageSize", func() {
				actual := apiv2.WBKafkaSpec{StorageSize: ""}
				defaultConfig := common.KafkaConfig{StorageSize: defaults.SmallKafkaStorageSize}

				result, err := BuildKafkaConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(defaults.SmallKafkaStorageSize))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actual := apiv2.WBKafkaSpec{StorageSize: overrideKafkaStorageSize}
				defaultConfig := common.KafkaConfig{StorageSize: defaults.SmallKafkaStorageSize}

				result, err := BuildKafkaConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(overrideKafkaStorageSize))
			})
		})

		Context("when both StorageSize values are empty", func() {
			It("should result in empty StorageSize", func() {
				actual := apiv2.WBKafkaSpec{StorageSize: ""}
				defaultConfig := common.KafkaConfig{StorageSize: ""}

				result, err := BuildKafkaConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(""))
			})
		})
	})

	Describe("Namespace merging", func() {
		Context("when actual Namespace is empty", func() {
			It("should use default Namespace", func() {
				actual := apiv2.WBKafkaSpec{Namespace: ""}
				defaultConfig := common.KafkaConfig{Namespace: overrideKafkaNamespace}

				result, err := BuildKafkaConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideKafkaNamespace))
			})
		})

		Context("when actual Namespace is set", func() {
			It("should use actual Namespace", func() {
				actual := apiv2.WBKafkaSpec{Namespace: overrideKafkaNamespace}
				defaultConfig := common.KafkaConfig{Namespace: "default-namespace"}

				result, err := BuildKafkaConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideKafkaNamespace))
			})
		})
	})

	Describe("Enabled field", func() {
		Context("when actual Enabled is true", func() {
			It("should use actual value regardless of default", func() {
				actual := apiv2.WBKafkaSpec{Enabled: true}
				defaultConfig := common.KafkaConfig{Enabled: false}

				result, err := BuildKafkaConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
			})
		})

		Context("when actual Enabled is false", func() {
			It("should use actual value regardless of default", func() {
				actual := apiv2.WBKafkaSpec{Enabled: false}
				defaultConfig := common.KafkaConfig{Enabled: true}

				result, err := BuildKafkaConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
			})
		})
	})
})

var _ = Describe("InfraConfigBuilder.AddKafkaConfig", func() {
	const testOwnerNamespace = "test-namespace"

	Context("when adding dev size spec", func() {
		It("should merge actual with dev defaults from common", func() {
			actual := apiv2.WBKafkaSpec{
				Enabled: true,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeDev)
			result := builder.AddKafkaConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedKafka.Enabled).To(BeTrue())
			Expect(builder.mergedKafka.Namespace).To(Equal(testOwnerNamespace))
			Expect(builder.mergedKafka.StorageSize).To(Equal(defaults.DevKafkaStorageSize))
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
			result := builder.AddKafkaConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())

			Expect(builder.mergedKafka.Enabled).To(Equal(overrideKafkaEnabled))
			Expect(builder.mergedKafka.Enabled).ToNot(Equal(true))

			Expect(builder.mergedKafka.Namespace).To(Equal(overrideKafkaNamespace))
			Expect(builder.mergedKafka.Namespace).ToNot(Equal(testOwnerNamespace))

			Expect(builder.mergedKafka.StorageSize).To(Equal(overrideKafkaStorageSize))
			Expect(builder.mergedKafka.StorageSize).ToNot(Equal(defaults.SmallKafkaStorageSize))

			Expect(builder.mergedKafka.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuRequest))
			Expect(builder.mergedKafka.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallKafkaCpuRequest))

			Expect(builder.mergedKafka.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideKafkaMemoryRequest))
			Expect(builder.mergedKafka.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallKafkaMemoryRequest))

			Expect(builder.mergedKafka.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuLimit))
			Expect(builder.mergedKafka.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallKafkaCpuLimit))

			Expect(builder.mergedKafka.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideKafkaMemoryLimit))
			Expect(builder.mergedKafka.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallKafkaMemoryLimit))
		})
	})

	Context("when adding small size spec with storage override only", func() {
		It("should use override storage and verify it differs from default", func() {
			actual := apiv2.WBKafkaSpec{
				Enabled:     true,
				StorageSize: overrideKafkaStorageSize,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddKafkaConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedKafka.StorageSize).To(Equal(overrideKafkaStorageSize))
			Expect(builder.mergedKafka.StorageSize).ToNot(Equal(defaults.SmallKafkaStorageSize))
		})
	})

	Context("when adding small size spec with namespace override only", func() {
		It("should use override namespace and verify it differs from default", func() {
			actual := apiv2.WBKafkaSpec{
				Enabled:   true,
				Namespace: overrideKafkaNamespace,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddKafkaConfig(actual)

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
			result := builder.AddKafkaConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())

			Expect(builder.mergedKafka.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuRequest))
			Expect(builder.mergedKafka.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallKafkaCpuRequest))

			Expect(builder.mergedKafka.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideKafkaMemoryRequest))
			Expect(builder.mergedKafka.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallKafkaMemoryRequest))

			Expect(builder.mergedKafka.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuLimit))
			Expect(builder.mergedKafka.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallKafkaCpuLimit))

			Expect(builder.mergedKafka.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideKafkaMemoryLimit))
			Expect(builder.mergedKafka.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallKafkaMemoryLimit))
		})
	})

	Context("when size is invalid", func() {
		It("should append error to builder", func() {
			actual := apiv2.WBKafkaSpec{Enabled: true}
			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSize("invalid"))
			result := builder.AddKafkaConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).ToNot(BeEmpty())
		})
	})
})

var _ = Describe("TranslateKafkaSpec", func() {
	Context("when translating a complete Kafka spec", func() {
		It("should correctly map all fields to common.KafkaConfig", func() {
			spec := apiv2.WBKafkaSpec{
				Enabled:     true,
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

			config := TranslateKafkaSpec(spec)

			Expect(config.Enabled).To(Equal(spec.Enabled))
			Expect(config.Namespace).To(Equal(spec.Namespace))
			Expect(config.StorageSize).To(Equal(spec.StorageSize))
			Expect(config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuRequest))
			Expect(config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideKafkaCpuLimit))
		})
	})
})

var _ = Describe("TranslateKafkaStatus", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("when common status has no errors or details", func() {
		It("should return ready status when Ready is true", func() {
			modelStatus := common.KafkaStatus{
				Ready: true,
				Connection: common.KafkaConnection{
					Host: "kafka.example.com",
					Port: "9092",
				},
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeTrue())
			Expect(result.State).To(Equal(apiv2.WBStateReady))
			Expect(result.Details).To(BeEmpty())
			Expect(result.Connection.KafkaHost).To(Equal("kafka.example.com"))
			Expect(result.Connection.KafkaPort).To(Equal("9092"))
			Expect(result.LastReconciled.IsZero()).To(BeFalse())
		})

		It("should return unknown status when Ready is false", func() {
			modelStatus := common.KafkaStatus{
				Ready: false,
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateUnknown))
			Expect(result.Details).To(BeEmpty())
		})
	})

	Context("when common status has errors", func() {
		It("should translate errors to status details with Error state", func() {
			modelStatus := common.KafkaStatus{
				Ready: false,
				Errors: []common.KafkaInfraError{
					{InfraError: common.NewKafkaError(common.KafkaErrFailedToCreateCode, "creation failed")},
					{InfraError: common.NewKafkaError(common.KafkaErrFailedToUpdateCode, "update failed")},
				},
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateError))
			Expect(result.Details).To(HaveLen(2))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateError))
			Expect(result.Details[0].Code).To(Equal(string(common.KafkaErrFailedToCreateCode)))
			Expect(result.Details[0].Message).To(Equal("creation failed"))
			Expect(result.Details[1].State).To(Equal(apiv2.WBStateError))
			Expect(result.Details[1].Code).To(Equal(string(common.KafkaErrFailedToUpdateCode)))
			Expect(result.Details[1].Message).To(Equal("update failed"))
		})
	})

	Context("when common status has status details", func() {
		It("should translate KafkaCreated to Updating state", func() {
			modelStatus := common.KafkaStatus{
				Ready: false,
				Details: []common.KafkaStatusDetail{
					{InfraStatusDetail: common.NewKafkaStatusDetail(common.KafkaCreatedCode, "Kafka created")},
				},
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(1))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.Details[0].Code).To(Equal(string(common.KafkaCreatedCode)))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate KafkaUpdated to Updating state", func() {
			modelStatus := common.KafkaStatus{
				Ready: false,
				Details: []common.KafkaStatusDetail{
					{InfraStatusDetail: common.NewKafkaStatusDetail(common.KafkaUpdatedCode, "Kafka updated")},
				},
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate KafkaDeleted to Deleting state", func() {
			modelStatus := common.KafkaStatus{
				Ready: false,
				Details: []common.KafkaStatusDetail{
					{InfraStatusDetail: common.NewKafkaStatusDetail(common.KafkaDeletedCode, "Kafka deleted")},
				},
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateDeleting))
			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})

		It("should translate KafkaNodePoolCreated to Updating state", func() {
			modelStatus := common.KafkaStatus{
				Ready: false,
				Details: []common.KafkaStatusDetail{
					{InfraStatusDetail: common.NewKafkaStatusDetail(common.KafkaNodePoolCreatedCode, "NodePool created")},
				},
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate KafkaNodePoolUpdated to Updating state", func() {
			modelStatus := common.KafkaStatus{
				Ready: false,
				Details: []common.KafkaStatusDetail{
					{InfraStatusDetail: common.NewKafkaStatusDetail(common.KafkaNodePoolUpdatedCode, "NodePool updated")},
				},
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate KafkaNodePoolDeleted to Deleting state", func() {
			modelStatus := common.KafkaStatus{
				Ready: false,
				Details: []common.KafkaStatusDetail{
					{InfraStatusDetail: common.NewKafkaStatusDetail(common.KafkaNodePoolDeletedCode, "NodePool deleted")},
				},
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateDeleting))
			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})

		It("should translate KafkaConnection to Ready state", func() {
			modelStatus := common.KafkaStatus{
				Ready: true,
				Details: []common.KafkaStatusDetail{
					{InfraStatusDetail: common.NewKafkaStatusDetail(common.KafkaConnectionCode, "connection established")},
				},
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateReady))
			Expect(result.State).To(Equal(apiv2.WBStateReady))
		})
	})

	Context("when common status has both errors and details", func() {
		It("should use worst state according to WorseThan", func() {
			modelStatus := common.KafkaStatus{
				Ready: false,
				Errors: []common.KafkaInfraError{
					{InfraError: common.NewKafkaError(common.KafkaErrFailedToCreateCode, "creation failed")},
				},
				Details: []common.KafkaStatusDetail{
					{InfraStatusDetail: common.NewKafkaStatusDetail(common.KafkaCreatedCode, "Kafka created")},
				},
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(2))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})
	})

	Context("when common status has multiple details with different states", func() {
		It("should compute worst state correctly", func() {
			modelStatus := common.KafkaStatus{
				Ready: false,
				Details: []common.KafkaStatusDetail{
					{InfraStatusDetail: common.NewKafkaStatusDetail(common.KafkaUpdatedCode, "updating")},
					{InfraStatusDetail: common.NewKafkaStatusDetail(common.KafkaDeletedCode, "deleting")},
				},
			}

			result := TranslateKafkaStatus(ctx, modelStatus)

			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})
	})
})
