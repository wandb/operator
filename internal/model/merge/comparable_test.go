package merge

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
})
