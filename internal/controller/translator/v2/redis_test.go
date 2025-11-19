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
	defaultSmallRedisCpuRequest       = resource.MustParse(defaults.SmallReplicaCpuRequest)
	defaultSmallRedisCpuLimit         = resource.MustParse(defaults.SmallReplicaCpuLimit)
	defaultSmallRedisMemoryRequest    = resource.MustParse(defaults.SmallReplicaMemoryRequest)
	defaultSmallRedisMemoryLimit      = resource.MustParse(defaults.SmallReplicaMemoryLimit)
	defaultSmallSentinelCpuRequest    = resource.MustParse(defaults.SmallSentinelCpuRequest)
	defaultSmallSentinelCpuLimit      = resource.MustParse(defaults.SmallSentinelCpuLimit)
	defaultSmallSentinelMemoryRequest = resource.MustParse(defaults.SmallSentinelMemoryRequest)
	defaultSmallSentinelMemoryLimit   = resource.MustParse(defaults.SmallSentinelMemoryLimit)

	overrideRedisStorageSize      = "10Gi"
	overrideRedisNamespace        = "custom-namespace"
	overrideRedisEnabled          = false
	overrideRedisCpuRequest       = resource.MustParse("1")
	overrideRedisCpuLimit         = resource.MustParse("2")
	overrideRedisMemoryRequest    = resource.MustParse("2Gi")
	overrideRedisMemoryLimit      = resource.MustParse("4Gi")
	overrideSentinelEnabled       = false
	overrideSentinelMasterName    = "custom-master"
	overrideSentinelCpuRequest    = resource.MustParse("200m")
	overrideSentinelCpuLimit      = resource.MustParse("400m")
	overrideSentinelMemoryRequest = resource.MustParse("256Mi")
	overrideSentinelMemoryLimit   = resource.MustParse("512Mi")
)

