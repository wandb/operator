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
	defaultSmallMySQLCpuRequest    = resource.MustParse(model.SmallMySQLCpuRequest)
	defaultSmallMySQLCpuLimit      = resource.MustParse(model.SmallMySQLCpuLimit)
	defaultSmallMySQLMemoryRequest = resource.MustParse(model.SmallMySQLMemoryRequest)
	defaultSmallMySQLMemoryLimit   = resource.MustParse(model.SmallMySQLMemoryLimit)

	overrideMySQLStorageSize   = "25Gi"
	overrideMySQLNamespace     = "custom-namespace"
	overrideMySQLEnabled       = false
	overrideMySQLCpuRequest    = resource.MustParse("750m")
	overrideMySQLCpuLimit      = resource.MustParse("1500m")
	overrideMySQLMemoryRequest = resource.MustParse("1.5Gi")
	overrideMySQLMemoryLimit   = resource.MustParse("3Gi")
)

var _ = Describe("BuildMySQLConfig", func() {
	Describe("Config merging", func() {
		Context("when actual Config is nil", func() {
			It("should use default Config resources", func() {
				actual := apiv2.WBMySQLSpec{Config: nil}
				defaultConfig := model.MySQLConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    defaultSmallMySQLCpuRequest,
							corev1.ResourceMemory: defaultSmallMySQLMemoryRequest,
						},
					},
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallMySQLCpuRequest))
				Expect(result.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallMySQLMemoryRequest))
			})
		})

		Context("when actual Config exists", func() {
			It("should use actual Config resources and merge with defaults", func() {
				actual := apiv2.WBMySQLSpec{
					Config: &apiv2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: overrideMySQLCpuRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: overrideMySQLMemoryLimit,
							},
						},
					},
				}
				defaultConfig := model.MySQLConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    defaultSmallMySQLCpuRequest,
							corev1.ResourceMemory: defaultSmallMySQLMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: defaultSmallMySQLCpuLimit,
						},
					},
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuRequest))
				Expect(result.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallMySQLMemoryRequest))
				Expect(result.Resources.Limits[corev1.ResourceCPU]).To(Equal(defaultSmallMySQLCpuLimit))
				Expect(result.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryLimit))
			})
		})
	})

	Describe("StorageSize merging", func() {
		Context("when actual StorageSize is empty", func() {
			It("should use default StorageSize", func() {
				actual := apiv2.WBMySQLSpec{
					StorageSize: "",
				}
				defaultConfig := model.MySQLConfig{
					StorageSize: model.SmallMySQLStorageSize,
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(model.SmallMySQLStorageSize))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actual := apiv2.WBMySQLSpec{
					StorageSize: overrideMySQLStorageSize,
				}
				defaultConfig := model.MySQLConfig{
					StorageSize: model.SmallMySQLStorageSize,
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(overrideMySQLStorageSize))
			})
		})

		Context("when both StorageSize values are empty", func() {
			It("should result in empty StorageSize", func() {
				actual := apiv2.WBMySQLSpec{
					StorageSize: "",
				}
				defaultConfig := model.MySQLConfig{
					StorageSize: "",
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(""))
			})
		})
	})

	Describe("Namespace merging", func() {
		Context("when actual Namespace is empty", func() {
			It("should use default Namespace", func() {
				actual := apiv2.WBMySQLSpec{
					Namespace: "",
				}
				defaultConfig := model.MySQLConfig{
					Namespace: overrideMySQLNamespace,
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideMySQLNamespace))
			})
		})

		Context("when actual Namespace is set", func() {
			It("should use actual Namespace", func() {
				actual := apiv2.WBMySQLSpec{
					Namespace: overrideMySQLNamespace,
				}
				defaultConfig := model.MySQLConfig{
					Namespace: "default-namespace",
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideMySQLNamespace))
			})
		})

		Context("when both Namespace values are empty", func() {
			It("should result in empty Namespace", func() {
				actual := apiv2.WBMySQLSpec{
					Namespace: "",
				}
				defaultConfig := model.MySQLConfig{
					Namespace: "",
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(""))
			})
		})
	})

	Describe("Enabled field", func() {
		Context("when actual Enabled is true", func() {
			It("should always use actual Enabled regardless of default", func() {
				actual := apiv2.WBMySQLSpec{
					Enabled: true,
				}
				defaultConfig := model.MySQLConfig{
					Enabled: false,
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
			})
		})

		Context("when actual Enabled is false", func() {
			It("should always use actual Enabled regardless of default", func() {
				actual := apiv2.WBMySQLSpec{
					Enabled: false,
				}
				defaultConfig := model.MySQLConfig{
					Enabled: true,
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
			})
		})
	})

	Describe("Complete spec merging", func() {
		Context("when actual is completely empty", func() {
			It("should return all default values except Enabled", func() {
				actual := apiv2.WBMySQLSpec{}
				defaultConfig := model.MySQLConfig{
					Enabled:     true,
					Namespace:   overrideMySQLNamespace,
					StorageSize: model.SmallMySQLStorageSize,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: defaultSmallMySQLCpuRequest,
						},
					},
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal(overrideMySQLNamespace))
				Expect(result.StorageSize).To(Equal(model.SmallMySQLStorageSize))
				Expect(result.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallMySQLCpuRequest))
			})
		})

		Context("when actual has all values set", func() {
			It("should use actual values for all fields", func() {
				actual := apiv2.WBMySQLSpec{
					Enabled:     overrideMySQLEnabled,
					Namespace:   overrideMySQLNamespace,
					StorageSize: overrideMySQLStorageSize,
					Config: &apiv2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    overrideMySQLCpuRequest,
								corev1.ResourceMemory: overrideMySQLMemoryRequest,
							},
						},
					},
				}
				defaultConfig := model.MySQLConfig{
					Enabled:     true,
					Namespace:   "default-namespace",
					StorageSize: model.SmallMySQLStorageSize,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: defaultSmallMySQLCpuRequest,
						},
					},
				}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(Equal(overrideMySQLEnabled))
				Expect(result.Namespace).To(Equal(overrideMySQLNamespace))
				Expect(result.StorageSize).To(Equal(overrideMySQLStorageSize))
				Expect(result.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuRequest))
				Expect(result.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryRequest))
			})
		})
	})

	Describe("Edge cases", func() {
		Context("when both specs are completely empty", func() {
			It("should return an empty config without error", func() {
				actual := apiv2.WBMySQLSpec{}
				defaultConfig := model.MySQLConfig{}

				result, err := BuildMySQLConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal(""))
				Expect(result.StorageSize).To(Equal(""))
			})
		})
	})
})

