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
	defaultSmallCHCpuRequest    = resource.MustParse(defaults.SmallClickHouseCpuRequest)
	defaultSmallCHCpuLimit      = resource.MustParse(defaults.SmallClickHouseCpuLimit)
	defaultSmallCHMemoryRequest = resource.MustParse(defaults.SmallClickHouseMemoryRequest)
	defaultSmallCHMemoryLimit   = resource.MustParse(defaults.SmallClickHouseMemoryLimit)

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

var _ = Describe("BuildClickHouseConfig", func() {
	Describe("Config merging", func() {
		Context("when actual Config is nil", func() {
			It("should use default Config resources", func() {
				actual := apiv2.WBClickHouseSpec{Config: nil}
				defaultConfig := common.ClickHouseConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: defaultSmallCHCpuRequest,
						},
					},
				}

				result, err := BuildClickHouseConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallCHCpuRequest))
			})
		})

		Context("when actual Config exists", func() {
			It("should use actual Config resources and merge with defaults", func() {
				actual := apiv2.WBClickHouseSpec{
					Config: &apiv2.WBClickHouseConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: overrideCHCpuRequest},
						},
					},
				}
				defaultConfig := common.ClickHouseConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    defaultSmallCHCpuRequest,
							corev1.ResourceMemory: defaultSmallCHMemoryRequest,
						},
					},
				}

				result, err := BuildClickHouseConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideCHCpuRequest))
				Expect(result.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallCHMemoryRequest))
			})
		})
	})

	Describe("Field merging", func() {
		Context("when actual fields are empty", func() {
			It("should use default values", func() {
				actual := apiv2.WBClickHouseSpec{
					Version:     "",
					StorageSize: "",
					Namespace:   "",
				}
				defaultConfig := common.ClickHouseConfig{
					Version:     defaults.ClickHouseVersion,
					StorageSize: defaults.SmallClickHouseStorageSize,
					Namespace:   "default-ns",
				}

				result, err := BuildClickHouseConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Version).To(Equal(defaults.ClickHouseVersion))
				Expect(result.StorageSize).To(Equal(defaults.SmallClickHouseStorageSize))
				Expect(result.Namespace).To(Equal("default-ns"))
			})
		})

		Context("when actual fields are set", func() {
			It("should use actual values", func() {
				actual := apiv2.WBClickHouseSpec{
					Version:     overrideCHVersion,
					StorageSize: overrideCHStorageSize,
					Namespace:   overrideCHNamespace,
				}
				defaultConfig := common.ClickHouseConfig{
					Version:     defaults.ClickHouseVersion,
					StorageSize: defaults.SmallClickHouseStorageSize,
					Namespace:   "default-ns",
				}

				result, err := BuildClickHouseConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Version).To(Equal(overrideCHVersion))
				Expect(result.StorageSize).To(Equal(overrideCHStorageSize))
				Expect(result.Namespace).To(Equal(overrideCHNamespace))
			})
		})
	})

	Describe("Enabled and Replicas fields", func() {
		It("should always use actual Enabled and Replicas", func() {
			actual := apiv2.WBClickHouseSpec{
				Enabled:  overrideCHEnabled,
				Replicas: overrideCHReplicas,
			}
			defaultConfig := common.ClickHouseConfig{
				Enabled:  true,
				Replicas: 3,
			}

			result, err := BuildClickHouseConfig(actual, defaultConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Enabled).To(Equal(overrideCHEnabled))
			Expect(result.Replicas).To(Equal(overrideCHReplicas))
		})
	})
})

