package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/wandb/operator/internal/controller/common"
)

// defaultedClusterIPSpec is what the API server stores for a Service created
// from a template that only names a port.
func defaultedClusterIPSpec() *corev1.ServiceSpec {
	itp := corev1.ServiceInternalTrafficPolicyCluster
	return &corev1.ServiceSpec{
		Type:                  corev1.ServiceTypeClusterIP,
		ClusterIP:             "10.0.0.1",
		SessionAffinity:       corev1.ServiceAffinityNone,
		InternalTrafficPolicy: &itp,
		Ports: []corev1.ServicePort{
			{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(80)},
		},
	}
}

func TestPreserveServerDefaultedServiceFields_SteadyStateSettles(t *testing.T) {
	current := defaultedClusterIPSpec()
	desired := &corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80}}}

	common.NormalizeServicePorts(desired.Ports)
	preserveServerDefaultedServiceFields(desired, current)

	require.Equal(t, current.Type, desired.Type)
	require.Equal(t, current.SessionAffinity, desired.SessionAffinity)
	require.Equal(t, current.InternalTrafficPolicy, desired.InternalTrafficPolicy)
	require.True(t, apiequality.Semantic.DeepEqual(current.Ports, desired.Ports),
		"a template-derived spec must compare equal to the server-defaulted one at steady state")
}

func TestPreserveServerDefaultedServiceFields_RealChangesStillDiffer(t *testing.T) {
	current := defaultedClusterIPSpec()
	desired := &corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 9090}}}

	common.NormalizeServicePorts(desired.Ports)
	preserveServerDefaultedServiceFields(desired, current)

	require.False(t, apiequality.Semantic.DeepEqual(current.Ports, desired.Ports),
		"a genuine port change must still be detected")
}

func TestPreserveServerDefaultedServiceFields_NodePorts(t *testing.T) {
	current := &corev1.ServiceSpec{
		Type: corev1.ServiceTypeNodePort,
		Ports: []corev1.ServicePort{
			{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(80), NodePort: 30080},
		},
	}

	desired := &corev1.ServiceSpec{
		Type:  corev1.ServiceTypeNodePort,
		Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
	}
	common.NormalizeServicePorts(desired.Ports)
	preserveServerDefaultedServiceFields(desired, current)
	require.Equal(t, int32(30080), desired.Ports[0].NodePort, "allocated NodePort is preserved")

	pinned := &corev1.ServiceSpec{
		Type:  corev1.ServiceTypeNodePort,
		Ports: []corev1.ServicePort{{Name: "http", Port: 80, NodePort: 31000}},
	}
	common.NormalizeServicePorts(pinned.Ports)
	preserveServerDefaultedServiceFields(pinned, current)
	require.Equal(t, int32(31000), pinned.Ports[0].NodePort, "template-pinned NodePort wins")
}

func TestPreserveServerDefaultedServiceFields_ClusterIPDropsNodePorts(t *testing.T) {
	current := &corev1.ServiceSpec{
		Type: corev1.ServiceTypeNodePort,
		Ports: []corev1.ServicePort{
			{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(80), NodePort: 30080},
		},
	}
	desired := &corev1.ServiceSpec{
		Type:  corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
	}
	common.NormalizeServicePorts(desired.Ports)
	preserveServerDefaultedServiceFields(desired, current)
	require.Zero(t, desired.Ports[0].NodePort,
		"switching to ClusterIP must not carry the stale NodePort")
}
