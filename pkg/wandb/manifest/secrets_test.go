package manifest_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

func TestManifestSecretDeclarationsDecodeAndMerge(t *testing.T) {
	root := t.TempDir()
	versionDir := filepath.Join(root, "test")
	if err := os.Mkdir(versionDir, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"manifest.yaml": "secrets:\n  - name: signer\n    useExactName: true\n  - name: session\n    generated:\n      length: 24\n      type: password\n",
		"sizing.yaml":   "secrets:\n  - name: default\n    generated: {}\ngeneratedSecrets:\n  - name: legacy\n    length: 12\n    useExactName: true\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(versionDir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m, err := manifest.LoadManifestFromFile(context.Background(), "file://"+filepath.ToSlash(root), "test")
	if err != nil {
		t.Fatal(err)
	}
	declarations := m.SecretDeclarations()
	if len(declarations) != 4 {
		t.Fatalf("expected merged declarations, got %+v", declarations)
	}
	if declarations[0].Generated != nil || !declarations[0].UseExactName {
		t.Fatal("empty declaration gained generation")
	}
	if declarations[1].Generated == nil || declarations[1].Generated.Length != 24 || declarations[1].Generated.CharacterType != "password" {
		t.Fatal("nested generated configuration not decoded")
	}
	if declarations[2].Generated == nil {
		t.Fatal("generated: {} must enable default generation")
	}
	if declarations[3].Name != "legacy" || declarations[3].Generated == nil || declarations[3].Generated.Length != 12 || !declarations[3].UseExactName {
		t.Fatal("legacy normalization changed generation or naming")
	}
}
