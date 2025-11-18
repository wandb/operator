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

var _ = Describe("BuildMySQLSpec", func() {
	Describe("Config merging", func() {
		Context("when both Config values are nil", func() {
			It("should result in nil Config", func() {
				actual := apiv2.WBMySQLSpec{
					Config: nil,
				}
				defaults := apiv2.WBMySQLSpec{
					Config: nil,
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).To(BeNil())
			})
		})

		Context("when actual Config is nil", func() {
			It("should use default Config", func() {
				actual := apiv2.WBMySQLSpec{
					Config: nil,
				}
				defaults := apiv2.WBMySQLSpec{
					Config: &apiv2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallMySQLCpuRequest,
								corev1.ResourceMemory: defaultSmallMySQLMemoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallMySQLCpuLimit,
								corev1.ResourceMemory: defaultSmallMySQLMemoryLimit,
							},
						},
					},
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallMySQLCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallMySQLMemoryRequest))
				Expect(result.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(defaultSmallMySQLCpuLimit))
				Expect(result.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(defaultSmallMySQLMemoryLimit))
			})
		})

		Context("when default Config is nil", func() {
			It("should use actual Config", func() {
				actual := apiv2.WBMySQLSpec{
					Config: &apiv2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: overrideMySQLCpuRequest,
							},
						},
					},
				}
				defaults := apiv2.WBMySQLSpec{
					Config: nil,
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuRequest))
			})
		})

		Context("when both Config values exist", func() {
			It("should merge resources with actual taking precedence", func() {
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
				defaults := apiv2.WBMySQLSpec{
					Config: &apiv2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallMySQLCpuRequest,
								corev1.ResourceMemory: defaultSmallMySQLMemoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallMySQLCpuLimit,
								corev1.ResourceMemory: defaultSmallMySQLMemoryLimit,
							},
						},
					},
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallMySQLMemoryRequest))
				Expect(result.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(defaultSmallMySQLCpuLimit))
				Expect(result.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryLimit))
			})
		})
	})

	Describe("StorageSize merging", func() {
		Context("when actual StorageSize is empty", func() {
			It("should use default StorageSize", func() {
				actual := apiv2.WBMySQLSpec{
					StorageSize: "",
				}
				defaults := apiv2.WBMySQLSpec{
					StorageSize: model.SmallMySQLStorageSize,
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(model.SmallMySQLStorageSize))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actual := apiv2.WBMySQLSpec{
					StorageSize: overrideMySQLStorageSize,
				}
				defaults := apiv2.WBMySQLSpec{
					StorageSize: model.SmallMySQLStorageSize,
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(overrideMySQLStorageSize))
			})
		})

		Context("when both StorageSize values are empty", func() {
			It("should result in empty StorageSize", func() {
				actual := apiv2.WBMySQLSpec{
					StorageSize: "",
				}
				defaults := apiv2.WBMySQLSpec{
					StorageSize: "",
				}

				result, err := BuildMySQLSpec(actual, defaults)
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
				defaults := apiv2.WBMySQLSpec{
					Namespace: overrideMySQLNamespace,
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideMySQLNamespace))
			})
		})

		Context("when actual Namespace is set", func() {
			It("should use actual Namespace", func() {
				actual := apiv2.WBMySQLSpec{
					Namespace: overrideMySQLNamespace,
				}
				defaults := apiv2.WBMySQLSpec{
					Namespace: "default-namespace",
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideMySQLNamespace))
			})
		})

		Context("when both Namespace values are empty", func() {
			It("should result in empty Namespace", func() {
				actual := apiv2.WBMySQLSpec{
					Namespace: "",
				}
				defaults := apiv2.WBMySQLSpec{
					Namespace: "",
				}

				result, err := BuildMySQLSpec(actual, defaults)
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
				defaults := apiv2.WBMySQLSpec{
					Enabled: false,
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
			})
		})

		Context("when actual Enabled is false", func() {
			It("should always use actual Enabled regardless of default", func() {
				actual := apiv2.WBMySQLSpec{
					Enabled: false,
				}
				defaults := apiv2.WBMySQLSpec{
					Enabled: true,
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
			})
		})
	})

	Describe("Complete spec merging", func() {
		Context("when actual is completely empty", func() {
			It("should return all default values except Enabled", func() {
				actual := apiv2.WBMySQLSpec{}
				defaults := apiv2.WBMySQLSpec{
					Enabled:     true,
					Namespace:   overrideMySQLNamespace,
					StorageSize: model.SmallMySQLStorageSize,
					Config: &apiv2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: defaultSmallMySQLCpuRequest,
							},
						},
					},
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal(overrideMySQLNamespace))
				Expect(result.StorageSize).To(Equal(model.SmallMySQLStorageSize))
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallMySQLCpuRequest))
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
				defaults := apiv2.WBMySQLSpec{
					Enabled:     true,
					Namespace:   "default-namespace",
					StorageSize: model.SmallMySQLStorageSize,
					Config: &apiv2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: defaultSmallMySQLCpuRequest,
							},
						},
					},
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(Equal(overrideMySQLEnabled))
				Expect(result.Namespace).To(Equal(overrideMySQLNamespace))
				Expect(result.StorageSize).To(Equal(overrideMySQLStorageSize))
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryRequest))
			})
		})
	})

	Describe("Edge cases", func() {
		Context("when both specs are completely empty", func() {
			It("should return an empty spec without error", func() {
				actual := apiv2.WBMySQLSpec{}
				defaults := apiv2.WBMySQLSpec{}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal(""))
				Expect(result.StorageSize).To(Equal(""))
				Expect(result.Config).To(BeNil())
			})
		})
	})
})