var _ = Describe("InfraConfigBuilder.AddMySQLConfig", func() {
	const testOwnerNamespace = "test-namespace"

	Context("when adding dev size spec", func() {
		It("should merge actual with dev defaults from model", func() {
			actual := apiv2.WBMySQLSpec{
				Enabled: true,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeDev)
			result := builder.AddMySQLConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMySQL.Enabled).To(BeTrue())
			Expect(builder.mergedMySQL.Namespace).To(Equal(testOwnerNamespace))
			Expect(builder.mergedMySQL.StorageSize).To(Equal(model.DevMySQLStorageSize))
		})
	})

	Context("when adding small size spec with all overrides", func() {
		It("should use all overrides and verify they differ from defaults", func() {
			actual := apiv2.WBMySQLSpec{
				Enabled:     overrideMySQLEnabled,
				Namespace:   overrideMySQLNamespace,
				StorageSize: overrideMySQLStorageSize,
				Config: &apiv2.WBMySQLConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideMySQLCpuRequest,
							corev1.ResourceMemory: overrideMySQLMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideMySQLCpuLimit,
							corev1.ResourceMemory: overrideMySQLMemoryLimit,
						},
					},
				},
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMySQLConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())

			Expect(builder.mergedMySQL.Enabled).To(Equal(overrideMySQLEnabled))
			Expect(builder.mergedMySQL.Enabled).ToNot(Equal(true))

			Expect(builder.mergedMySQL.Namespace).To(Equal(overrideMySQLNamespace))
			Expect(builder.mergedMySQL.Namespace).ToNot(Equal(testOwnerNamespace))

			Expect(builder.mergedMySQL.StorageSize).To(Equal(overrideMySQLStorageSize))
			Expect(builder.mergedMySQL.StorageSize).ToNot(Equal(model.SmallMySQLStorageSize))

			Expect(builder.mergedMySQL.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuRequest))
			Expect(builder.mergedMySQL.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMySQLCpuRequest))

			Expect(builder.mergedMySQL.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryRequest))
			Expect(builder.mergedMySQL.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMySQLMemoryRequest))

			Expect(builder.mergedMySQL.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuLimit))
			Expect(builder.mergedMySQL.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMySQLCpuLimit))

			Expect(builder.mergedMySQL.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryLimit))
			Expect(builder.mergedMySQL.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMySQLMemoryLimit))
		})
	})

	Context("when adding small size spec with storage override only", func() {
		It("should use override storage and verify it differs from default", func() {
			actual := apiv2.WBMySQLSpec{
				Enabled:     true,
				StorageSize: overrideMySQLStorageSize,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMySQLConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMySQL.StorageSize).To(Equal(overrideMySQLStorageSize))
			Expect(builder.mergedMySQL.StorageSize).ToNot(Equal(model.SmallMySQLStorageSize))
		})
	})

	Context("when adding small size spec with namespace override only", func() {
		It("should use override namespace and verify it differs from default", func() {
			actual := apiv2.WBMySQLSpec{
				Enabled:   true,
				Namespace: overrideMySQLNamespace,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMySQLConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMySQL.Namespace).To(Equal(overrideMySQLNamespace))
			Expect(builder.mergedMySQL.Namespace).ToNot(Equal(testOwnerNamespace))
		})
	})

	Context("when adding small size spec with resource overrides only", func() {
		It("should use override resources and verify they differ from defaults", func() {
			actual := apiv2.WBMySQLSpec{
				Enabled: true,
				Config: &apiv2.WBMySQLConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideMySQLCpuRequest,
							corev1.ResourceMemory: overrideMySQLMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideMySQLCpuLimit,
							corev1.ResourceMemory: overrideMySQLMemoryLimit,
						},
					},
				},
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMySQLConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())

			Expect(builder.mergedMySQL.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuRequest))
			Expect(builder.mergedMySQL.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMySQLCpuRequest))

			Expect(builder.mergedMySQL.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryRequest))
			Expect(builder.mergedMySQL.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMySQLMemoryRequest))

			Expect(builder.mergedMySQL.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuLimit))
			Expect(builder.mergedMySQL.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMySQLCpuLimit))

			Expect(builder.mergedMySQL.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryLimit))
			Expect(builder.mergedMySQL.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMySQLMemoryLimit))
		})
	})

	Context("when size is invalid", func() {
		It("should append error to builder", func() {
			actual := apiv2.WBMySQLSpec{Enabled: true}
			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSize("invalid"))
			result := builder.AddMySQLConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).ToNot(BeEmpty())
		})
	})
})

