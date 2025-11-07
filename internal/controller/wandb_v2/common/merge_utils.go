package common

import corev1 "k8s.io/api/core/v1"

func MergeResources(actual, defaultValues corev1.ResourceRequirements) corev1.ResourceRequirements {
	var results corev1.ResourceRequirements
	results.Limits = mergeMap(actual.Limits, defaultValues.Limits)
	results.Requests = mergeMap(actual.Requests, defaultValues.Requests)
	results.Claims = mergeResourceClaims(actual.Claims, defaultValues.Claims)

	return results
}

func mergeMap[K comparable, V any](actual, defaultValues map[K]V) map[K]V {
	result := make(map[K]V)

	for k, v := range defaultValues {
		result[k] = v
	}

	for k, v := range actual {
		result[k] = v
	}

	return result
}

func mergeResourceClaims(actual, defaultValues []corev1.ResourceClaim) []corev1.ResourceClaim {
	if len(defaultValues) == 0 {
		return actual
	}
	if len(actual) == 0 {
		return defaultValues
	}

	claimsByName := make(map[string]corev1.ResourceClaim)

	for _, claim := range defaultValues {
		claimsByName[claim.Name] = claim
	}

	for _, claim := range actual {
		claimsByName[claim.Name] = claim
	}

	result := make([]corev1.ResourceClaim, 0, len(claimsByName))
	for _, claim := range claimsByName {
		result = append(result, claim)
	}

	return result
}

func Coalesce[T comparable](actual, defaultValue T) T {
	var zero T
	if actual != zero {
		return actual
	}
	return defaultValue
}
