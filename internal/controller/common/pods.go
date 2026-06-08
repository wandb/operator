package common

import corev1 "k8s.io/api/core/v1"

// PodReady reports whether a pod is running and passing its readiness checks.
// A pod that is starting up or stuck in CrashLoopBackOff stays Running at the
// phase level but reports its Ready condition as false, so this is a more
// accurate "serving" signal than the pod phase alone.
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
