package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Resources", func() {
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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

				Expect(result.Limits).To(Equal(actual.Limits))
				Expect(result.Requests).To(Equal(actual.Requests))
			})
		})

		Context("when both are empty", func() {
			It("should return empty resources", func() {
				actual := corev1.ResourceRequirements{}
				defaults := corev1.ResourceRequirements{}

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

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

				result := Resources(actual, defaults)

				Expect(result.Limits).To(HaveLen(2))
				Expect(result.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("3")))
				Expect(result.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("4Gi")))

				Expect(result.Requests).To(HaveLen(2))
				Expect(result.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("1")))
				Expect(result.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
			})
		})
	})
})
