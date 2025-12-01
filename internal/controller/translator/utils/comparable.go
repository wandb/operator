package utils

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

func Coalesce[T comparable](actual, defaultValue T) T {
	var zero T
	if actual != zero {
		return actual
	}
	return defaultValue
}

func CoalesceQuantity(actual, defaultValue string) string {
	if actual != "" {
		if qty, err := resource.ParseQuantity(actual); err == nil && !qty.IsZero() {
			return qty.String()
		}
	}

	if defaultValue != "" {
		if qty, err := resource.ParseQuantity(defaultValue); err == nil && !qty.IsZero() {
			return qty.String()
		}
	}

	return ""
}
