package common

// HasAllLabelKeys reports whether existing contains every key present in desired,
// regardless of value.
func HasAllLabelKeys(existing, desired map[string]string) bool {
	for k := range desired {
		if _, ok := existing[k]; !ok {
			return false
		}
	}
	return true
}