var _ = Describe("InfraConfigBuilder.AddClickHouseConfig", func() {
	const testOwnerNamespace = "test-namespace"

	Context("when adding dev size spec", func() {
		It("should merge actual with dev defaults from common", func() {
			actual := apiv2.WBClickHouseSpec{
				Enabled:  true,
				Replicas: 1,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeDev)
			result := builder.AddClickHouseConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedClickHouse.Enabled).To(BeTrue())
			Expect(builder.mergedClickHouse.Namespace).To(Equal(testOwnerNamespace))
			Expect(builder.mergedClickHouse.StorageSize).To(Equal(defaults.DevClickHouseStorageSize))
			Expect(builder.mergedClickHouse.Replicas).To(Equal(int32(1)))
			Expect(builder.mergedClickHouse.Version).To(Equal(defaults.ClickHouseVersion))
		})
	})

	Context("when adding small size spec with empty actual", func() {
		It("should use all small defaults including resources", func() {
			actual := apiv2.WBClickHouseSpec{
				Enabled:  true,
				Replicas: 3,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddClickHouseConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedClickHouse.Enabled).To(BeTrue())
			Expect(builder.mergedClickHouse.Replicas).To(Equal(int32(3)))
			Expect(builder.mergedClickHouse.StorageSize).To(Equal(defaults.SmallClickHouseStorageSize))
			Expect(builder.mergedClickHouse.Version).To(Equal(defaults.ClickHouseVersion))
			Expect(builder.mergedClickHouse.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallCHCpuRequest))
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
			result := builder.AddClickHouseConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())

			Expect(builder.mergedClickHouse.Enabled).To(Equal(overrideCHEnabled))
			Expect(builder.mergedClickHouse.Enabled).ToNot(Equal(true))

			Expect(builder.mergedClickHouse.Namespace).To(Equal(overrideCHNamespace))
			Expect(builder.mergedClickHouse.Namespace).ToNot(Equal(testOwnerNamespace))

			Expect(builder.mergedClickHouse.StorageSize).To(Equal(overrideCHStorageSize))
			Expect(builder.mergedClickHouse.StorageSize).ToNot(Equal(defaults.SmallClickHouseStorageSize))

			Expect(builder.mergedClickHouse.Version).To(Equal(overrideCHVersion))
			Expect(builder.mergedClickHouse.Version).ToNot(Equal(defaults.ClickHouseVersion))

			Expect(builder.mergedClickHouse.Replicas).To(Equal(overrideCHReplicas))
			Expect(builder.mergedClickHouse.Replicas).ToNot(Equal(int32(3)))

			Expect(builder.mergedClickHouse.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideCHCpuRequest))
			Expect(builder.mergedClickHouse.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallCHCpuRequest))

			Expect(builder.mergedClickHouse.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideCHMemoryRequest))
			Expect(builder.mergedClickHouse.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallCHMemoryRequest))

			Expect(builder.mergedClickHouse.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideCHCpuLimit))
			Expect(builder.mergedClickHouse.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallCHCpuLimit))

			Expect(builder.mergedClickHouse.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideCHMemoryLimit))
			Expect(builder.mergedClickHouse.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallCHMemoryLimit))
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
			result := builder.AddClickHouseConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedClickHouse.StorageSize).To(Equal(overrideCHStorageSize))
			Expect(builder.mergedClickHouse.StorageSize).ToNot(Equal(defaults.SmallClickHouseStorageSize))
			Expect(builder.mergedClickHouse.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallCHCpuRequest))
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
			result := builder.AddClickHouseConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedClickHouse.Version).To(Equal(overrideCHVersion))
			Expect(builder.mergedClickHouse.Version).ToNot(Equal(defaults.ClickHouseVersion))
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
			result := builder.AddClickHouseConfig(actual)

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
			result := builder.AddClickHouseConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())

			Expect(builder.mergedClickHouse.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideCHCpuRequest))
			Expect(builder.mergedClickHouse.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallCHCpuRequest))

			Expect(builder.mergedClickHouse.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideCHMemoryRequest))
			Expect(builder.mergedClickHouse.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallCHMemoryRequest))

			Expect(builder.mergedClickHouse.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideCHCpuLimit))
			Expect(builder.mergedClickHouse.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallCHCpuLimit))

			Expect(builder.mergedClickHouse.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideCHMemoryLimit))
			Expect(builder.mergedClickHouse.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallCHMemoryLimit))
		})
	})

	Context("when size is invalid", func() {
		It("should append error to builder", func() {
			actual := apiv2.WBClickHouseSpec{Enabled: true}
			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSize("invalid"))
			result := builder.AddClickHouseConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).ToNot(BeEmpty())
		})
	})
})

var _ = Describe("TranslateClickHouseSpec", func() {
	Context("when translating a complete ClickHouse spec", func() {
		It("should correctly map all fields to common.ClickHouseConfig", func() {
			spec := apiv2.WBClickHouseSpec{
				Enabled:     true,
				Namespace:   overrideCHNamespace,
				StorageSize: overrideCHStorageSize,
				Replicas:    overrideCHReplicas,
				Version:     overrideCHVersion,
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

			config := TranslateClickHouseSpec(spec)

			Expect(config.Enabled).To(Equal(spec.Enabled))
			Expect(config.Namespace).To(Equal(spec.Namespace))
			Expect(config.StorageSize).To(Equal(spec.StorageSize))
			Expect(config.Replicas).To(Equal(spec.Replicas))
			Expect(config.Version).To(Equal(spec.Version))
			Expect(config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideCHCpuRequest))
			Expect(config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideCHCpuLimit))
			Expect(config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideCHMemoryRequest))
			Expect(config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideCHMemoryLimit))
		})
	})

	Context("when translating a minimal ClickHouse spec", func() {
		It("should handle nil Config", func() {
			spec := apiv2.WBClickHouseSpec{
				Enabled:     overrideCHEnabled,
				Namespace:   overrideCHNamespace,
				StorageSize: defaults.DevClickHouseStorageSize,
				Replicas:    1,
				Version:     defaults.ClickHouseVersion,
				Config:      nil,
			}

			config := TranslateClickHouseSpec(spec)

			Expect(config.Enabled).To(Equal(spec.Enabled))
			Expect(config.Namespace).To(Equal(spec.Namespace))
			Expect(config.StorageSize).To(Equal(spec.StorageSize))
			Expect(config.Replicas).To(Equal(spec.Replicas))
			Expect(config.Version).To(Equal(spec.Version))
			Expect(config.Resources).To(Equal(corev1.ResourceRequirements{}))
		})
	})
})

