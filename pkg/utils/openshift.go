package utils

import (
	"os"
	"strconv"
	"sync/atomic"

	corev1 "k8s.io/api/core/v1"
)

// openshiftMode is set at startup by the operator's main entry point or via
// the OPENSHIFT environment variable. When true, managed-infrastructure
// reconcilers apply OpenShift restricted-v2 SCC-compatible security contexts
// to the pods they create.
var openshiftMode atomic.Bool

func init() {
	if v, err := strconv.ParseBool(os.Getenv("OPENSHIFT")); err == nil && v {
		openshiftMode.Store(true)
	}
}

// SetOpenShiftMode enables or disables OpenShift mode at runtime. Intended
// for use by the operator's main package and tests.
func SetOpenShiftMode(enabled bool) {
	openshiftMode.Store(enabled)
}

// IsOpenShift reports whether the operator is configured to run against an
// OpenShift cluster under the restricted-v2 SCC.
func IsOpenShift() bool {
	return openshiftMode.Load()
}

// OpenShiftPodSecurityContext returns a PodSecurityContext compatible with the
// OpenShift restricted-v2 SCC. UIDs/GIDs are intentionally unset so that the
// SCC's range-based admission controller can populate them.
func OpenShiftPodSecurityContext() *corev1.PodSecurityContext {
	t := true
	return &corev1.PodSecurityContext{
		RunAsNonRoot: &t,
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// OpenShiftContainerSecurityContext returns a container SecurityContext
// compatible with the OpenShift restricted-v2 SCC.
func OpenShiftContainerSecurityContext() *corev1.SecurityContext {
	f := false
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: &f,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}
