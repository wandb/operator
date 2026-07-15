package utils

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestIsRegisteredUsesKindGroupVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := gatewayv1.Install(scheme); err != nil {
		t.Fatalf("install gateway api scheme: %v", err)
	}

	t.Run("returns true when server resource key uses version", func(t *testing.T) {
		serverResources = map[string]bool{}
		AddServerResource("HTTPRoute.gateway.networking.k8s.io/v1")

		if !IsRegistered(scheme, &gatewayv1.HTTPRoute{}) {
			t.Fatalf("expected HTTPRoute to be registered")
		}
	})

	t.Run("returns false when server resource key is missing", func(t *testing.T) {
		serverResources = map[string]bool{}

		if IsRegistered(scheme, &gatewayv1.HTTPRoute{}) {
			t.Fatalf("expected HTTPRoute to be unregistered")
		}
	})
}
