package controller

import (
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsAPISupported(t *testing.T) {
	t.Run("returns false when mapping is absent", func(t *testing.T) {
		mapper := apimeta.NewDefaultRESTMapper([]schema.GroupVersion{
			{Group: "apps", Version: "v1"},
		})

		supported, err := isAPISupported(
			mapper,
			schema.GroupKind{Group: "gateway.networking.k8s.io", Kind: "Gateway"},
			"v1",
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if supported {
			t.Fatalf("expected gateway api to be unsupported")
		}
	})

	t.Run("returns true when mapping exists", func(t *testing.T) {
		groupVersion := schema.GroupVersion{Group: "gateway.networking.k8s.io", Version: "v1"}
		mapper := apimeta.NewDefaultRESTMapper([]schema.GroupVersion{groupVersion})
		mapper.Add(schema.GroupVersionKind{Group: groupVersion.Group, Version: groupVersion.Version, Kind: "Gateway"}, apimeta.RESTScopeNamespace)

		supported, err := isAPISupported(
			mapper,
			schema.GroupKind{Group: groupVersion.Group, Kind: "Gateway"},
			groupVersion.Version,
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !supported {
			t.Fatalf("expected gateway api to be supported")
		}
	})
}
