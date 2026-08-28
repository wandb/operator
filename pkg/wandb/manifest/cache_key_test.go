package manifest

import "testing"

// The on-disk manifest cache is namespaced by repository so two CRs pulling
// different (possibly private) registries under the same version tag can never
// resolve each other's cached artifact.
func TestRepositoryCacheKey(t *testing.T) {
	a := repositoryCacheKey("reg-a.example.com/wandb/server-manifest")
	b := repositoryCacheKey("reg-b.example.com/wandb/server-manifest")
	if a == b {
		t.Fatalf("different repositories must not share a cache key: %q", a)
	}
	if a != repositoryCacheKey("reg-a.example.com/wandb/server-manifest") {
		t.Fatal("cache key must be stable for the same repository")
	}
}
