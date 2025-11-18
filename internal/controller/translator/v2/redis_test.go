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
	defaultSmallRedisCpuRequest       = resource.MustParse(model.SmallReplicaCpuRequest)
	defaultSmallRedisCpuLimit         = resource.MustParse(model.SmallReplicaCpuLimit)
	defaultSmallRedisMemoryRequest    = resource.MustParse(model.SmallReplicaMemoryRequest)
	defaultSmallRedisMemoryLimit      = resource.MustParse(model.SmallReplicaMemoryLimit)
	defaultSmallSentinelCpuRequest    = resource.MustParse(model.SmallSentinelCpuRequest)
	defaultSmallSentinelCpuLimit      = resource.MustParse(model.SmallSentinelCpuLimit)
	defaultSmallSentinelMemoryRequest = resource.MustParse(model.SmallSentinelMemoryRequest)
	defaultSmallSentinelMemoryLimit   = resource.MustParse(model.SmallSentinelMemoryLimit)

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

var _ = Describe("BuildRedisSpec", func() {
	Describe("Sentinel merging", func() {
		Context("when both Sentinel values are nil", func() {
			It("should result in nil Sentinel", func() {
				actual := apiv2.WBRedisSpec{
					Sentinel: nil,
				}
				defaults := apiv2.WBRedisSpec{
					Sentinel: nil,
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).To(BeNil())
			})
		})

		Context("when actual Sentinel is nil", func() {
			It("should use default Sentinel", func() {
				actual := apiv2.WBRedisSpec{
					Sentinel: nil,
				}
				defaults := apiv2.WBRedisSpec{
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &apiv2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: defaultSmallSentinelCpuRequest,
								},
							},
						},
					},
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).ToNot(BeNil())
				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.Config).ToNot(BeNil())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallSentinelCpuRequest))
			})
		})

		Context("when default Sentinel is nil", func() {
			It("should use actual Sentinel", func() {
				actual := apiv2.WBRedisSpec{
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: overrideSentinelEnabled,
						Config: &apiv2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: overrideSentinelCpuRequest,
								},
							},
						},
					},
				}
				defaults := apiv2.WBRedisSpec{
					Sentinel: nil,
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).ToNot(BeNil())
				Expect(result.Sentinel.Enabled).To(Equal(overrideSentinelEnabled))
				Expect(result.Sentinel.Config).ToNot(BeNil())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
			})
		})

		Context("when both Sentinel values exist with Config", func() {
			It("should merge Sentinel resources with actual taking precedence", func() {
				actual := apiv2.WBRedisSpec{
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &apiv2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: overrideSentinelCpuRequest,
								},
							},
						},
					},
				}
				defaults := apiv2.WBRedisSpec{
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: false,
						Config: &apiv2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    defaultSmallSentinelCpuRequest,
									corev1.ResourceMemory: defaultSmallSentinelMemoryRequest,
								},
							},
						},
					},
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).ToNot(BeNil())
				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.Config).ToNot(BeNil())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallSentinelMemoryRequest))
			})
		})

		Context("when both Sentinel values exist but actual Config is nil", func() {
			It("should use default Sentinel Config", func() {
				actual := apiv2.WBRedisSpec{
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: true,
						Config:  nil,
					},
				}
				defaults := apiv2.WBRedisSpec{
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: false,
						Config: &apiv2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: defaultSmallSentinelCpuRequest,
								},
							},
						},
					},
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).ToNot(BeNil())
				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.Config).ToNot(BeNil())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallSentinelCpuRequest))
			})
		})

		Context("when both Sentinel values exist but default Config is nil", func() {
			It("should use actual Sentinel Config", func() {
				actual := apiv2.WBRedisSpec{
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &apiv2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: overrideSentinelCpuRequest,
								},
							},
						},
					},
				}
				defaults := apiv2.WBRedisSpec{
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: false,
						Config:  nil,
					},
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).ToNot(BeNil())
				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.Config).ToNot(BeNil())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
			})
		})
	})

	Describe("Config merging", func() {
		Context("when both Config values are nil", func() {
			It("should result in nil Config", func() {
				actual := apiv2.WBRedisSpec{
					Config: nil,
				}
				defaults := apiv2.WBRedisSpec{
					Config: nil,
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).To(BeNil())
			})
		})

		Context("when actual Config is nil", func() {
			It("should use default Config", func() {
				actual := apiv2.WBRedisSpec{
					Config: nil,
				}
				defaults := apiv2.WBRedisSpec{
					Config: &apiv2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallRedisCpuRequest,
								corev1.ResourceMemory: defaultSmallRedisMemoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallRedisCpuLimit,
								corev1.ResourceMemory: defaultSmallRedisMemoryLimit,
							},
						},
					},
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallRedisCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallRedisMemoryRequest))
				Expect(result.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(defaultSmallRedisCpuLimit))
				Expect(result.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(defaultSmallRedisMemoryLimit))
			})
		})

		Context("when default Config is nil", func() {
			It("should use actual Config", func() {
				actual := apiv2.WBRedisSpec{
					Config: &apiv2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: overrideRedisCpuRequest,
							},
						},
					},
				}
				defaults := apiv2.WBRedisSpec{
					Config: nil,
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideRedisCpuRequest))
			})
		})

		Context("when both Config values exist", func() {
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
				defaults := apiv2.WBRedisSpec{
					Config: &apiv2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallRedisCpuRequest,
								corev1.ResourceMemory: defaultSmallRedisMemoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallRedisCpuLimit,
								corev1.ResourceMemory: defaultSmallRedisMemoryLimit,
							},
						},
					},
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideRedisCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallRedisMemoryRequest))
				Expect(result.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(defaultSmallRedisCpuLimit))
				Expect(result.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryLimit))
			})
		})
	})

	Describe("StorageSize merging", func() {
		Context("when actual StorageSize is empty", func() {
			It("should use default StorageSize", func() {
				actual := apiv2.WBRedisSpec{
					StorageSize: "",
				}
				defaults := apiv2.WBRedisSpec{
					StorageSize: overrideRedisStorageSize,
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(overrideRedisStorageSize))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actual := apiv2.WBRedisSpec{
					StorageSize: overrideRedisStorageSize,
				}
				defaults := apiv2.WBRedisSpec{
					StorageSize: model.SmallStorageRequest,
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(overrideRedisStorageSize))
			})
		})

		Context("when both StorageSize values are empty", func() {
			It("should result in empty StorageSize", func() {
				actual := apiv2.WBRedisSpec{
					StorageSize: "",
				}
				defaults := apiv2.WBRedisSpec{
					StorageSize: "",
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(""))
			})
		})
	})

	Describe("Enabled field", func() {
		Context("when actual Enabled is true", func() {
			It("should always use actual Enabled regardless of default", func() {
				actual := apiv2.WBRedisSpec{
					Enabled: true,
				}
				defaults := apiv2.WBRedisSpec{
					Enabled: false,
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
			})
		})

		Context("when actual Enabled is false", func() {
			It("should always use actual Enabled regardless of default", func() {
				actual := apiv2.WBRedisSpec{
					Enabled: false,
				}
				defaults := apiv2.WBRedisSpec{
					Enabled: true,
				}

				result, err := BuildRedisSpec(actual, defaults)
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
				defaults := apiv2.WBRedisSpec{
					Namespace: "default-namespace",
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideRedisNamespace))
			})
		})

		Context("when actual Namespace is empty", func() {
			It("should use default Namespace", func() {
				actual := apiv2.WBRedisSpec{
					Namespace: "",
				}
				defaults := apiv2.WBRedisSpec{
					Namespace: overrideRedisNamespace,
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(overrideRedisNamespace))
			})
		})
	})

	Describe("Complete spec merging", func() {
		Context("when actual is completely empty", func() {
			It("should return all default values except Enabled", func() {
				actual := apiv2.WBRedisSpec{}
				defaults := apiv2.WBRedisSpec{
					Enabled:     true,
					Namespace:   overrideRedisNamespace,
					StorageSize: overrideRedisStorageSize,
					Config: &apiv2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: defaultSmallRedisCpuRequest,
							},
						},
					},
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &apiv2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: defaultSmallSentinelCpuRequest,
								},
							},
						},
					},
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal(overrideRedisNamespace))
				Expect(result.StorageSize).To(Equal(overrideRedisStorageSize))
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallRedisCpuRequest))
				Expect(result.Sentinel).ToNot(BeNil())
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
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: overrideSentinelCpuRequest,
								},
							},
						},
					},
				}
				defaults := apiv2.WBRedisSpec{
					Enabled:     true,
					Namespace:   "default-namespace",
					StorageSize: model.SmallStorageRequest,
					Config: &apiv2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: defaultSmallRedisCpuRequest,
							},
						},
					},
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &apiv2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: defaultSmallSentinelCpuRequest,
								},
							},
						},
					},
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(Equal(overrideRedisEnabled))
				Expect(result.Namespace).To(Equal(overrideRedisNamespace))
				Expect(result.StorageSize).To(Equal(overrideRedisStorageSize))
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideRedisCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryRequest))
				Expect(result.Sentinel.Enabled).To(Equal(overrideSentinelEnabled))
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
			})
		})

		Context("when merging complex partial specs", func() {
			It("should correctly merge all nested fields", func() {
				actual := apiv2.WBRedisSpec{
					Enabled:     true,
					Namespace:   overrideRedisNamespace,
					StorageSize: overrideRedisStorageSize,
					Config: &apiv2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU: overrideRedisCpuLimit,
							},
						},
					},
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &apiv2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: overrideSentinelMemoryLimit,
								},
							},
						},
					},
				}
				defaults := apiv2.WBRedisSpec{
					Enabled:     false,
					Namespace:   "default-namespace",
					StorageSize: model.SmallStorageRequest,
					Config: &apiv2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallRedisCpuRequest,
								corev1.ResourceMemory: defaultSmallRedisMemoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    defaultSmallRedisCpuLimit,
								corev1.ResourceMemory: defaultSmallRedisMemoryLimit,
							},
						},
					},
					Sentinel: &apiv2.WBRedisSentinelSpec{
						Enabled: false,
						Config: &apiv2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    defaultSmallSentinelCpuRequest,
									corev1.ResourceMemory: defaultSmallSentinelMemoryRequest,
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    defaultSmallSentinelCpuLimit,
									corev1.ResourceMemory: defaultSmallSentinelMemoryLimit,
								},
							},
						},
					},
				}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.Namespace).To(Equal(overrideRedisNamespace))
				Expect(result.StorageSize).To(Equal(overrideRedisStorageSize))

				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallRedisCpuRequest))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallRedisMemoryRequest))
				Expect(result.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideRedisCpuLimit))
				Expect(result.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(defaultSmallRedisMemoryLimit))

				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultSmallSentinelCpuRequest))
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultSmallSentinelMemoryRequest))
				Expect(result.Sentinel.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(defaultSmallSentinelCpuLimit))
				Expect(result.Sentinel.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideSentinelMemoryLimit))
			})
		})
	})

	Describe("Edge cases", func() {
		Context("when both specs are completely empty", func() {
			It("should return an empty spec without error", func() {
				actual := apiv2.WBRedisSpec{}
				defaults := apiv2.WBRedisSpec{}

				result, err := BuildRedisSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal(""))
				Expect(result.StorageSize).To(Equal(""))
				Expect(result.Config).To(BeNil())
				Expect(result.Sentinel).To(BeNil())
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

var _ = Describe("InfraConfigBuilder.AddRedisSpec", func() {
	const testOwnerNamespace = "test-namespace"

	Context("when adding dev size spec", func() {
		It("should merge actual with dev defaults from model", func() {
			actual := apiv2.WBRedisSpec{
				Enabled: true,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeDev)
			result := builder.AddRedisSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedRedis).ToNot(BeNil())
			Expect(builder.mergedRedis.Enabled).To(BeTrue())
			Expect(builder.mergedRedis.Namespace).To(Equal(testOwnerNamespace))
			Expect(builder.mergedRedis.StorageSize).To(Equal(model.DevStorageRequest))
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
			result := builder.AddRedisSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedRedis).ToNot(BeNil())

			Expect(builder.mergedRedis.Enabled).To(Equal(overrideRedisEnabled))
			Expect(builder.mergedRedis.Enabled).ToNot(Equal(true))

			Expect(builder.mergedRedis.Namespace).To(Equal(overrideRedisNamespace))
			Expect(builder.mergedRedis.Namespace).ToNot(Equal(testOwnerNamespace))

			Expect(builder.mergedRedis.StorageSize).To(Equal(overrideRedisStorageSize))
			Expect(builder.mergedRedis.StorageSize).ToNot(Equal(model.SmallStorageRequest))

			Expect(builder.mergedRedis.Config).ToNot(BeNil())
			Expect(builder.mergedRedis.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideRedisCpuRequest))
			Expect(builder.mergedRedis.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallRedisCpuRequest))

			Expect(builder.mergedRedis.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryRequest))
			Expect(builder.mergedRedis.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallRedisMemoryRequest))

			Expect(builder.mergedRedis.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideRedisCpuLimit))
			Expect(builder.mergedRedis.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallRedisCpuLimit))

			Expect(builder.mergedRedis.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryLimit))
			Expect(builder.mergedRedis.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallRedisMemoryLimit))

			Expect(builder.mergedRedis.Sentinel).ToNot(BeNil())
			Expect(builder.mergedRedis.Sentinel.Enabled).To(Equal(overrideSentinelEnabled))
			Expect(builder.mergedRedis.Sentinel.Enabled).ToNot(Equal(true))

			Expect(builder.mergedRedis.Sentinel.Config).ToNot(BeNil())
			Expect(builder.mergedRedis.Sentinel.Config.MasterName).To(Equal(overrideSentinelMasterName))
			Expect(builder.mergedRedis.Sentinel.Config.MasterName).ToNot(Equal(model.DefaultSentinelGroup))

			Expect(builder.mergedRedis.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
			Expect(builder.mergedRedis.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallSentinelCpuRequest))

			Expect(builder.mergedRedis.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideSentinelMemoryRequest))
			Expect(builder.mergedRedis.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallSentinelMemoryRequest))

			Expect(builder.mergedRedis.Sentinel.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuLimit))
			Expect(builder.mergedRedis.Sentinel.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallSentinelCpuLimit))

			Expect(builder.mergedRedis.Sentinel.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideSentinelMemoryLimit))
			Expect(builder.mergedRedis.Sentinel.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallSentinelMemoryLimit))
		})
	})

	Context("when adding small size spec with storage override only", func() {
		It("should use override storage and verify it differs from default", func() {
			actual := apiv2.WBRedisSpec{
				Enabled:     true,
				StorageSize: overrideRedisStorageSize,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddRedisSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedRedis.StorageSize).To(Equal(overrideRedisStorageSize))
			Expect(builder.mergedRedis.StorageSize).ToNot(Equal(model.SmallStorageRequest))
			Expect(builder.mergedRedis.Config).ToNot(BeNil())
			Expect(builder.mergedRedis.Sentinel).ToNot(BeNil())
		})
	})

	Context("when adding small size spec with namespace override only", func() {
		It("should use override namespace and verify it differs from default", func() {
			actual := apiv2.WBRedisSpec{
				Enabled:   true,
				Namespace: overrideRedisNamespace,
			}

			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSizeSmall)
			result := builder.AddRedisSpec(actual)

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
			result := builder.AddRedisSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedRedis.Config).ToNot(BeNil())

			Expect(builder.mergedRedis.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideRedisCpuRequest))
			Expect(builder.mergedRedis.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallRedisCpuRequest))

			Expect(builder.mergedRedis.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryRequest))
			Expect(builder.mergedRedis.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallRedisMemoryRequest))

			Expect(builder.mergedRedis.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideRedisCpuLimit))
			Expect(builder.mergedRedis.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallRedisCpuLimit))

			Expect(builder.mergedRedis.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryLimit))
			Expect(builder.mergedRedis.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallRedisMemoryLimit))
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
			result := builder.AddRedisSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).To(BeEmpty())
			Expect(builder.mergedRedis.Sentinel).ToNot(BeNil())

			Expect(builder.mergedRedis.Sentinel.Config.MasterName).To(Equal(overrideSentinelMasterName))
			Expect(builder.mergedRedis.Sentinel.Config.MasterName).ToNot(Equal(model.DefaultSentinelGroup))

			Expect(builder.mergedRedis.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
			Expect(builder.mergedRedis.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(defaultSmallSentinelCpuRequest))

			Expect(builder.mergedRedis.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(overrideSentinelMemoryRequest))
			Expect(builder.mergedRedis.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(defaultSmallSentinelMemoryRequest))

			Expect(builder.mergedRedis.Sentinel.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuLimit))
			Expect(builder.mergedRedis.Sentinel.Config.Resources.Limits[corev1.ResourceCPU]).ToNot(Equal(defaultSmallSentinelCpuLimit))

			Expect(builder.mergedRedis.Sentinel.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideSentinelMemoryLimit))
			Expect(builder.mergedRedis.Sentinel.Config.Resources.Limits[corev1.ResourceMemory]).ToNot(Equal(defaultSmallSentinelMemoryLimit))
		})
	})

	Context("when size is invalid", func() {
		It("should append error to builder", func() {
			actual := apiv2.WBRedisSpec{Enabled: true}
			builder := BuildInfraConfig(testOwnerNamespace, apiv2.WBSize("invalid"))
			result := builder.AddRedisSpec(actual)

			Expect(result).To(Equal(builder))
			Expect(builder.errors).ToNot(BeEmpty())
		})
	})
})

