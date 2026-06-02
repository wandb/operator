package seaweedfs

import (
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	filerHTTPPort = 8888
	filerGRPCPort = 18888
)

// ToFilerNetworkPolicy generates a NetworkPolicy that restricts ingress to the
// Seaweed Filer pods so only sibling Seaweed components (S3 gateway, Master,
// Volume, and other Filer replicas) can reach them. The Filer's HTTP API on
// port 8888 is unauthenticated by default; this policy keeps it off-limits to
// arbitrary in-cluster traffic.
func ToFilerNetworkPolicy(
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*networkingv1.NetworkPolicy, error) {
	infraSpec := wandb.Spec.ObjectStore.ManagedObjectStore
	if infraSpec == nil {
		return nil, nil
	}

	seaweedName := SeaweedName(infraSpec.Name)
	policyName := fmt.Sprintf("%s-filer-restrict", seaweedName)
	tcp := corev1.ProtocolTCP
	httpPort := intstr.FromInt(filerHTTPPort)
	grpcPort := intstr.FromInt(filerGRPCPort)

	componentPeer := func(component string) networkingv1.NetworkPolicyPeer {
		return networkingv1.NetworkPolicyPeer{
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance":  seaweedName,
					"app.kubernetes.io/component": component,
				},
			},
		}
	}

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: infraSpec.Namespace,
			Labels:    BuildWandbObjectStoreLabels(wandb),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance":  seaweedName,
					"app.kubernetes.io/component": "filer",
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						componentPeer("s3"),
						componentPeer("master"),
						componentPeer("volume"),
						componentPeer("filer"),
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{Protocol: &tcp, Port: &httpPort},
						{Protocol: &tcp, Port: &grpcPort},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(wandb, policy, scheme); err != nil {
		return nil, fmt.Errorf("failed to set owner reference on Filer NetworkPolicy: %w", err)
	}

	return policy, nil
}