var _ = Describe("TranslateMySQLSpec", func() {
	Context("when translating a complete MySQL spec", func() {
		It("should correctly map all fields to model.MySQLConfig", func() {
			spec := apiv2.WBMySQLSpec{
				Enabled:     true,
				Namespace:   overrideMySQLNamespace,
				StorageSize: overrideMySQLStorageSize,
				Config: &apiv2.WBMySQLConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideMySQLCpuRequest,
							corev1.ResourceMemory: overrideMySQLMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideMySQLCpuLimit,
							corev1.ResourceMemory: overrideMySQLMemoryLimit,
						},
					},
				},
			}

			config := TranslateMySQLSpec(spec)

			Expect(config.Enabled).To(Equal(spec.Enabled))
			Expect(config.Namespace).To(Equal(spec.Namespace))
			Expect(config.StorageSize).To(Equal(spec.StorageSize))
			Expect(config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuRequest))
			Expect(config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryLimit))
		})
	})
})

var _ = Describe("TranslateMySQLStatus", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("when model status has no errors or details", func() {
		It("should return ready status when Ready is true", func() {
			modelStatus := model.MySQLStatus{
				Ready: true,
				Connection: model.MySQLConnection{
					Host: "mysql.example.com",
					Port: "3306",
					User: "admin",
				},
			}

			result := TranslateMySQLStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeTrue())
			Expect(result.State).To(Equal(apiv2.WBStateReady))
			Expect(result.Details).To(BeEmpty())
			Expect(result.Connection.MySQLHost).To(Equal("mysql.example.com"))
			Expect(result.Connection.MySQLPort).To(Equal("3306"))
			Expect(result.Connection.MySQLUser).To(Equal("admin"))
			Expect(result.LastReconciled.IsZero()).To(BeFalse())
		})

		It("should return unknown status when Ready is false", func() {
			modelStatus := model.MySQLStatus{
				Ready: false,
			}

			result := TranslateMySQLStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateUnknown))
			Expect(result.Details).To(BeEmpty())
		})
	})

	Context("when model status has errors", func() {
		It("should translate errors to status details with Error state", func() {
			modelStatus := model.MySQLStatus{
				Ready: false,
				Errors: []model.MySQLInfraError{
					{InfraError: model.NewMySQLError(model.MySQLErrFailedToCreateCode, "creation failed")},
					{InfraError: model.NewMySQLError(model.MySQLErrFailedToUpdateCode, "update failed")},
				},
			}

			result := TranslateMySQLStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateError))
			Expect(result.Details).To(HaveLen(2))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateError))
			Expect(result.Details[0].Code).To(Equal(string(model.MySQLErrFailedToCreateCode)))
			Expect(result.Details[0].Message).To(Equal("creation failed"))
			Expect(result.Details[1].State).To(Equal(apiv2.WBStateError))
			Expect(result.Details[1].Code).To(Equal(string(model.MySQLErrFailedToUpdateCode)))
			Expect(result.Details[1].Message).To(Equal("update failed"))
		})
	})

	Context("when model status has status details", func() {
		It("should translate MySQLCreated to Updating state", func() {
			modelStatus := model.MySQLStatus{
				Ready: false,
				Details: []model.MySQLStatusDetail{
					{InfraStatusDetail: model.NewMySQLStatusDetail(model.MySQLCreatedCode, "MySQL created")},
				},
			}

			result := TranslateMySQLStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(1))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.Details[0].Code).To(Equal(string(model.MySQLCreatedCode)))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate MySQLUpdated to Updating state", func() {
			modelStatus := model.MySQLStatus{
				Ready: false,
				Details: []model.MySQLStatusDetail{
					{InfraStatusDetail: model.NewMySQLStatusDetail(model.MySQLUpdatedCode, "MySQL updated")},
				},
			}

			result := TranslateMySQLStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate MySQLDeleted to Deleting state", func() {
			modelStatus := model.MySQLStatus{
				Ready: false,
				Details: []model.MySQLStatusDetail{
					{InfraStatusDetail: model.NewMySQLStatusDetail(model.MySQLDeletedCode, "MySQL deleted")},
				},
			}

			result := TranslateMySQLStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateDeleting))
			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})

		It("should translate MySQLConnection to Ready state", func() {
			modelStatus := model.MySQLStatus{
				Ready: true,
				Details: []model.MySQLStatusDetail{
					{InfraStatusDetail: model.NewMySQLStatusDetail(model.MySQLConnectionCode, "connection established")},
				},
			}

			result := TranslateMySQLStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateReady))
			Expect(result.State).To(Equal(apiv2.WBStateReady))
		})
	})

	Context("when model status has both errors and details", func() {
		It("should use worst state according to WorseThan", func() {
			modelStatus := model.MySQLStatus{
				Ready: false,
				Errors: []model.MySQLInfraError{
					{InfraError: model.NewMySQLError(model.MySQLErrFailedToCreateCode, "creation failed")},
				},
				Details: []model.MySQLStatusDetail{
					{InfraStatusDetail: model.NewMySQLStatusDetail(model.MySQLCreatedCode, "MySQL created")},
				},
			}

			result := TranslateMySQLStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(2))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})
	})

	Context("when model status has multiple details with different states", func() {
		It("should compute worst state correctly", func() {
			modelStatus := model.MySQLStatus{
				Ready: false,
				Details: []model.MySQLStatusDetail{
					{InfraStatusDetail: model.NewMySQLStatusDetail(model.MySQLUpdatedCode, "updating")},
					{InfraStatusDetail: model.NewMySQLStatusDetail(model.MySQLDeletedCode, "deleting")},
				},
			}

			result := TranslateMySQLStatus(ctx, modelStatus)

			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})
	})
})
