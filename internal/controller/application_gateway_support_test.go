package controller

import (
	"context"
	"testing"

	wandbv2 "github.com/wandb/operator/api/v2"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestDeleteHTTPRouteSkipsWhenGatewayAPIIsUnavailable(t *testing.T) {
	mapper := apimeta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "apps", Version: "v1"},
	})

	reconciler := &ApplicationReconciler{
		RESTMapper: mapper,
	}

	app := &wandbv2.Application{}
	if err := reconciler.deleteHTTPRoute(context.Background(), app); err != nil {
		t.Fatalf("expected deleteHTTPRoute to skip cleanly when HTTPRoute is unsupported, got %v", err)
	}
}