var _ = Describe("BuildRedisConfig", func() {
	Describe("Sentinel merging", func() {
		Context("when actual Sentinel is nil", func() {
			It("should use default Sentinel", func() {
				actual := apiv2.WBRedisSpec{
					Sentinel: nil,
				}
				defaultConfig := common.RedisConfig{
					Sentinel: common.SentinelConfig{
						Enabled:         true,
						MasterGroupName: defaults.DefaultSentinelGroup,
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: defaultSmallSentinelCpuRequest,
						},
					},
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallSentinelCpuRequest))
			})
		})

		Context("when actual Sentinel exists", func() {
			It("should merge Sentinel config", func() {
				actual := apiv2.WBRedisSpec{
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &apiv2.WBRedisSentinelConfig{
							MasterName: overrideSentinelMasterName,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: overrideSentinelCpuRequest,
								},
							},
						},
					},
				}
				defaultConfig := common.RedisConfig{
					Sentinel: common.SentinelConfig{
						Enabled:         false,
						MasterGroupName: defaults.DefaultSentinelGroup,
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    defaultSmallSentinelCpuRequest,
							corev1.ResourceMemory: defaultSmallSentinelMemoryRequest,
						},
					},
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.MasterGroupName).To(Equal(overrideSentinelMasterName))
				Expect(result.Sentinel.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
				Expect(result.Sentinel.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallSentinelMemoryRequest))
			})
		})
	})

	Describe("Resources merging", func() {
		Context("when actual Config is nil", func() {
			It("should use default resources", func() {
				actual := apiv2.WBRedisSpec{
					Config: nil,
				}
				defaultConfig := common.RedisConfig{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    defaultSmallRedisCpuRequest,
						corev1.ResourceMemory: defaultSmallRedisMemoryRequest,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    defaultSmallRedisCpuLimit,
						corev1.ResourceMemory: defaultSmallRedisMemoryLimit,
					},
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallRedisCpuRequest))
				Expect(result.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallRedisMemoryRequest))
				Expect(result.Limits[corev1.ResourceCPU]).To(Equal(defaultSmallRedisCpuLimit))
				Expect(result.Limits[corev1.ResourceMemory]).To(Equal(defaultSmallRedisMemoryLimit))
			})
		})

		Context("when actual Config exists", func() {
			It("should merge resources with actual taking precedence", func() {
				actual := apiv2.WBRedisSpec{
					Config: &apiv2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: overrideRedisCpuRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: overrideRedisMemoryLimit,
							},
						},
					},
				}
				defaultConfig := common.RedisConfig{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    defaultSmallRedisCpuRequest,
						corev1.ResourceMemory: defaultSmallRedisMemoryRequest,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    defaultSmallRedisCpuLimit,
						corev1.ResourceMemory: defaultSmallRedisMemoryLimit,
					},
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requests[corev1.ResourceCPU]).To(Equal(overrideRedisCpuRequest))
				Expect(result.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallRedisMemoryRequest))
				Expect(result.Limits[corev1.ResourceCPU]).To(Equal(defaultSmallRedisCpuLimit))
				Expect(result.Limits[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryLimit))
			})
		})
	})

	Describe("StorageSize merging", func() {
		Context("when actual StorageSize is empty", func() {
			It("should use default StorageSize", func() {
				actual := apiv2.WBRedisSpec{
					StorageSize: "",
				}
				defaultConfig := common.RedisConfig{
					StorageSize: resource.MustParse(overrideRedisStorageSize),
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize.String()).To(Equal(overrideRedisStorageSize))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actual := apiv2.WBRedisSpec{
					StorageSize: overrideRedisStorageSize,
				}
				defaultConfig := common.RedisConfig{
					StorageSize: resource.MustParse(defaults.SmallStorageRequest),
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize.String()).To(Equal(overrideRedisStorageSize))
			})
		})

		Context("when both StorageSize values are empty", func() {
			It("should result in empty StorageSize", func() {
				actual := apiv2.WBRedisSpec{
					StorageSize: "",
				}
				defaultConfig := common.RedisConfig{
					StorageSize: resource.Quantity{},
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize.IsZero()).To(BeTrue())
			})
		})
	})

	Describe("Enabled field", func() {
		Context("when actual Enabled is true", func() {
			It("should always use actual Enabled regardless of default", func() {
				actual := apiv2.WBRedisSpec{
					Enabled: true,
				}
				defaultConfig := common.RedisConfig{
					Enabled: false,
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
			})
		})

		Context("when actual Enabled is false", func() {
			It("should always use actual Enabled regardless of default", func() {
				actual := apiv2.WBRedisSpec{
					Enabled: false,
				}
				defaultConfig := common.RedisConfig{
					Enabled: true,
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
			})
		})
	})

	Describe("Namespace field", func() {
		Context("when actual Namespace is set", func() {
			It("should always use actual Namespace regardless of default", func() {
				actual := apiv2.WBRedisSpec{
					Namespace: overrideRedisNamespace,
				}
				defaultConfig := common.RedisConfig{
					Namespace: "default-namespace",
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideRedisNamespace))
			})
		})

		Context("when actual Namespace is empty", func() {
			It("should use default Namespace", func() {
				actual := apiv2.WBRedisSpec{
					Namespace: "",
				}
				defaultConfig := common.RedisConfig{
					Namespace: overrideRedisNamespace,
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideRedisNamespace))
			})
		})
	})

	Describe("Complete spec merging", func() {
		Context("when actual is completely empty", func() {
			It("should return all default values except Enabled", func() {
				actual := apiv2.WBRedisSpec{}
				defaultConfig := common.RedisConfig{
					Enabled:     true,
					Namespace:   overrideRedisNamespace,
					StorageSize: resource.MustParse(overrideRedisStorageSize),
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: defaultSmallRedisCpuRequest,
					},
					Sentinel: common.SentinelConfig{
						Enabled:         true,
						MasterGroupName: defaults.DefaultSentinelGroup,
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: defaultSmallSentinelCpuRequest,
						},
					},
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal(overrideRedisNamespace))
				Expect(result.StorageSize.String()).To(Equal(overrideRedisStorageSize))
				Expect(result.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallRedisCpuRequest))
				Expect(result.Sentinel.Enabled).To(BeTrue())
			})
		})

		Context("when actual has all values set", func() {
			It("should use actual values for all fields", func() {
				actual := apiv2.WBRedisSpec{
					Enabled:     overrideRedisEnabled,
					Namespace:   overrideRedisNamespace,
					StorageSize: overrideRedisStorageSize,
					Config: &apiv2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    overrideRedisCpuRequest,
								corev1.ResourceMemory: overrideRedisMemoryRequest,
							},
						},
					},
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: overrideSentinelEnabled,
						Config: &apiv2.WBRedisSentinelConfig{
							MasterName: overrideSentinelMasterName,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: overrideSentinelCpuRequest,
								},
							},
						},
					},
				}
				defaultConfig := common.RedisConfig{
					Enabled:     true,
					Namespace:   "default-namespace",
					StorageSize: resource.MustParse(defaults.SmallStorageRequest),
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: defaultSmallRedisCpuRequest,
					},
					Sentinel: common.SentinelConfig{
						Enabled:         true,
						MasterGroupName: defaults.DefaultSentinelGroup,
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: defaultSmallSentinelCpuRequest,
						},
					},
				}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(Equal(overrideRedisEnabled))
				Expect(result.Namespace).To(Equal(overrideRedisNamespace))
				Expect(result.StorageSize.String()).To(Equal(overrideRedisStorageSize))
				Expect(result.Requests[corev1.ResourceCPU]).To(Equal(overrideRedisCpuRequest))
				Expect(result.Requests[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryRequest))
				Expect(result.Sentinel.Enabled).To(Equal(overrideSentinelEnabled))
				Expect(result.Sentinel.MasterGroupName).To(Equal(overrideSentinelMasterName))
				Expect(result.Sentinel.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
			})
		})
	})

	Describe("Edge cases", func() {
		Context("when both specs are completely empty", func() {
			It("should return an empty config without error", func() {
				actual := apiv2.WBRedisSpec{}
				defaultConfig := common.RedisConfig{}

				result, err := BuildRedisConfig(actual, defaultConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal(""))
				Expect(result.StorageSize.IsZero()).To(BeTrue())
				Expect(result.Sentinel.Enabled).To(BeFalse())
			})
		})
	})
})

