// Vendored helper types to avoid dependencies on internal Percona packages
package v1

import corev1 "k8s.io/api/core/v1"

// Platform represents the Kubernetes platform type
type Platform string

const (
	PlatformUndef      Platform = ""
	PlatformKubernetes Platform = "kubernetes"
	PlatformOpenshift  Platform = "openshift"
)

// PMM user constants (from pkg/pxc/users)
const (
	PMMServer      = "pmmserver"
	PMMServerKey   = "pmmserverkey"
	PMMServerToken = "pmmservertoken"
)

// MergeEnvLists merges two environment variable lists (from pkg/util)
// VENDORED: Simplified version to avoid dependency
func MergeEnvLists(envs ...[]corev1.EnvVar) []corev1.EnvVar {
	if len(envs) == 0 {
		return nil
	}
	if len(envs) == 1 {
		return envs[0]
	}

	merged := make(map[string]corev1.EnvVar)
	for _, env := range envs {
		for _, e := range env {
			merged[e.Name] = e
		}
	}

	result := make([]corev1.EnvVar, 0, len(merged))
	for _, v := range merged {
		result = append(result, v)
	}
	return result
}
