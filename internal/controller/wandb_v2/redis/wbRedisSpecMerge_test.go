package redis

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("wbRedisSpecsMerge", func() {
	Describe("Sentinel merging", func() {
		Context("when both Sentinel values are nil", func() {
			It("should result in nil Sentinel", func() {
				actual := v2.WBRedisSpec{
					Sentinel: nil,
				}
				defaults := v2.WBRedisSpec{
					Sentinel: nil,
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).To(BeNil())
			})
		})

		Context("when actual Sentinel is nil", func() {
			It("should use default Sentinel", func() {
				actual := v2.WBRedisSpec{
					Sentinel: nil,
				}
				defaults := v2.WBRedisSpec{
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &v2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("100m"),
								},
							},
						},
					},
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).ToNot(BeNil())
				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.Config).ToNot(BeNil())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("100m")))
			})
		})

		Context("when default Sentinel is nil", func() {
			It("should use actual Sentinel", func() {
				actual := v2.WBRedisSpec{
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: false,
						Config: &v2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("200m"),
								},
							},
						},
					},
				}
				defaults := v2.WBRedisSpec{
					Sentinel: nil,
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).ToNot(BeNil())
				Expect(result.Sentinel.Enabled).To(BeFalse())
				Expect(result.Sentinel.Config).ToNot(BeNil())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("200m")))
			})
		})

		Context("when both Sentinel values exist with Config", func() {
			It("should merge Sentinel resources with actual taking precedence", func() {
				actual := v2.WBRedisSpec{
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &v2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("300m"),
								},
							},
						},
					},
				}
				defaults := v2.WBRedisSpec{
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: false,
						Config: &v2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).ToNot(BeNil())
				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.Config).ToNot(BeNil())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("300m")))
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("128Mi")))
			})
		})

		Context("when both Sentinel values exist but actual Config is nil", func() {
			It("should use default Sentinel Config", func() {
				actual := v2.WBRedisSpec{
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: true,
						Config:  nil,
					},
				}
				defaults := v2.WBRedisSpec{
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: false,
						Config: &v2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("100m"),
								},
							},
						},
					},
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).ToNot(BeNil())
				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.Config).ToNot(BeNil())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("100m")))
			})
		})

		Context("when both Sentinel values exist but default Config is nil", func() {
			It("should use actual Sentinel Config", func() {
				actual := v2.WBRedisSpec{
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &v2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("200m"),
								},
							},
						},
					},
				}
				defaults := v2.WBRedisSpec{
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: false,
						Config:  nil,
					},
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Sentinel).ToNot(BeNil())
				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.Config).ToNot(BeNil())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("200m")))
			})
		})
	})

	Describe("Config merging", func() {
		Context("when both Config values are nil", func() {
			It("should result in nil Config", func() {
				actual := v2.WBRedisSpec{
					Config: nil,
				}
				defaults := v2.WBRedisSpec{
					Config: nil,
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).To(BeNil())
			})
		})

		Context("when actual Config is nil", func() {
			It("should use default Config", func() {
				actual := v2.WBRedisSpec{
					Config: nil,
				}
				defaults := v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:     resource.MustParse("500m"),
								corev1.ResourceMemory:  resource.MustParse("512Mi"),
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("512Mi")))
				Expect(result.Config.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("1Gi")))
				Expect(result.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("1000m")))
				Expect(result.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))
			})
		})

		Context("when default Config is nil", func() {
			It("should use actual Config", func() {
				actual := v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("250m"),
							},
						},
					},
				}
				defaults := v2.WBRedisSpec{
					Config: nil,
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("250m")))
			})
		})

		Context("when both Config values exist", func() {
			It("should merge resources with actual taking precedence", func() {
				actual := v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("750m"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
					},
				}
				defaults := v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:     resource.MustParse("500m"),
								corev1.ResourceMemory:  resource.MustParse("512Mi"),
								corev1.ResourceStorage: resource.MustParse("5Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("750m")))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("512Mi")))
				Expect(result.Config.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("5Gi")))
				Expect(result.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("1000m")))
				Expect(result.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
			})
		})
	})

	Describe("StorageSize merging", func() {
		Context("when actual StorageSize is empty", func() {
			It("should use default StorageSize", func() {
				actual := v2.WBRedisSpec{
					StorageSize: "",
				}
				defaults := v2.WBRedisSpec{
					StorageSize: "10Gi",
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal("10Gi"))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actual := v2.WBRedisSpec{
					StorageSize: "20Gi",
				}
				defaults := v2.WBRedisSpec{
					StorageSize: "10Gi",
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal("20Gi"))
			})
		})

		Context("when both StorageSize values are empty", func() {
			It("should result in empty StorageSize", func() {
				actual := v2.WBRedisSpec{
					StorageSize: "",
				}
				defaults := v2.WBRedisSpec{
					StorageSize: "",
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal(""))
			})
		})
	})

	Describe("Enabled field", func() {
		Context("when actual Enabled is true", func() {
			It("should always use actual Enabled regardless of default", func() {
				actual := v2.WBRedisSpec{
					Enabled: true,
				}
				defaults := v2.WBRedisSpec{
					Enabled: false,
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
			})
		})

		Context("when actual Enabled is false", func() {
			It("should always use actual Enabled regardless of default", func() {
				actual := v2.WBRedisSpec{
					Enabled: false,
				}
				defaults := v2.WBRedisSpec{
					Enabled: true,
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
			})
		})
	})

	Describe("Namespace field", func() {
		Context("when actual Namespace is set", func() {
			It("should always use actual Namespace regardless of default", func() {
				actual := v2.WBRedisSpec{
					Namespace: "custom-namespace",
				}
				defaults := v2.WBRedisSpec{
					Namespace: "default-namespace",
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal("custom-namespace"))
			})
		})

		Context("when actual Namespace is empty", func() {
			It("should always use actual Namespace regardless of default", func() {
				actual := v2.WBRedisSpec{
					Namespace: "",
				}
				defaults := v2.WBRedisSpec{
					Namespace: "default-namespace",
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal(""))
			})
		})
	})

	Describe("Complete spec merging", func() {
		Context("when actual is completely empty", func() {
			It("should return all default values except Enabled and Namespace", func() {
				actual := v2.WBRedisSpec{}
				defaults := v2.WBRedisSpec{
					Enabled:     true,
					Namespace:   "default",
					StorageSize: "10Gi",
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("500m"),
							},
						},
					},
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &v2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("100m"),
								},
							},
						},
					},
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal(""))
				Expect(result.StorageSize).To(Equal("10Gi"))
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
				Expect(result.Sentinel).ToNot(BeNil())
				Expect(result.Sentinel.Enabled).To(BeTrue())
			})
		})

		Context("when actual has all values set", func() {
			It("should use actual values for all fields", func() {
				actual := v2.WBRedisSpec{
					Enabled:     false,
					Namespace:   "actual-namespace",
					StorageSize: "25Gi",
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
					},
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: false,
						Config: &v2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("200m"),
								},
							},
						},
					},
				}
				defaults := v2.WBRedisSpec{
					Enabled:     true,
					Namespace:   "default-namespace",
					StorageSize: "10Gi",
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("500m"),
							},
						},
					},
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &v2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("100m"),
								},
							},
						},
					},
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal("actual-namespace"))
				Expect(result.StorageSize).To(Equal("25Gi"))
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("1")))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
				Expect(result.Sentinel.Enabled).To(BeFalse())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("200m")))
			})
		})

		Context("when merging complex partial specs", func() {
			It("should correctly merge all nested fields", func() {
				actual := v2.WBRedisSpec{
					Enabled:     true,
					Namespace:   "prod",
					StorageSize: "50Gi",
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("2"),
							},
						},
					},
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: true,
						Config: &v2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				}
				defaults := v2.WBRedisSpec{
					Enabled:     false,
					Namespace:   "dev",
					StorageSize: "10Gi",
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:     resource.MustParse("500m"),
								corev1.ResourceMemory:  resource.MustParse("1Gi"),
								corev1.ResourceStorage: resource.MustParse("5Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
					},
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: false,
						Config: &v2.WBRedisSentinelConfig{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				}

				result, err := wbRedisSpecsMerge(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.Namespace).To(Equal("prod"))
				Expect(result.StorageSize).To(Equal("50Gi"))

				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))
				Expect(result.Config.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("5Gi")))
				Expect(result.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("2")))
				Expect(result.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))

				Expect(result.Sentinel.Enabled).To(BeTrue())
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("100m")))
				Expect(result.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("128Mi")))
				Expect(result.Sentinel.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("200m")))
				Expect(result.Sentinel.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("256Mi")))
			})
		})
	})

	Describe("Edge cases", func() {
		Context("when both specs are completely empty", func() {
			It("should return an empty spec without error", func() {
				actual := v2.WBRedisSpec{}
				defaults := v2.WBRedisSpec{}

				result, err := wbRedisSpecsMerge(actual, defaults)
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