var _ = Describe("RedisSentinelEnabled", func() {
	Context("when Sentinel is nil", func() {
		It("should return false", func() {
			spec := apiv2.WBRedisSpec{
				Sentinel: nil,
			}
			result := RedisSentinelEnabled(spec)
			Expect(result).To(BeFalse())
		})
	})

	Context("when Sentinel is disabled", func() {
		It("should return false", func() {
			spec := apiv2.WBRedisSpec{
				Sentinel: &apiv2.WBRedisSentinelSpec{
					Enabled: false,
				},
			}
			result := RedisSentinelEnabled(spec)
			Expect(result).To(BeFalse())
		})
	})

	Context("when Sentinel is enabled", func() {
		It("should return true", func() {
			spec := apiv2.WBRedisSpec{
				Sentinel: &apiv2.WBRedisSentinelSpec{
					Enabled: true,
				},
			}
			result := RedisSentinelEnabled(spec)
			Expect(result).To(BeTrue())
		})
	})
})

var _ = Describe("InfraConfigBuilder.AddRedisConfig", func() {
	const testOwnerNamespace = "test-namespace"

	Context("when adding dev size spec", func() {
		It("should merge actual with dev defaults from common", func() {
			actual := apiv2.WBRedisSpec{
				Enabled: true,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeDev)
			result := builder.AddRedisConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedRedis.Enabled).To(BeTrue())
			Expect(builder.mergedRedis.Namespace).To(Equal(testOwnerNamespace))
			Expect(builder.mergedRedis.StorageSize.String()).To(Equal(defaults.DevStorageRequest))
		})
	})

	Context("when adding small size spec with all overrides including sentinel", func() {
		It("should use all overrides and verify they differ from defaults", func() {
			actual := apiv2.WBRedisSpec{
				Enabled:     overrideRedisEnabled,
				Namespace:   overrideRedisNamespace,
				StorageSize: overrideRedisStorageSize,
				Config: &apiv2.WBRedisConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideRedisCpuRequest,
							corev1.ResourceMemory: overrideRedisMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideRedisCpuLimit,
							corev1.ResourceMemory: overrideRedisMemoryLimit,
						},
					},
				},
				Sentinel: &apiv2.WBRedisSentinelSpec{
					Enabled: overrideSentinelEnabled,
					Config: &apiv2.WBRedisSentinelConfig{
						MasterName: overrideSentinelMasterName,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    overrideSentinelCpuRequest,
								corev1.ResourceMemory: overrideSentinelMemoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    overrideSentinelCpuLimit,
								corev1.ResourceMemory: overrideSentinelMemoryLimit,
							},
						},
					},
				},
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddRedisConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())

			Expect(builder.mergedRedis.Enabled).To(Equal(overrideRedisEnabled))
			Expect(builder.mergedRedis.Enabled).ToNot(Equal(true))

			Expect(builder.mergedRedis.Namespace).To(Equal(overrideRedisNamespace))
			Expect(builder.mergedRedis.Namespace).ToNot(Equal(testOwnerNamespace))

			Expect(builder.mergedRedis.StorageSize.String()).To(Equal(overrideRedisStorageSize))
			Expect(builder.mergedRedis.StorageSize.String()).ToNot(Equal(defaults.SmallStorageRequest))

			Expect(builder.mergedRedis.Requests[corev1.ResourceCPU]).To(Equal(overrideRedisCpuRequest))
			Expect(builder.mergedRedis.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallRedisCpuRequest))

			Expect(builder.mergedRedis.Requests[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryRequest))
			Expect(builder.mergedRedis.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallRedisMemoryRequest))

			Expect(builder.mergedRedis.Limits[corev1.ResourceCPU]).To(Equal(overrideRedisCpuLimit))
			Expect(builder.mergedRedis.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallRedisCpuLimit))

			Expect(builder.mergedRedis.Limits[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryLimit))
			Expect(builder.mergedRedis.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallRedisMemoryLimit))

			Expect(builder.mergedRedis.Sentinel.Enabled).To(Equal(overrideSentinelEnabled))
			Expect(builder.mergedRedis.Sentinel.Enabled).ToNot(Equal(true))

			Expect(builder.mergedRedis.Sentinel.MasterGroupName).To(Equal(overrideSentinelMasterName))
			Expect(builder.mergedRedis.Sentinel.MasterGroupName).ToNot(Equal(defaults.DefaultSentinelGroup))

			Expect(builder.mergedRedis.Sentinel.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
			Expect(builder.mergedRedis.Sentinel.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallSentinelCpuRequest))

			Expect(builder.mergedRedis.Sentinel.Requests[corev1.ResourceMemory]).To(Equal(overrideSentinelMemoryRequest))
			Expect(builder.mergedRedis.Sentinel.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallSentinelMemoryRequest))

			Expect(builder.mergedRedis.Sentinel.Limits[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuLimit))
			Expect(builder.mergedRedis.Sentinel.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallSentinelCpuLimit))

			Expect(builder.mergedRedis.Sentinel.Limits[corev1.ResourceMemory]).To(Equal(overrideSentinelMemoryLimit))
			Expect(builder.mergedRedis.Sentinel.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallSentinelMemoryLimit))
		})
	})

	Context("when adding small size spec with storage override only", func() {
		It("should use override storage and verify it differs from default", func() {
			actual := apiv2.WBRedisSpec{
				Enabled:     true,
				StorageSize: overrideRedisStorageSize,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddRedisConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedRedis.StorageSize.String()).To(Equal(overrideRedisStorageSize))
			Expect(builder.mergedRedis.StorageSize.String()).ToNot(Equal(defaults.SmallStorageRequest))
		})
	})

	Context("when adding small size spec with namespace override only", func() {
		It("should use override namespace and verify it differs from default", func() {
			actual := apiv2.WBRedisSpec{
				Enabled:   true,
				Namespace: overrideRedisNamespace,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddRedisConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedRedis.Namespace).To(Equal(overrideRedisNamespace))
			Expect(builder.mergedRedis.Namespace).ToNot(Equal(testOwnerNamespace))
		})
	})

	Context("when adding small size spec with redis resource overrides only", func() {
		It("should use override resources and verify they differ from defaults", func() {
			actual := apiv2.WBRedisSpec{
				Enabled: true,
				Config: &apiv2.WBRedisConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideRedisCpuRequest,
							corev1.ResourceMemory: overrideRedisMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideRedisCpuLimit,
							corev1.ResourceMemory: overrideRedisMemoryLimit,
						},
					},
				},
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddRedisConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())

			Expect(builder.mergedRedis.Requests[corev1.ResourceCPU]).To(Equal(overrideRedisCpuRequest))
			Expect(builder.mergedRedis.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallRedisCpuRequest))

			Expect(builder.mergedRedis.Requests[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryRequest))
			Expect(builder.mergedRedis.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallRedisMemoryRequest))

			Expect(builder.mergedRedis.Limits[corev1.ResourceCPU]).To(Equal(overrideRedisCpuLimit))
			Expect(builder.mergedRedis.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallRedisCpuLimit))

			Expect(builder.mergedRedis.Limits[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryLimit))
			Expect(builder.mergedRedis.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallRedisMemoryLimit))
		})
	})

	Context("when adding small size spec with sentinel overrides only", func() {
		It("should use override sentinel config and verify it differs from defaults", func() {
			actual := apiv2.WBRedisSpec{
				Enabled: true,
				Sentinel: &apiv2.WBRedisSentinelSpec{
					Enabled: true,
					Config: &apiv2.WBRedisSentinelConfig{
						MasterName: overrideSentinelMasterName,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    overrideSentinelCpuRequest,
								corev1.ResourceMemory: overrideSentinelMemoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    overrideSentinelCpuLimit,
								corev1.ResourceMemory: overrideSentinelMemoryLimit,
							},
						},
					},
				},
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddRedisConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())

			Expect(builder.mergedRedis.Sentinel.MasterGroupName).To(Equal(overrideSentinelMasterName))
			Expect(builder.mergedRedis.Sentinel.MasterGroupName).ToNot(Equal(defaults.DefaultSentinelGroup))

			Expect(builder.mergedRedis.Sentinel.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
			Expect(builder.mergedRedis.Sentinel.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallSentinelCpuRequest))

			Expect(builder.mergedRedis.Sentinel.Requests[corev1.ResourceMemory]).To(Equal(overrideSentinelMemoryRequest))
			Expect(builder.mergedRedis.Sentinel.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallSentinelMemoryRequest))

			Expect(builder.mergedRedis.Sentinel.Limits[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuLimit))
			Expect(builder.mergedRedis.Sentinel.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallSentinelCpuLimit))

			Expect(builder.mergedRedis.Sentinel.Limits[corev1.ResourceMemory]).To(Equal(overrideSentinelMemoryLimit))
			Expect(builder.mergedRedis.Sentinel.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallSentinelMemoryLimit))
		})
	})

	Context("when size is invalid", func() {
		It("should append error to builder", func() {
			actual := apiv2.WBRedisSpec{Enabled: true}
			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSize("invalid"))
			result := builder.AddRedisConfig(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).ToNot(BeEmpty())
		})
	})
})