var _ = Describe("InfraConfigBuilder.AddMySQLSpec", func() {
	const testOwnerNamespace = "test-namespace"

	Context("when adding dev size spec", func() {
		It("should merge actual with dev defaults from model", func() {
			actual := apiv2.WBMySQLSpec{
				Enabled: true,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeDev)
			result := builder.AddMySQLSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMySQL).ToNot(BeNil())
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
			result := builder.AddMySQLSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMySQL).ToNot(BeNil())

			Expect(builder.mergedMySQL.Enabled).To(Equal(overrideMySQLEnabled))
			Expect(builder.mergedMySQL.Enabled).ToNot(Equal(true))

			Expect(builder.mergedMySQL.Namespace).To(Equal(overrideMySQLNamespace))
			Expect(builder.mergedMySQL.Namespace).ToNot(Equal(testOwnerNamespace))

			Expect(builder.mergedMySQL.StorageSize).To(Equal(overrideMySQLStorageSize))
			Expect(builder.mergedMySQL.StorageSize).ToNot(Equal(model.SmallMySQLStorageSize))

			Expect(builder.mergedMySQL.Config).ToNot(BeNil())
			Expect(builder.mergedMySQL.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuRequest))
			Expect(builder.mergedMySQL.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMySQLCpuRequest))

			Expect(builder.mergedMySQL.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryRequest))
			Expect(builder.mergedMySQL.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMySQLMemoryRequest))

			Expect(builder.mergedMySQL.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuLimit))
			Expect(builder.mergedMySQL.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMySQLCpuLimit))

			Expect(builder.mergedMySQL.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryLimit))
			Expect(builder.mergedMySQL.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMySQLMemoryLimit))
		})
	})

	Context("when adding small size spec with storage override only", func() {
		It("should use override storage and verify it differs from default", func() {
			actual := apiv2.WBMySQLSpec{
				Enabled:     true,
				StorageSize: overrideMySQLStorageSize,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMySQLSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMySQL.StorageSize).To(Equal(overrideMySQLStorageSize))
			Expect(builder.mergedMySQL.StorageSize).ToNot(Equal(model.SmallMySQLStorageSize))
			Expect(builder.mergedMySQL.Config).ToNot(BeNil())
		})
	})

	Context("when adding small size spec with namespace override only", func() {
		It("should use override namespace and verify it differs from default", func() {
			actual := apiv2.WBMySQLSpec{
				Enabled:   true,
				Namespace: overrideMySQLNamespace,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddMySQLSpec(actual)

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
			result := builder.AddMySQLSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedMySQL.Config).ToNot(BeNil())

			Expect(builder.mergedMySQL.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuRequest))
			Expect(builder.mergedMySQL.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMySQLCpuRequest))

			Expect(builder.mergedMySQL.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryRequest))
			Expect(builder.mergedMySQL.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMySQLMemoryRequest))

			Expect(builder.mergedMySQL.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuLimit))
			Expect(builder.mergedMySQL.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallMySQLCpuLimit))

			Expect(builder.mergedMySQL.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryLimit))
			Expect(builder.mergedMySQL.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallMySQLMemoryLimit))
		})
	})

	Context("when size is invalid", func() {
		It("should append error to builder", func() {
			actual := apiv2.WBMySQLSpec{Enabled: true}
			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSize("invalid"))
			result := builder.AddMySQLSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).ToNot(BeEmpty())
		})
	})
})

var _ = Describe("TranslateMySQLConfig", func() {
	Context("when translating a complete MySQL config", func() {
		It("should correctly map all fields to WBMySQLSpec", func() {
			config := model.MySQLConfig{
				Enabled:     true,
				Namespace:   overrideMySQLNamespace,
				StorageSize: overrideMySQLStorageSize,
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
			}

			spec := TranslateMySQLConfig(config)

			Expect(spec.Enabled).To(Equal(config.Enabled))
			Expect(spec.Namespace).To(Equal(config.Namespace))
			Expect(spec.StorageSize).To(Equal(config.StorageSize))
			Expect(spec.Config).ToNot(BeNil())
			Expect(spec.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideMySQLCpuRequest))
			Expect(spec.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideMySQLMemoryLimit))
		})
	})
})
