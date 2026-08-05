package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestNormalizeServicePorts(t *testing.T) {
	ports := []corev1.ServicePort{
		{Name: "http", Port: 8080},
		{Name: "grpc", Port: 9000, Protocol: corev1.ProtocolUDP, TargetPort: intstr.FromString("grpc")},
	}
	NormalizeServicePorts(ports)

	require.Equal(t, corev1.ProtocolTCP, ports[0].Protocol)
	require.Equal(t, intstr.FromInt32(8080), ports[0].TargetPort)

	require.Equal(t, corev1.ProtocolUDP, ports[1].Protocol, "explicit protocol is kept")
	require.Equal(t, intstr.FromString("grpc"), ports[1].TargetPort, "explicit targetPort is kept")

	NormalizeServicePorts(nil)
}