var _ = Describe("TranslateRedisSpec", func() {
	Context("when translating a Redis spec with sentinel", func() {
		It("should correctly map all fields including sentinel", func() {
			spec := apiv2.WBRedisSpec{
				Enabled:     true,
				Namespace:   overrideRedisNamespace,
				StorageSize: overrideRedisStorageSize,
				Config: &apiv2.WBRedisConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    overrideRedisCpuRequest,
							corev1.ResourceMemory: overrideRedisMemoryRequest,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    overrideRedisCpuLimit,
							corev1.ResourceMemory: overrideRedisMemoryLimit,
						},
					},
				},
				Sentinel: &apiv2.WBRedisSentinelSpec{
					Enabled: true,
					Config: &apiv2.WBRedisSentinelConfig{
						MasterName: overrideSentinelMasterName,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    overrideSentinelCpuRequest,
								corev1.ResourceMemory: overrideSentinelMemoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    overrideSentinelCpuLimit,
								corev1.ResourceMemory: overrideSentinelMemoryLimit,
							},
						},
					},
				},
			}

			config := TranslateRedisSpec(spec)

			Expect(config.Enabled).To(Equal(spec.Enabled))
			Expect(config.Namespace).To(Equal(spec.Namespace))
			Expect(config.StorageSize.String()).To(Equal(overrideRedisStorageSize))
			Expect(config.Requests[corev1.ResourceCPU]).To(Equal(overrideRedisCpuRequest))
			Expect(config.Limits[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryLimit))
			Expect(config.Sentinel.Enabled).To(BeTrue())
			Expect(config.Sentinel.MasterGroupName).To(Equal(overrideSentinelMasterName))
			Expect(config.Sentinel.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
		})
	})

	Context("when translating a Redis spec without sentinel", func() {
		It("should have sentinel disabled", func() {
			spec := apiv2.WBRedisSpec{
				Enabled:     true,
				Namespace:   overrideRedisNamespace,
				StorageSize: defaults.SmallStorageRequest,
				Config:      nil,
				Sentinel:    nil,
			}

			config := TranslateRedisSpec(spec)

			Expect(config.Enabled).To(Equal(spec.Enabled))
			Expect(config.Sentinel.Enabled).To(BeFalse())
		})
	})
})