var _ = Describe("TranslateClickHouseStatus", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("when common status has no errors or details", func() {
		It("should return ready status when Ready is true", func() {
			modelStatus := common.ClickHouseStatus{
				Ready: true,
				Connection: common.ClickHouseConnection{
					Host: "ch.example.com",
					Port: "9000",
					User: "admin",
				},
			}

			result := TranslateClickHouseStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeTrue())
			Expect(result.State).To(Equal(apiv2.WBStateReady))
			Expect(result.Details).To(BeEmpty())
			Expect(result.Connection.ClickHouseHost).To(Equal("ch.example.com"))
			Expect(result.Connection.ClickHousePort).To(Equal("9000"))
			Expect(result.Connection.ClickHouseUser).To(Equal("admin"))
			Expect(result.LastReconciled.IsZero()).To(BeFalse())
		})

		It("should return unknown status when Ready is false", func() {
			modelStatus := common.ClickHouseStatus{
				Ready: false,
			}

			result := TranslateClickHouseStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateUnknown))
			Expect(result.Details).To(BeEmpty())
		})
	})

	Context("when common status has errors", func() {
		It("should translate errors to status details with Error state", func() {
			modelStatus := common.ClickHouseStatus{
				Ready: false,
				Errors: []common.ClickHouseInfraError{
					{InfraError: common.NewClickHouseError(common.ClickHouseErrFailedToCreateCode, "creation failed")},
					{InfraError: common.NewClickHouseError(common.ClickHouseErrFailedToUpdateCode, "update failed")},
				},
			}

			result := TranslateClickHouseStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateError))
			Expect(result.Details).To(HaveLen(2))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateError))
			Expect(result.Details[0].Code).To(Equal(string(common.ClickHouseErrFailedToCreateCode)))
			Expect(result.Details[0].Message).To(Equal("creation failed"))
			Expect(result.Details[1].State).To(Equal(apiv2.WBStateError))
			Expect(result.Details[1].Code).To(Equal(string(common.ClickHouseErrFailedToUpdateCode)))
			Expect(result.Details[1].Message).To(Equal("update failed"))
		})
	})

	Context("when common status has status details", func() {
		It("should translate ClickHouseCreated to Updating state", func() {
			modelStatus := common.ClickHouseStatus{
				Ready: false,
				Details: []common.ClickHouseStatusDetail{
					{InfraStatusDetail: common.NewClickHouseStatusDetail(common.ClickHouseCreatedCode, "ClickHouse created")},
				},
			}

			result := TranslateClickHouseStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(1))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.Details[0].Code).To(Equal(string(common.ClickHouseCreatedCode)))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate ClickHouseUpdated to Updating state", func() {
			modelStatus := common.ClickHouseStatus{
				Ready: false,
				Details: []common.ClickHouseStatusDetail{
					{InfraStatusDetail: common.NewClickHouseStatusDetail(common.ClickHouseUpdatedCode, "ClickHouse updated")},
				},
			}

			result := TranslateClickHouseStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate ClickHouseDeleted to Deleting state", func() {
			modelStatus := common.ClickHouseStatus{
				Ready: false,
				Details: []common.ClickHouseStatusDetail{
					{InfraStatusDetail: common.NewClickHouseStatusDetail(common.ClickHouseDeletedCode, "ClickHouse deleted")},
				},
			}

			result := TranslateClickHouseStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateDeleting))
			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})

		It("should translate ClickHouseConnection to Ready state", func() {
			modelStatus := common.ClickHouseStatus{
				Ready: true,
				Details: []common.ClickHouseStatusDetail{
					{InfraStatusDetail: common.NewClickHouseStatusDetail(common.ClickHouseConnectionCode, "connection established")},
				},
			}

			result := TranslateClickHouseStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateReady))
			Expect(result.State).To(Equal(apiv2.WBStateReady))
		})
	})

	Context("when common status has both errors and details", func() {
		It("should use worst state according to WorseThan", func() {
			modelStatus := common.ClickHouseStatus{
				Ready: false,
				Errors: []common.ClickHouseInfraError{
					{InfraError: common.NewClickHouseError(common.ClickHouseErrFailedToCreateCode, "creation failed")},
				},
				Details: []common.ClickHouseStatusDetail{
					{InfraStatusDetail: common.NewClickHouseStatusDetail(common.ClickHouseCreatedCode, "ClickHouse created")},
				},
			}

			result := TranslateClickHouseStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(2))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})
	})

	Context("when common status has multiple details with different states", func() {
		It("should compute worst state correctly", func() {
			modelStatus := common.ClickHouseStatus{
				Ready: false,
				Details: []common.ClickHouseStatusDetail{
					{InfraStatusDetail: common.NewClickHouseStatusDetail(common.ClickHouseUpdatedCode, "updating")},
					{InfraStatusDetail: common.NewClickHouseStatusDetail(common.ClickHouseDeletedCode, "deleting")},
				},
			}

			result := TranslateClickHouseStatus(ctx, modelStatus)

			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})
	})
})
