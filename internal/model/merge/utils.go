package merge

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
