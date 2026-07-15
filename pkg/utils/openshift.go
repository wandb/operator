package utils

import (
	"os"
	"strconv"
	"sync/atomic"
)

var openshiftMode atomic.Bool

func init() {
	if enabled, err := strconv.ParseBool(os.Getenv("OPENSHIFT")); err == nil && enabled {
		openshiftMode.Store(true)
	}
}

// SetOpenShiftMode enables or disables OpenShift rendering behavior.
func SetOpenShiftMode(enabled bool) {
	openshiftMode.Store(enabled)
}

// IsOpenShift reports whether OpenShift-specific rendering behavior is enabled.
func IsOpenShift() bool {
	return openshiftMode.Load()
}
