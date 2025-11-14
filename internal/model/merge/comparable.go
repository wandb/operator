package merge

func Coalesce[T comparable](actual, defaultValue T) T {
	var zero T
	if actual != zero {
		return actual
	}
	return defaultValue
}
