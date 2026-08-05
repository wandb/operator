package common

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NormalizeServicePorts fills the port fields the API server defaults (protocol,
// targetPort) so specs built from manifests round-trip equal to what is stored.
func NormalizeServicePorts(ports []corev1.ServicePort) {
	for i := range ports {
		if ports[i].Protocol == "" {
			ports[i].Protocol = corev1.ProtocolTCP
		}
		if ports[i].TargetPort == (intstr.IntOrString{}) {
			ports[i].TargetPort = intstr.FromInt32(ports[i].Port)
		}
	}
}
