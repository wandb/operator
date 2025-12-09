package utils

func ContainsString(strings []string, target string) bool {
	for _, s := range strings {
		if s == target {
			return true
		}
	}
	return false
}

func RemoveString(strings []string, target string) []string {
	result := []string{}
	for _, s := range strings {
		if s != target {
			result = append(result, s)
		}
	}
	return result
}
