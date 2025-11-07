package common_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/wandb_v2/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("MergeResources", func() {
	Describe("merging resource limits and requests", func() {
		Context("when both actual and defaults have values", func() {
			It("should merge with actual taking precedence", func() {
				actual := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("2"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				}
				defaults := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				}

				result := common.MergeResources(actual, defaults)

				Expect(result.Limits).To(HaveLen(2))
				Expect(result.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("2")))
				Expect(result.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))

				Expect(result.Requests).To(HaveLen(2))
				Expect(result.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
				Expect(result.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
			})

			It("should override default values with actual values", func() {
				actual := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				}
				defaults := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				}

				result := common.MergeResources(actual, defaults)

				Expect(result.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("4")))
				Expect(result.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("4Gi")))
				Expect(result.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("2")))
				Expect(result.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
			})
		})

		Context("when actual is empty", func() {
			It("should use all default values", func() {
				actual := corev1.ResourceRequirements{}
				defaults := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				}

				result := common.MergeResources(actual, defaults)

				Expect(result.Limits).To(Equal(defaults.Limits))
				Expect(result.Requests).To(Equal(defaults.Requests))
			})
		})

		Context("when defaults is empty", func() {
			It("should use all actual values", func() {
				actual := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				}
				defaults := corev1.ResourceRequirements{}

				result := common.MergeResources(actual, defaults)

				Expect(result.Limits).To(Equal(actual.Limits))
				Expect(result.Requests).To(Equal(actual.Requests))
			})
		})

		Context("when both are empty", func() {
			It("should return empty resources", func() {
				actual := corev1.ResourceRequirements{}
				defaults := corev1.ResourceRequirements{}

				result := common.MergeResources(actual, defaults)

				Expect(result.Limits).To(BeEmpty())
				Expect(result.Requests).To(BeEmpty())
				Expect(result.Claims).To(BeEmpty())
			})
		})

		Context("with various resource types", func() {
			It("should handle storage resources", func() {
				actual := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				}
				defaults := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceStorage:          resource.MustParse("5Gi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				}

				result := common.MergeResources(actual, defaults)

				Expect(result.Limits[corev1.ResourceStorage]).To(Equal(resource.MustParse("10Gi")))
				Expect(result.Limits[corev1.ResourceEphemeralStorage]).To(Equal(resource.MustParse("1Gi")))
			})

			It("should handle extended resources", func() {
				actual := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("2"),
					},
				}
				defaults := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"nvidia.com/gpu":   resource.MustParse("1"),
						corev1.ResourceCPU: resource.MustParse("4"),
					},
				}

				result := common.MergeResources(actual, defaults)

				Expect(result.Limits["nvidia.com/gpu"]).To(Equal(resource.MustParse("2")))
				Expect(result.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("4")))
			})
		})
	})

	Describe("merging resource claims", func() {
		Context("when both actual and defaults have claims", func() {
			It("should merge with actual claims taking precedence by name", func() {
				actual := corev1.ResourceRequirements{
					Claims: []corev1.ResourceClaim{
						{Name: "claim1"},
						{Name: "claim2"},
					},
				}
				defaults := corev1.ResourceRequirements{
					Claims: []corev1.ResourceClaim{
						{Name: "claim1"},
						{Name: "claim3"},
					},
				}

				result := common.MergeResources(actual, defaults)

				Expect(result.Claims).To(HaveLen(3))
				claimNames := make(map[string]bool)
				for _, claim := range result.Claims {
					claimNames[claim.Name] = true
				}
				Expect(claimNames).To(HaveKey("claim1"))
				Expect(claimNames).To(HaveKey("claim2"))
				Expect(claimNames).To(HaveKey("claim3"))
			})

			It("should not duplicate claims with same name", func() {
				actual := corev1.ResourceRequirements{
					Claims: []corev1.ResourceClaim{
						{Name: "shared-claim"},
					},
				}
				defaults := corev1.ResourceRequirements{
					Claims: []corev1.ResourceClaim{
						{Name: "shared-claim"},
						{Name: "default-claim"},
					},
				}

				result := common.MergeResources(actual, defaults)

				Expect(result.Claims).To(HaveLen(2))
				claimNames := make([]string, 0, len(result.Claims))
				for _, claim := range result.Claims {
					claimNames = append(claimNames, claim.Name)
				}
				Expect(claimNames).To(ContainElement("shared-claim"))
				Expect(claimNames).To(ContainElement("default-claim"))
			})
		})

		Context("when only actual has claims", func() {
			It("should return actual claims", func() {
				actual := corev1.ResourceRequirements{
					Claims: []corev1.ResourceClaim{
						{Name: "claim1"},
						{Name: "claim2"},
					},
				}
				defaults := corev1.ResourceRequirements{}

				result := common.MergeResources(actual, defaults)

				Expect(result.Claims).To(HaveLen(2))
				Expect(result.Claims[0].Name).To(Equal("claim1"))
				Expect(result.Claims[1].Name).To(Equal("claim2"))
			})
		})

		Context("when only defaults has claims", func() {
			It("should return default claims", func() {
				actual := corev1.ResourceRequirements{}
				defaults := corev1.ResourceRequirements{
					Claims: []corev1.ResourceClaim{
						{Name: "default1"},
						{Name: "default2"},
					},
				}

				result := common.MergeResources(actual, defaults)

				Expect(result.Claims).To(HaveLen(2))
				claimNames := make([]string, 0, len(result.Claims))
				for _, claim := range result.Claims {
					claimNames = append(claimNames, claim.Name)
				}
				Expect(claimNames).To(ContainElement("default1"))
				Expect(claimNames).To(ContainElement("default2"))
			})
		})

		Context("when neither has claims", func() {
			It("should return empty claims", func() {
				actual := corev1.ResourceRequirements{}
				defaults := corev1.ResourceRequirements{}

				result := common.MergeResources(actual, defaults)

				Expect(result.Claims).To(BeEmpty())
			})
		})
	})

	Describe("integration scenarios", func() {
		Context("when merging complete resource requirements", func() {
			It("should handle limits, requests, and claims together", func() {
				actual := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("2"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
					Claims: []corev1.ResourceClaim{
						{Name: "actual-claim"},
						{Name: "shared-claim"},
					},
				}
				defaults := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Claims: []corev1.ResourceClaim{
						{Name: "default-claim"},
						{Name: "shared-claim"},
					},
				}

				result := common.MergeResources(actual, defaults)

				Expect(result.Limits).To(HaveLen(2))
				Expect(result.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("2")))
				Expect(result.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))

				Expect(result.Requests).To(HaveLen(2))
				Expect(result.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
				Expect(result.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))

				Expect(result.Claims).To(HaveLen(3))
				claimNames := make(map[string]bool)
				for _, claim := range result.Claims {
					claimNames[claim.Name] = true
				}
				Expect(claimNames).To(HaveKey("actual-claim"))
				Expect(claimNames).To(HaveKey("default-claim"))
				Expect(claimNames).To(HaveKey("shared-claim"))
			})
		})

		Context("when actual completely overrides defaults", func() {
			It("should replace all default resource values", func() {
				actual := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("16Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				}
				defaults := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				}

				result := common.MergeResources(actual, defaults)

				Expect(result.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("8")))
				Expect(result.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("16Gi")))
				Expect(result.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("4")))
				Expect(result.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("8Gi")))
			})
		})

		Context("when actual provides partial overrides", func() {
			It("should fill missing values from defaults", func() {
				actual := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("3"),
					},
				}
				defaults := corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				}

				result := common.MergeResources(actual, defaults)

				Expect(result.Limits).To(HaveLen(2))
				Expect(result.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("3")))
				Expect(result.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("4Gi")))

				Expect(result.Requests).To(HaveLen(2))
				Expect(result.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("1")))
				Expect(result.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
			})
		})
	})

	Describe("Coalesce", func() {
		Context("with string values", func() {
			It("should return actual when actual is non-empty", func() {
				result := common.Coalesce("actual", "default")
				Expect(result).To(Equal("actual"))
			})

			It("should return default when actual is empty", func() {
				result := common.Coalesce("", "default")
				Expect(result).To(Equal("default"))
			})

			It("should return default when both are empty", func() {
				result := common.Coalesce("", "")
				Expect(result).To(Equal(""))
			})

			It("should handle empty default with non-empty actual", func() {
				result := common.Coalesce("actual", "")
				Expect(result).To(Equal("actual"))
			})
		})

		Context("with integer values", func() {
			It("should return actual when actual is non-zero", func() {
				result := common.Coalesce(42, 10)
				Expect(result).To(Equal(42))
			})

			It("should return default when actual is zero", func() {
				result := common.Coalesce(0, 10)
				Expect(result).To(Equal(10))
			})

			It("should handle negative values", func() {
				result := common.Coalesce(-5, 10)
				Expect(result).To(Equal(-5))
			})

			It("should return zero default when actual is zero", func() {
				result := common.Coalesce(0, 0)
				Expect(result).To(Equal(0))
			})
		})

		Context("with boolean values", func() {
			It("should return actual when actual is true", func() {
				result := common.Coalesce(true, false)
				Expect(result).To(BeTrue())
			})

			It("should return default when actual is false", func() {
				result := common.Coalesce(false, true)
				Expect(result).To(BeTrue())
			})

			It("should handle both false", func() {
				result := common.Coalesce(false, false)
				Expect(result).To(BeFalse())
			})
		})

		Context("with float values", func() {
			It("should return actual when actual is non-zero", func() {
				result := common.Coalesce(3.14, 2.71)
				Expect(result).To(Equal(3.14))
			})

			It("should return default when actual is zero", func() {
				result := common.Coalesce(0.0, 2.71)
				Expect(result).To(Equal(2.71))
			})

			It("should handle negative floats", func() {
				result := common.Coalesce(-1.5, 2.71)
				Expect(result).To(Equal(-1.5))
			})
		})

		Context("with custom comparable types", func() {
			type CustomString string

			It("should work with type aliases", func() {
				result := common.Coalesce(CustomString("actual"), CustomString("default"))
				Expect(result).To(Equal(CustomString("actual")))
			})

			It("should return default for empty type alias", func() {
				result := common.Coalesce(CustomString(""), CustomString("default"))
				Expect(result).To(Equal(CustomString("default")))
			})
		})

		Context("integration scenarios", func() {
			It("should be usable in merge functions", func() {
				actualSize := "10Gi"
				defaultSize := "5Gi"
				result := common.Coalesce(actualSize, defaultSize)
				Expect(result).To(Equal("10Gi"))
			})

			It("should handle missing actual values", func() {
				var actualSize string
				defaultSize := "5Gi"
				result := common.Coalesce(actualSize, defaultSize)
				Expect(result).To(Equal("5Gi"))
			})

			It("should work with resource quantities", func() {
				actualReplicas := int32(5)
				defaultReplicas := int32(3)
				result := common.Coalesce(actualReplicas, defaultReplicas)
				Expect(result).To(Equal(int32(5)))
			})

			It("should provide fallback for unset replica counts", func() {
				actualReplicas := int32(0)
				defaultReplicas := int32(3)
				result := common.Coalesce(actualReplicas, defaultReplicas)
				Expect(result).To(Equal(int32(3)))
			})
		})
	})
})
