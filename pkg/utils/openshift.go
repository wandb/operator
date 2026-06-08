package utils

import (
	"os"
	"strconv"
	"sync/atomic"
)

// OpenShiftNginxOverlayLabel marks a workload that needs its read-only nginx
// asset and runtime directories overlaid with writable emptyDirs to run under
// OpenShift's restricted-v2 SCC (arbitrary UID). The label value is the name of
// the container the overlay applies to.
const OpenShiftNginxOverlayLabel = "operator.wandb.ai/openshift-nginx-overlay"

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