var _ = Describe("TranslateRedisConfig", func() {
	Context("when translating a Redis config with sentinel", func() {
		It("should correctly map all fields including sentinel", func() {
			config := model.RedisConfig{
				Enabled:     true,
				Namespace:   overrideRedisNamespace,
				StorageSize: resource.MustParse(overrideRedisStorageSize),
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    overrideRedisCpuRequest,
					corev1.ResourceMemory: overrideRedisMemoryRequest,
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    overrideRedisCpuLimit,
					corev1.ResourceMemory: overrideRedisMemoryLimit,
				},
				Sentinel: model.SentinelConfig{
					Enabled:         true,
					MasterGroupName: overrideSentinelMasterName,
					ReplicaCount:    model.ReplicaSentinelCount,
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    overrideSentinelCpuRequest,
						corev1.ResourceMemory: overrideSentinelMemoryRequest,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    overrideSentinelCpuLimit,
						corev1.ResourceMemory: overrideSentinelMemoryLimit,
					},
				},
			}

			spec := TranslateRedisConfig(config)

			Expect(spec.Enabled).To(Equal(config.Enabled))
			Expect(spec.Namespace).To(Equal(config.Namespace))
			Expect(spec.StorageSize).To(Equal(overrideRedisStorageSize))
			Expect(spec.Config).ToNot(BeNil())
			Expect(spec.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideRedisCpuRequest))
			Expect(spec.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(overrideRedisMemoryLimit))
			Expect(spec.Sentinel).ToNot(BeNil())
			Expect(spec.Sentinel.Enabled).To(BeTrue())
			Expect(spec.Sentinel.Config.MasterName).To(Equal(overrideSentinelMasterName))
			Expect(spec.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(overrideSentinelCpuRequest))
		})
	})

	Context("when translating a Redis config without sentinel", func() {
		It("should not create sentinel config", func() {
			config := model.RedisConfig{
				Enabled:     true,
				Namespace:   overrideRedisNamespace,
				StorageSize: resource.MustParse(model.SmallStorageRequest),
				Requests:    corev1.ResourceList{},
				Limits:      corev1.ResourceList{},
				Sentinel: model.SentinelConfig{
					Enabled: false,
				},
			}

			spec := TranslateRedisConfig(config)

			Expect(spec.Enabled).To(Equal(config.Enabled))
			Expect(spec.Sentinel).To(BeNil())
		})
	})
})
