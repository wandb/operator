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
	defaultSmallMinioCpuRequest    = resource.MustParse(defaults.SmallMinioCpuRequest)
	defaultSmallMinioCpuLimit      = resource.MustParse(defaults.SmallMinioCpuLimit)
	defaultSmallMinioMemoryRequest = resource.MustParse(defaults.SmallMinioMemoryRequest)
	defaultSmallMinioMemoryLimit   = resource.MustParse(defaults.SmallMinioMemoryLimit)

	overrideMinioStorageSize   = "50Gi"
	overrideMinioNamespace     = "custom-namespace"
	overrideMinioReplicas      = int32(5)
	overrideMinioEnabled       = false
	overrideMinioCpuRequest    = resource.MustParse("1")
	overrideMinioCpuLimit      = resource.MustParse("2")
	overrideMinioMemoryRequest = resource.MustParse("2Gi")
	overrideMinioMemoryLimit   = resource.MustParse("4Gi")
)

var _ = Describe("BuildMinioConfig", func() {
	Describe("Config merging", func() {
		Context("when actual Config is nil", func() {
			It("should use default Config resources", func() {
				actual := apiv2.WBMinioSpec{Config: nil}
				defaultConfig := common.MinioConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: defaultSmallMinioCpuRequest,
						},
					},
				}

				result, err := BuildMinioConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallMinioCpuRequest))
			})
		})

		Context("when actual Config exists", func() {
			It("should use actual Config resources and merge with defaults", func() {
				actual := apiv2.WBMinioSpec{
					Config: &apiv2.WBMinioConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: overrideMinioCpuRequest,
							},
						},
					},
				}
				defaultConfig := common.MinioConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    defaultSmallMinioCpuRequest,
							corev1.ResourceMemory: defaultSmallMinioMemoryRequest,
						},
					},
				}

				result, err := BuildMinioConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMinioCpuRequest))
				Expect(result.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallMinioMemoryRequest))
			})
		})
	})

	Describe("StorageSize merging", func() {
		Context("when actual StorageSize is empty", func() {
			It("should use default StorageSize", func() {
				actual := apiv2.WBMinioSpec{StorageSize: ""}
				defaultConfig := common.MinioConfig{StorageSize: defaults.SmallMinioStorageSize}

				result, err := BuildMinioConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(defaults.SmallMinioStorageSize))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actual := apiv2.WBMinioSpec{StorageSize: overrideMinioStorageSize}
				defaultConfig := common.MinioConfig{StorageSize: defaults.SmallMinioStorageSize}

				result, err := BuildMinioConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(overrideMinioStorageSize))
			})
		})
	})

	Describe("Namespace merging", func() {
		Context("when actual Namespace is empty", func() {
			It("should use default Namespace", func() {
				actual := apiv2.WBMinioSpec{Namespace: ""}
				defaultConfig := common.MinioConfig{Namespace: overrideMinioNamespace}

				result, err := BuildMinioConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideMinioNamespace))
			})
		})

		Context("when actual Namespace is set", func() {
			It("should use actual Namespace", func() {
				actual := apiv2.WBMinioSpec{Namespace: overrideMinioNamespace}
				defaultConfig := common.MinioConfig{Namespace: "default-namespace"}

				result, err := BuildMinioConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideMinioNamespace))
			})
		})
	})

	Describe("Enabled field", func() {
		Context("when actual Enabled is true", func() {
			It("should use actual value regardless of default", func() {
				actual := apiv2.WBMinioSpec{Enabled: true}
				defaultConfig := common.MinioConfig{Enabled: false}

				result, err := BuildMinioConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
			})
		})

		Context("when actual Enabled is false", func() {
			It("should use actual value regardless of default", func() {
				actual := apiv2.WBMinioSpec{Enabled: false}
				defaultConfig := common.MinioConfig{Enabled: true}

				result, err := BuildMinioConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
			})
		})
	})
})

