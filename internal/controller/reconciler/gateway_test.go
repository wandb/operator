package reconciler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func gatewayTestClient(t *testing.T, objects ...ctrlClient.Object) ctrlClient.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	require.NoError(t, gatewayv1.Install(scheme))

	// deleteGateway no-ops unless the Gateway CRD is registered in the cluster;
	// mirror the runtime discovery that populates this set.
	utils.AddServerResource("Gateway.gateway.networking.k8s.io/v1")

	builder := fake.NewClientBuilder().WithScheme(scheme)
	if len(objects) > 0 {
		builder.WithObjects(objects...)
	}
	return builder.Build()
}

func newWandbForGateway() *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
			UID:       "wandb-uid",
		},
	}
}

func TestDeleteGatewayDeletesOwnedGateway(t *testing.T) {
	wandb := newWandbForGateway()
	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-gateway",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps.wandb.com/v2",
				Kind:       "WeightsAndBiases",
				Name:       wandb.Name,
				UID:        wandb.UID,
			}},
		},
	}
	client := gatewayTestClient(t, wandb, gw)

	require.NoError(t, deleteGateway(context.Background(), client, wandb))

	err := client.Get(context.Background(), types.NamespacedName{Name: "wandb-gateway", Namespace: "default"}, &gatewayv1.Gateway{})
	require.True(t, apiErrors.IsNotFound(err), "owned gateway should have been deleted")
}

func TestDeleteGatewayDoesNotDeleteUnownedGateway(t *testing.T) {
	wandb := newWandbForGateway()
	// A user-provided gateway that happens to share our derived name; no owner ref to us.
	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-gateway",
			Namespace: "default",
		},
	}
	client := gatewayTestClient(t, wandb, gw)

	require.NoError(t, deleteGateway(context.Background(), client, wandb))

	err := client.Get(context.Background(), types.NamespacedName{Name: "wandb-gateway", Namespace: "default"}, &gatewayv1.Gateway{})
	require.NoError(t, err, "unowned gateway must not be deleted")
}

func TestBuildListenerTLSCopiesOptionsWithoutCertificateRefs(t *testing.T) {
	wandb := newWandbForGateway()
	tlsConfig := &apiv2.ListenerTLSConfig{
		Options: map[string]apiv2.TLSOptionValue{
			"networking.gke.io/pre-shared-certs": "wandb-cert",
			"example.com/min-tls-version":        "TLS_1_2",
		},
	}

	got := buildListenerTLS(tlsConfig, wandb)

	require.Equal(t, gatewayv1.TLSModeTerminate, *got.Mode)
	require.Empty(t, got.CertificateRefs)
	require.Equal(t, map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue{
		"networking.gke.io/pre-shared-certs": "wandb-cert",
		"example.com/min-tls-version":        "TLS_1_2",
	}, got.Options)
}

func TestBuildListenerTLSKeepsSecretFallbackWithOptions(t *testing.T) {
	wandb := newWandbForGateway()
	wandb.Spec.Networking.TLS = &apiv2.TLSConfig{SecretName: "wandb-tls"}
	tlsConfig := &apiv2.ListenerTLSConfig{
		Options: map[string]apiv2.TLSOptionValue{
			"example.com/min-tls-version": "TLS_1_2",
		},
	}

	got := buildListenerTLS(tlsConfig, wandb)

	require.Equal(t, []gatewayv1.SecretObjectReference{{
		Name: gatewayv1.ObjectName("wandb-tls"),
	}}, got.CertificateRefs)
	require.Equal(t, gatewayv1.AnnotationValue("TLS_1_2"), got.Options["example.com/min-tls-version"])
}
