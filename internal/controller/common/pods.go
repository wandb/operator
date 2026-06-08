package common

import corev1 "k8s.io/api/core/v1"

// PodReady reports whether a pod is Running with its Ready condition true, so a
// starting or CrashLoopBackOff pod (Running but not Ready) is not counted.
func PodReady(pod *corev1.Pod) bool {
	if pod == nil || pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