var _ = Describe("InfraConfigBuilder.AddMinioConfig", func() {
	const testOwnerNamespace = "test-namespace"

	Context("when adding dev size spec", func() {
		It("should merge actual with dev defaults from common", func() {
			actual := apiv2.WBMinioSpec{
				Enabled: true,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeDev)
			result := builder.AddMinioConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMinio.Enabled).To(BeTrue())
			Expect(builder.mergedMinio.Namespace).To(Equal(testOwnerNamespace))
			Expect(builder.mergedMinio.StorageSize).To(Equal(defaults.DevMinioStorageSize))
		})
	})

	Context("when adding small size spec with all overrides", func() {
		It("should use all overrides and verify they differ from defaults", func() {
			actual := apiv2.WBMinioSpec{
				Enabled:     overrideMinioEnabled,
				Namespace:   overrideMinioNamespace,
				StorageSize: overrideMinioStorageSize,
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
			result := builder.AddMinioConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())

			Expect(builder.mergedMinio.Enabled).To(Equal(overrideMinioEnabled))
			Expect(builder.mergedMinio.Enabled).ToNot(Equal(true))

			Expect(builder.mergedMinio.Namespace).To(Equal(overrideMinioNamespace))
			Expect(builder.mergedMinio.Namespace).ToNot(Equal(testOwnerNamespace))

			Expect(builder.mergedMinio.StorageSize).To(Equal(overrideMinioStorageSize))
			Expect(builder.mergedMinio.StorageSize).ToNot(Equal(defaults.SmallMinioStorageSize))

			Expect(builder.mergedMinio.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMinioCpuRequest))
			Expect(builder.mergedMinio.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMinioCpuRequest))

			Expect(builder.mergedMinio.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideMinioMemoryRequest))
			Expect(builder.mergedMinio.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMinioMemoryRequest))

			Expect(builder.mergedMinio.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideMinioCpuLimit))
			Expect(builder.mergedMinio.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMinioCpuLimit))

			Expect(builder.mergedMinio.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMinioMemoryLimit))
			Expect(builder.mergedMinio.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMinioMemoryLimit))
		})
	})

	Context("when adding small size spec with storage override only", func() {
		It("should use override storage and verify it differs from default", func() {
			actual := apiv2.WBMinioSpec{
				Enabled:     true,
				StorageSize: overrideMinioStorageSize,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMinioConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMinio.StorageSize).To(Equal(overrideMinioStorageSize))
			Expect(builder.mergedMinio.StorageSize).ToNot(Equal(defaults.SmallMinioStorageSize))
		})
	})

	Context("when adding small size spec with namespace override only", func() {
		It("should use override namespace and verify it differs from default", func() {
			actual := apiv2.WBMinioSpec{
				Enabled:   true,
				Namespace: overrideMinioNamespace,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMinioConfig(actual)

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
			result := builder.AddMinioConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())

			Expect(builder.mergedMinio.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMinioCpuRequest))
			Expect(builder.mergedMinio.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMinioCpuRequest))

			Expect(builder.mergedMinio.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideMinioMemoryRequest))
			Expect(builder.mergedMinio.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMinioMemoryRequest))

			Expect(builder.mergedMinio.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideMinioCpuLimit))
			Expect(builder.mergedMinio.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMinioCpuLimit))

			Expect(builder.mergedMinio.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMinioMemoryLimit))
			Expect(builder.mergedMinio.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMinioMemoryLimit))
		})
	})

	Context("when size is invalid", func() {
		It("should append error to builder", func() {
			actual := apiv2.WBMinioSpec{Enabled: true}
			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSize("invalid"))
			result := builder.AddMinioConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).ToNot(BeEmpty())
		})
	})
})

var _ = Describe("TranslateMinioSpec", func() {
	Context("when translating a complete Minio spec", func() {
		It("should correctly map all fields to common.MinioConfig", func() {
			spec := apiv2.WBMinioSpec{
				Enabled:     true,
				Namespace:   overrideMinioNamespace,
				StorageSize: overrideMinioStorageSize,
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

			config := TranslateMinioSpec(spec)

			Expect(config.Enabled).To(Equal(spec.Enabled))
			Expect(config.Namespace).To(Equal(spec.Namespace))
			Expect(config.StorageSize).To(Equal(spec.StorageSize))
			Expect(config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMinioCpuRequest))
			Expect(config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMinioMemoryLimit))
		})
	})
})

