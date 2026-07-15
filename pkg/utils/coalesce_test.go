package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Comparable", func() {
	Describe("Coalesce", func() {
		Context("with string values", func() {
			It("should return actual when actual is non-empty", func() {
				result := Coalesce("actual", "default")
				Expect(result).To(Equal("actual"))
			})

			It("should return default when actual is empty", func() {
				result := Coalesce("", "default")
				Expect(result).To(Equal("default"))
			})

			It("should return default when both are empty", func() {
				result := Coalesce("", "")
				Expect(result).To(Equal(""))
			})

			It("should handle empty default with non-empty actual", func() {
				result := Coalesce("actual", "")
				Expect(result).To(Equal("actual"))
			})
		})

		Context("with integer values", func() {
			It("should return actual when actual is non-zero", func() {
				result := Coalesce(42, 10)
				Expect(result).To(Equal(42))
			})

			It("should return default when actual is zero", func() {
				result := Coalesce(0, 10)
				Expect(result).To(Equal(10))
			})

			It("should handle negative values", func() {
				result := Coalesce(-5, 10)
				Expect(result).To(Equal(-5))
			})

			It("should return zero default when actual is zero", func() {
				result := Coalesce(0, 0)
				Expect(result).To(Equal(0))
			})
		})

		Context("with boolean values", func() {
			It("should return actual when actual is true", func() {
				result := Coalesce(true, false)
				Expect(result).To(BeTrue())
			})

			It("should return default when actual is false", func() {
				result := Coalesce(false, true)
				Expect(result).To(BeTrue())
			})

			It("should handle both false", func() {
				result := Coalesce(false, false)
				Expect(result).To(BeFalse())
			})
		})

		Context("with float values", func() {
			It("should return actual when actual is non-zero", func() {
				result := Coalesce(3.14, 2.71)
				Expect(result).To(Equal(3.14))
			})

			It("should return default when actual is zero", func() {
				result := Coalesce(0.0, 2.71)
				Expect(result).To(Equal(2.71))
			})

			It("should handle negative floats", func() {
				result := Coalesce(-1.5, 2.71)
				Expect(result).To(Equal(-1.5))
			})
		})

		Context("with custom comparable types", func() {
			type CustomString string

			It("should work with type aliases", func() {
				result := Coalesce(CustomString("actual"), CustomString("default"))
				Expect(result).To(Equal(CustomString("actual")))
			})

			It("should return default for empty type alias", func() {
				result := Coalesce(CustomString(""), CustomString("default"))
				Expect(result).To(Equal(CustomString("default")))
			})
		})

		Context("integration scenarios", func() {
			It("should be usable in merge functions", func() {
				actualSize := "10Gi"
				defaultSize := "5Gi"
				result := Coalesce(actualSize, defaultSize)
				Expect(result).To(Equal("10Gi"))
			})

			It("should handle missing actual values", func() {
				var actualSize string
				defaultSize := "5Gi"
				result := Coalesce(actualSize, defaultSize)
				Expect(result).To(Equal("5Gi"))
			})

			It("should work with resource quantities", func() {
				actualReplicas := int32(5)
				defaultReplicas := int32(3)
				result := Coalesce(actualReplicas, defaultReplicas)
				Expect(result).To(Equal(int32(5)))
			})

			It("should provide fallback for unset replica counts", func() {
				actualReplicas := int32(0)
				defaultReplicas := int32(3)
				result := Coalesce(actualReplicas, defaultReplicas)
				Expect(result).To(Equal(int32(3)))
			})
		})
	})

	Describe("CoalesceQuantity", func() {
		Context("with valid quantity strings", func() {
			It("should return actual when actual is non-empty and valid", func() {
				result := CoalesceQuantity("100Mi", "50Mi")
				Expect(result).To(Equal("100Mi"))
			})

			It("should return default when actual is empty", func() {
				result := CoalesceQuantity("", "50Mi")
				Expect(result).To(Equal("50Mi"))
			})

			It("should return empty string when both are empty", func() {
				result := CoalesceQuantity("", "")
				Expect(result).To(Equal(""))
			})

			It("should handle CPU quantities", func() {
				result := CoalesceQuantity("2", "1")
				Expect(result).To(Equal("2"))
			})

			It("should handle millicpu quantities", func() {
				result := CoalesceQuantity("500m", "250m")
				Expect(result).To(Equal("500m"))
			})

			It("should handle various memory units", func() {
				result := CoalesceQuantity("2Gi", "1Gi")
				Expect(result).To(Equal("2Gi"))
			})
		})

		Context("with zero value", func() {
			It("should return default when actual is '0'", func() {
				result := CoalesceQuantity("0", "100Mi")
				Expect(result).To(Equal("100Mi"))
			})

			It("should return empty string when both are '0'", func() {
				result := CoalesceQuantity("0", "0")
				Expect(result).To(Equal(""))
			})

			It("should return empty string when actual is '0' and default is empty", func() {
				result := CoalesceQuantity("0", "")
				Expect(result).To(Equal(""))
			})

			It("should return empty string when actual is empty and default is '0'", func() {
				result := CoalesceQuantity("", "0")
				Expect(result).To(Equal(""))
			})

			It("should treat '0Mi' as zero and return default", func() {
				result := CoalesceQuantity("0Mi", "50Mi")
				Expect(result).To(Equal("50Mi"))
			})

			It("should treat '0Gi' as zero and return default", func() {
				result := CoalesceQuantity("0Gi", "1Gi")
				Expect(result).To(Equal("1Gi"))
			})
		})

		Context("with invalid quantity strings", func() {
			It("should return default when actual is invalid", func() {
				result := CoalesceQuantity("invalid", "50Mi")
				Expect(result).To(Equal("50Mi"))
			})

			It("should return empty string when both are invalid", func() {
				result := CoalesceQuantity("invalid", "also-invalid")
				Expect(result).To(Equal(""))
			})

			It("should return actual when actual is valid and default is invalid", func() {
				result := CoalesceQuantity("100Mi", "invalid")
				Expect(result).To(Equal("100Mi"))
			})
		})

		Context("integration scenarios", func() {
			It("should handle storage size specifications", func() {
				result := CoalesceQuantity("10Gi", "5Gi")
				Expect(result).To(Equal("10Gi"))
			})

			It("should provide fallback for missing storage size", func() {
				result := CoalesceQuantity("", "5Gi")
				Expect(result).To(Equal("5Gi"))
			})

			It("should handle mixed unit formats", func() {
				result := CoalesceQuantity("1024Mi", "1Gi")
				Expect(result).To(Equal("1Gi"))
			})

			It("should handle very large quantities", func() {
				result := CoalesceQuantity("1Ti", "500Gi")
				Expect(result).To(Equal("1Ti"))
			})

			It("should handle very small CPU quantities", func() {
				result := CoalesceQuantity("100m", "50m")
				Expect(result).To(Equal("100m"))
			})
		})
	})
})
