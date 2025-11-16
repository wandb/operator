package v2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("BuildMySQLSpec", func() {
	Describe("Config merging", func() {
		Context("when both Config values are nil", func() {
			It("should result in nil Config", func() {
				actual := v2.WBMySQLSpec{
					Config: nil,
				}
				defaults := v2.WBMySQLSpec{
					Config: nil,
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).To(BeNil())
			})
		})

		Context("when actual Config is nil", func() {
			It("should use default Config", func() {
				actual := v2.WBMySQLSpec{
					Config: nil,
				}
				defaults := v2.WBMySQLSpec{
					Config: &v2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
					},
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))
				Expect(result.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("1000m")))
				Expect(result.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
			})
		})

		Context("when default Config is nil", func() {
			It("should use actual Config", func() {
				actual := v2.WBMySQLSpec{
					Config: &v2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("250m"),
							},
						},
					},
				}
				defaults := v2.WBMySQLSpec{
					Config: nil,
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("250m")))
			})
		})

		Context("when both Config values exist", func() {
			It("should merge resources with actual taking precedence", func() {
				actual := v2.WBMySQLSpec{
					Config: &v2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("750m"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("3Gi"),
							},
						},
					},
				}
				defaults := v2.WBMySQLSpec{
					Config: &v2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
					},
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("750m")))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))
				Expect(result.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("1000m")))
				Expect(result.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("3Gi")))
			})
		})
	})

	Describe("StorageSize merging", func() {
		Context("when actual StorageSize is empty", func() {
			It("should use default StorageSize", func() {
				actual := v2.WBMySQLSpec{
					StorageSize: "",
				}
				defaults := v2.WBMySQLSpec{
					StorageSize: "10Gi",
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal("10Gi"))
			})
		})

		Context("when actual StorageSize is set", func() {
			It("should use actual StorageSize", func() {
				actual := v2.WBMySQLSpec{
					StorageSize: "20Gi",
				}
				defaults := v2.WBMySQLSpec{
					StorageSize: "10Gi",
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.StorageSize).To(Equal("20Gi"))
			})
		})

		Context("when both StorageSize values are empty", func() {
			It("should result in empty StorageSize", func() {
				actual := v2.WBMySQLSpec{
					StorageSize: "",
				}
				defaults := v2.WBMySQLSpec{
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
				actual := v2.WBMySQLSpec{
					Namespace: "",
				}
				defaults := v2.WBMySQLSpec{
					Namespace: "default-namespace",
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal("default-namespace"))
			})
		})

		Context("when actual Namespace is set", func() {
			It("should use actual Namespace", func() {
				actual := v2.WBMySQLSpec{
					Namespace: "custom-namespace",
				}
				defaults := v2.WBMySQLSpec{
					Namespace: "default-namespace",
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Namespace).To(Equal("custom-namespace"))
			})
		})

		Context("when both Namespace values are empty", func() {
			It("should result in empty Namespace", func() {
				actual := v2.WBMySQLSpec{
					Namespace: "",
				}
				defaults := v2.WBMySQLSpec{
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
				actual := v2.WBMySQLSpec{
					Enabled: true,
				}
				defaults := v2.WBMySQLSpec{
					Enabled: false,
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
			})
		})

		Context("when actual Enabled is false", func() {
			It("should always use actual Enabled regardless of default", func() {
				actual := v2.WBMySQLSpec{
					Enabled: false,
				}
				defaults := v2.WBMySQLSpec{
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
				actual := v2.WBMySQLSpec{}
				defaults := v2.WBMySQLSpec{
					Enabled:     true,
					Namespace:   "default",
					StorageSize: "10Gi",
					Config: &v2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("500m"),
							},
						},
					},
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal("default"))
				Expect(result.StorageSize).To(Equal("10Gi"))
				Expect(result.Config).ToNot(BeNil())
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
			})
		})

		Context("when actual has all values set", func() {
			It("should use actual values for all fields", func() {
				actual := v2.WBMySQLSpec{
					Enabled:     false,
					Namespace:   "actual-namespace",
					StorageSize: "25Gi",
					Config: &v2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("2"),
								corev1.ResourceMemory: resource.MustParse("4Gi"),
							},
						},
					},
				}
				defaults := v2.WBMySQLSpec{
					Enabled:     true,
					Namespace:   "default-namespace",
					StorageSize: "10Gi",
					Config: &v2.WBMySQLConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("500m"),
							},
						},
					},
				}

				result, err := BuildMySQLSpec(actual, defaults)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.Namespace).To(Equal("actual-namespace"))
				Expect(result.StorageSize).To(Equal("25Gi"))
				Expect(result.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("2")))
				Expect(result.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("4Gi")))
			})
		})
	})

	Describe("Edge cases", func() {
		Context("when both specs are completely empty", func() {
			It("should return an empty spec without error", func() {
				actual := v2.WBMySQLSpec{}
				defaults := v2.WBMySQLSpec{}

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

var _ = Describe("BuildMySQLDefaults", func() {
	Describe("Dev profile", func() {
		Context("when profile is Dev", func() {
			It("should return a MySQL spec with storage only and no resources", func() {
				spec, err := BuildMySQLDefaults(v2.WBSizeDev)
				Expect(err).ToNot(HaveOccurred())
				Expect(spec.Enabled).To(BeTrue())
				Expect(spec.Namespace).To(Equal(defaultNamespace))
				Expect(spec.StorageSize).To(Equal(devMySQLStorageSize))
				Expect(spec.Config).To(BeNil())
			})
		})
	})

	Describe("Small profile", func() {
		Context("when profile is Small", func() {
			It("should return a MySQL spec with full resource requirements", func() {
				spec, err := BuildMySQLDefaults(v2.WBSizeSmall)
				Expect(err).ToNot(HaveOccurred())
				Expect(spec.Enabled).To(BeTrue())
				Expect(spec.Namespace).To(Equal(defaultNamespace))
				Expect(spec.StorageSize).To(Equal(smallMySQLStorageSize))
				Expect(spec.Config).ToNot(BeNil())

				cpuRequest, err := resource.ParseQuantity(smallMySQLCpuRequest)
				Expect(err).ToNot(HaveOccurred())
				cpuLimit, err := resource.ParseQuantity(smallMySQLCpuLimit)
				Expect(err).ToNot(HaveOccurred())
				memoryRequest, err := resource.ParseQuantity(smallMySQLMemoryRequest)
				Expect(err).ToNot(HaveOccurred())
				memoryLimit, err := resource.ParseQuantity(smallMySQLMemoryLimit)
				Expect(err).ToNot(HaveOccurred())

				Expect(spec.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(cpuRequest))
				Expect(spec.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(cpuLimit))
				Expect(spec.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(memoryRequest))
				Expect(spec.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(memoryLimit))
			})
		})
	})

	Describe("Invalid profile", func() {
		Context("when profile is invalid", func() {
			It("should return an error", func() {
				_, err := BuildMySQLDefaults(v2.WBSize("invalid"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported size for MySQL"))
				Expect(err.Error()).To(ContainSubstring("only 'dev' and 'small' are supported"))
			})
		})
	})

	Describe("Constants validation", func() {
		It("should have valid resource quantity constants", func() {
			quantities := map[string]string{
				"devMySQLStorageSize":     devMySQLStorageSize,
				"smallMySQLStorageSize":   smallMySQLStorageSize,
				"smallMySQLCpuRequest":    smallMySQLCpuRequest,
				"smallMySQLCpuLimit":      smallMySQLCpuLimit,
				"smallMySQLMemoryRequest": smallMySQLMemoryRequest,
				"smallMySQLMemoryLimit":   smallMySQLMemoryLimit,
			}

			for name, value := range quantities {
				_, err := resource.ParseQuantity(value)
				Expect(err).ToNot(HaveOccurred(), "Failed to parse %s: %s", name, value)
			}
		})
	})
})
