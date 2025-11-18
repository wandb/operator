package v2

import (
	"context"

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

var _ = Describe("BuildMinioConfig", func() {
	Describe("Config merging", func() {
		Context("when actual Config is nil", func() {
			It("should use default Config resources", func() {
				actual := apiv2.WBMinioSpec{Config: nil}
				defaultConfig := model.MinioConfig{
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
				defaultConfig := model.MinioConfig{
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
				defaultConfig := model.MinioConfig{StorageSize: model.SmallMinioStorageSize}

				result, err := BuildMinioConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(model.SmallMinioStorageSize))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actual := apiv2.WBMinioSpec{StorageSize: overrideMinioStorageSize}
				defaultConfig := model.MinioConfig{StorageSize: model.SmallMinioStorageSize}

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
				defaultConfig := model.MinioConfig{Namespace: overrideMinioNamespace}

				result, err := BuildMinioConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideMinioNamespace))
			})
		})

		Context("when actual Namespace is set", func() {
			It("should use actual Namespace", func() {
				actual := apiv2.WBMinioSpec{Namespace: overrideMinioNamespace}
				defaultConfig := model.MinioConfig{Namespace: "default-namespace"}

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
				defaultConfig := model.MinioConfig{Enabled: false}

				result, err := BuildMinioConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
			})
		})

		Context("when actual Enabled is false", func() {
			It("should use actual value regardless of default", func() {
				actual := apiv2.WBMinioSpec{Enabled: false}
				defaultConfig := model.MinioConfig{Enabled: true}

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
		It("should merge actual with dev defaults from model", func() {
			actual := apiv2.WBMinioSpec{
				Enabled: true,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeDev)
			result := builder.AddMinioConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
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
			Expect(builder.mergedMinio.StorageSize).ToNot(Equal(model.SmallMinioStorageSize))

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
			Expect(builder.mergedMinio.StorageSize).ToNot(Equal(model.SmallMinioStorageSize))
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
		It("should correctly map all fields to model.MinioConfig", func() {
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

	Context("when model status has no errors or details", func() {
		It("should return ready status when Ready is true", func() {
			modelStatus := model.MinioStatus{
				Ready: true,
				Connection: model.MinioConnection{
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
			modelStatus := model.MinioStatus{
				Ready: false,
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateUnknown))
			Expect(result.Details).To(BeEmpty())
		})
	})

	Context("when model status has errors", func() {
		It("should translate errors to status details with Error state", func() {
			modelStatus := model.MinioStatus{
				Ready: false,
				Errors: []model.MinioInfraError{
					{InfraError: model.NewMinioError(model.MinioErrFailedToCreateCode, "creation failed")},
					{InfraError: model.NewMinioError(model.MinioErrFailedToUpdateCode, "update failed")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateError))
			Expect(result.Details).To(HaveLen(2))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateError))
			Expect(result.Details[0].Code).To(Equal(string(model.MinioErrFailedToCreateCode)))
			Expect(result.Details[0].Message).To(Equal("creation failed"))
			Expect(result.Details[1].State).To(Equal(apiv2.WBStateError))
			Expect(result.Details[1].Code).To(Equal(string(model.MinioErrFailedToUpdateCode)))
			Expect(result.Details[1].Message).To(Equal("update failed"))
		})
	})

	Context("when model status has status details", func() {
		It("should translate MinioCreated to Updating state", func() {
			modelStatus := model.MinioStatus{
				Ready: false,
				Details: []model.MinioStatusDetail{
					{InfraStatusDetail: model.NewMinioStatusDetail(model.MinioCreatedCode, "Minio created")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(1))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.Details[0].Code).To(Equal(string(model.MinioCreatedCode)))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate MinioUpdated to Updating state", func() {
			modelStatus := model.MinioStatus{
				Ready: false,
				Details: []model.MinioStatusDetail{
					{InfraStatusDetail: model.NewMinioStatusDetail(model.MinioUpdatedCode, "Minio updated")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate MinioDeleted to Deleting state", func() {
			modelStatus := model.MinioStatus{
				Ready: false,
				Details: []model.MinioStatusDetail{
					{InfraStatusDetail: model.NewMinioStatusDetail(model.MinioDeletedCode, "Minio deleted")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateDeleting))
			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})

		It("should translate MinioConnection to Ready state", func() {
			modelStatus := model.MinioStatus{
				Ready: true,
				Details: []model.MinioStatusDetail{
					{InfraStatusDetail: model.NewMinioStatusDetail(model.MinioConnectionCode, "connection established")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateReady))
			Expect(result.State).To(Equal(apiv2.WBStateReady))
		})
	})

	Context("when model status has both errors and details", func() {
		It("should use worst state according to WorseThan", func() {
			modelStatus := model.MinioStatus{
				Ready: false,
				Errors: []model.MinioInfraError{
					{InfraError: model.NewMinioError(model.MinioErrFailedToCreateCode, "creation failed")},
				},
				Details: []model.MinioStatusDetail{
					{InfraStatusDetail: model.NewMinioStatusDetail(model.MinioCreatedCode, "Minio created")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(2))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})
	})

	Context("when model status has multiple details with different states", func() {
		It("should compute worst state correctly", func() {
			modelStatus := model.MinioStatus{
				Ready: false,
				Details: []model.MinioStatusDetail{
					{InfraStatusDetail: model.NewMinioStatusDetail(model.MinioUpdatedCode, "updating")},
					{InfraStatusDetail: model.NewMinioStatusDetail(model.MinioDeletedCode, "deleting")},
				},
			}

			result := TranslateMinioStatus(ctx, modelStatus)

			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})
	})
})