var _ = Describe("TranslateRedisStatus", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("when common status has no errors or details", func() {
		It("should return ready status when Ready is true with Sentinel connection", func() {
			modelStatus := common.RedisStatus{
				Ready: true,
				Connection: common.RedisConnection{
					SentinelHost:   "redis-sentinel.example.com",
					SentinelPort:   "26379",
					SentinelMaster: "mymaster",
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeTrue())
			Expect(result.State).To(Equal(apiv2.WBStateReady))
			Expect(result.Details).To(BeEmpty())
			Expect(result.Connection.RedisSentinelHost).To(Equal("redis-sentinel.example.com"))
			Expect(result.Connection.RedisSentinelPort).To(Equal("26379"))
			Expect(result.Connection.RedisMasterName).To(Equal("mymaster"))
			Expect(result.LastReconciled.IsZero()).To(BeFalse())
		})

		It("should return ready status when Ready is true with Standalone connection", func() {
			modelStatus := common.RedisStatus{
				Ready: true,
				Connection: common.RedisConnection{
					RedisHost: "redis.example.com",
					RedisPort: "6379",
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeTrue())
			Expect(result.State).To(Equal(apiv2.WBStateReady))
			Expect(result.Details).To(BeEmpty())
			Expect(result.Connection.RedisHost).To(Equal("redis.example.com"))
			Expect(result.Connection.RedisPort).To(Equal("6379"))
			Expect(result.LastReconciled.IsZero()).To(BeFalse())
		})

		It("should return unknown status when Ready is false", func() {
			modelStatus := common.RedisStatus{
				Ready: false,
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateUnknown))
			Expect(result.Details).To(BeEmpty())
		})
	})

	Context("when common status has errors", func() {
		It("should translate errors to status details with Error state", func() {
			modelStatus := common.RedisStatus{
				Ready: false,
				Errors: []common.RedisInfraError{
					{InfraError: common.NewRedisError(common.RedisDeploymentConflictCode, "deployment conflict")},
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Ready).To(BeFalse())
			Expect(result.State).To(Equal(apiv2.WBStateError))
			Expect(result.Details).To(HaveLen(1))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateError))
			Expect(result.Details[0].Code).To(Equal(string(common.RedisDeploymentConflictCode)))
			Expect(result.Details[0].Message).To(Equal("deployment conflict"))
		})
	})

	Context("when common status has status details", func() {
		It("should translate RedisSentinelCreated to Updating state", func() {
			modelStatus := common.RedisStatus{
				Ready: false,
				Details: []common.RedisStatusDetail{
					{InfraStatusDetail: common.NewRedisStatusDetail(common.RedisSentinelCreatedCode, "Sentinel created")},
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(1))
			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.Details[0].Code).To(Equal(string(common.RedisSentinelCreatedCode)))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate RedisReplicationCreated to Updating state", func() {
			modelStatus := common.RedisStatus{
				Ready: false,
				Details: []common.RedisStatusDetail{
					{InfraStatusDetail: common.NewRedisStatusDetail(common.RedisReplicationCreatedCode, "Replication created")},
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate RedisStandaloneCreated to Updating state", func() {
			modelStatus := common.RedisStatus{
				Ready: false,
				Details: []common.RedisStatusDetail{
					{InfraStatusDetail: common.NewRedisStatusDetail(common.RedisStandaloneCreatedCode, "Standalone created")},
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})

		It("should translate RedisSentinelDeleted to Deleting state", func() {
			modelStatus := common.RedisStatus{
				Ready: false,
				Details: []common.RedisStatusDetail{
					{InfraStatusDetail: common.NewRedisStatusDetail(common.RedisSentinelDeletedCode, "Sentinel deleted")},
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateDeleting))
			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})

		It("should translate RedisReplicationDeleted to Deleting state", func() {
			modelStatus := common.RedisStatus{
				Ready: false,
				Details: []common.RedisStatusDetail{
					{InfraStatusDetail: common.NewRedisStatusDetail(common.RedisReplicationDeletedCode, "Replication deleted")},
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateDeleting))
			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})

		It("should translate RedisStandaloneDeleted to Deleting state", func() {
			modelStatus := common.RedisStatus{
				Ready: false,
				Details: []common.RedisStatusDetail{
					{InfraStatusDetail: common.NewRedisStatusDetail(common.RedisStandaloneDeletedCode, "Standalone deleted")},
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateDeleting))
			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})

		It("should translate RedisSentinelConnection to Ready state", func() {
			modelStatus := common.RedisStatus{
				Ready: true,
				Details: []common.RedisStatusDetail{
					{InfraStatusDetail: common.NewRedisStatusDetail(common.RedisSentinelConnectionCode, "sentinel connection established")},
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateReady))
			Expect(result.State).To(Equal(apiv2.WBStateReady))
		})

		It("should translate RedisStandaloneConnection to Ready state", func() {
			modelStatus := common.RedisStatus{
				Ready: true,
				Details: []common.RedisStatusDetail{
					{InfraStatusDetail: common.NewRedisStatusDetail(common.RedisStandaloneConnectionCode, "standalone connection established")},
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Details[0].State).To(Equal(apiv2.WBStateReady))
			Expect(result.State).To(Equal(apiv2.WBStateReady))
		})
	})

	Context("when common status has both errors and details", func() {
		It("should use worst state according to WorseThan", func() {
			modelStatus := common.RedisStatus{
				Ready: false,
				Errors: []common.RedisInfraError{
					{InfraError: common.NewRedisError(common.RedisDeploymentConflictCode, "deployment conflict")},
				},
				Details: []common.RedisStatusDetail{
					{InfraStatusDetail: common.NewRedisStatusDetail(common.RedisSentinelCreatedCode, "Sentinel created")},
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.Details).To(HaveLen(2))
			Expect(result.State).To(Equal(apiv2.WBStateUpdating))
		})
	})

	Context("when common status has multiple details with different states", func() {
		It("should compute worst state correctly", func() {
			modelStatus := common.RedisStatus{
				Ready: false,
				Details: []common.RedisStatusDetail{
					{InfraStatusDetail: common.NewRedisStatusDetail(common.RedisReplicationCreatedCode, "replication creating")},
					{InfraStatusDetail: common.NewRedisStatusDetail(common.RedisSentinelDeletedCode, "sentinel deleting")},
				},
			}

			result := TranslateRedisStatus(ctx, modelStatus)

			Expect(result.State).To(Equal(apiv2.WBStateDeleting))
		})
	})
})