var _ = Describe("TranslateMinioStatus", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("when common status has no errors or details", func() {
		It("should return ready status when Ready is true", func() {
			modelStatus := common.MinioStatus{
				Ready: true,
				Connection: common.MinioConnection{
					Host:      "minio.example.com",
					Port:      "9000",
					AccessKey: "admin",
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeTrue())
			Expect(result.State).To(Equal(apiv2.WBStateReady))
			Expect(result.Details).To(BeEmpty())
			Expect(result.Connection.MinioHost).To(Equal("minio.example.com"))
			Expect(result.Connection.MinioPort).To(Equal("9000"))
			Expect(result.Connection.MinioAccessKey).To(Equal("admin"))
			Expect(result.LastReconciled.IsZero()).To(BeFalse())
		})

		It("should return unknown status when Ready is false", func() {
			modelStatus := common.MinioStatus{
				Ready: false,
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateUnknown))
			Expect(result.Details).To(BeEmpty())
		})
	})

	Context("when common status has errors", func() {
		It("should translate errors to status details with Error state", func() {
			modelStatus := common.MinioStatus{
				Ready: false,
				Errors: []common.MinioInfraError{
					{InfraError: common.NewMinioError(common.MinioErrFailedToCreateCode, "creation failed")},
					{InfraError: common.NewMinioError(common.MinioErrFailedToUpdateCode, "update failed")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateError))
			Expect(result.Details).To(HaveLen(2))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateError))
			Expect(result.Details[0].Code).To(Equal(string(common.MinioErrFailedToCreateCode)))
			Expect(result.Details[0].Message).To(Equal("creation failed"))
			Expect(result.Details[1].State).To(Equal(apiv2.WBStateError))
			Expect(result.Details[1].Code).To(Equal(string(common.MinioErrFailedToUpdateCode)))
			Expect(result.Details[1].Message).To(Equal("update failed"))
		})
	})

	Context("when common status has status details", func() {
		It("should translate MinioCreated to Updating state", func() {
			modelStatus := common.MinioStatus{
				Ready: false,
				Details: []common.MinioStatusDetail{
					{InfraStatusDetail: common.NewMinioStatusDetail(common.MinioCreatedCode, "Minio created")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(1))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.Details[0].Code).To(Equal(string(common.MinioCreatedCode)))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate MinioUpdated to Updating state", func() {
			modelStatus := common.MinioStatus{
				Ready: false,
				Details: []common.MinioStatusDetail{
					{InfraStatusDetail: common.NewMinioStatusDetail(common.MinioUpdatedCode, "Minio updated")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate MinioDeleted to Deleting state", func() {
			modelStatus := common.MinioStatus{
				Ready: false,
				Details: []common.MinioStatusDetail{
					{InfraStatusDetail: common.NewMinioStatusDetail(common.MinioDeletedCode, "Minio deleted")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateDeleting))
			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})

		It("should translate MinioConnection to Ready state", func() {
			modelStatus := common.MinioStatus{
				Ready: true,
				Details: []common.MinioStatusDetail{
					{InfraStatusDetail: common.NewMinioStatusDetail(common.MinioConnectionCode, "connection established")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateReady))
			Expect(result.State).To(Equal(apiv2.WBStateReady))
		})
	})

	Context("when common status has both errors and details", func() {
		It("should use worst state according to WorseThan", func() {
			modelStatus := common.MinioStatus{
				Ready: false,
				Errors: []common.MinioInfraError{
					{InfraError: common.NewMinioError(common.MinioErrFailedToCreateCode, "creation failed")},
				},
				Details: []common.MinioStatusDetail{
					{InfraStatusDetail: common.NewMinioStatusDetail(common.MinioCreatedCode, "Minio created")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(2))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})
	})

	Context("when common status has multiple details with different states", func() {
		It("should compute worst state correctly", func() {
			modelStatus := common.MinioStatus{
				Ready: false,
				Details: []common.MinioStatusDetail{
					{InfraStatusDetail: common.NewMinioStatusDetail(common.MinioUpdatedCode, "updating")},
					{InfraStatusDetail: common.NewMinioStatusDetail(common.MinioDeletedCode, "deleting")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})
	})
})
