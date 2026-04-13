package v2

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type noMatchGatewayClient struct {
	ctrlClient.Client
}

func (c *noMatchGatewayClient) Get(ctx context.Context, key ctrlClient.ObjectKey, obj ctrlClient.Object, opts ...ctrlClient.GetOption) error {
	switch obj.(type) {
	case *gatewayv1.Gateway:
		return missingGatewayAPIKind("Gateway")
	default:
		return c.Client.Get(ctx, key, obj, opts...)
	}
}

func (c *noMatchGatewayClient) List(ctx context.Context, list ctrlClient.ObjectList, opts ...ctrlClient.ListOption) error {
	switch list.(type) {
	case *gatewayv1.HTTPRouteList:
		return missingGatewayAPIKind("HTTPRoute")
	default:
		return c.Client.List(ctx, list, opts...)
	}
}

func missingGatewayAPIKind(kind string) error {
	return &apimeta.NoKindMatchError{
		GroupKind:        schema.GroupKind{Group: gatewayv1.GroupVersion.Group, Kind: kind},
		SearchedVersions: []string{gatewayv1.GroupVersion.Version},
	}
}

func TestCleanupNetworkingModeResourcesIngressSkipsMissingGatewayAPICRDs(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
		},
		Spec: apiv2.WeightsAndBiasesSpec{
			Networking: apiv2.NetworkingSpec{
				Mode: apiv2.NetworkingModeIngress,
			},
		},
	}

	client := &noMatchGatewayClient{
		Client: fake.NewClientBuilder().Build(),
	}

	if err := cleanupNetworkingModeResources(context.Background(), client, wandb); err != nil {
		t.Fatalf("expected ingress cleanup to ignore missing gateway api CRDs, got %v", err)
	}
}
