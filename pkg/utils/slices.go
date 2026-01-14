package utils

func FilterFunc[T any](s []T, predicate func(T) bool) []T {
	var result []T
	for _, v := range s {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

func ContainsFunc[T any](s []T, predicate func(T) bool) bool {
	for _, v := range s {
		if predicate(v) {
			return true
		}
	}
	return false
}

func MapFunc[T any, U any](s []T, f func(T) U) []U {
	var result []U
	for _, v := range s {
		result = append(result, f(v))
	}
	return result
}

func FilterMapFunc[T any, U any](s []T, f func(T) (U, bool)) []U {
	var result []U
	for _, v := range s {
		next, ok := f(v)
		if ok {
			result = append(result, next)
		}

	}
	return result
}
