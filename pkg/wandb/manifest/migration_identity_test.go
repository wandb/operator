package manifest_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

func TestMigrationServiceAccountManifest(t *testing.T) {
	root := t.TempDir()
	for version, contents := range map[string]string{
		"0.86.0-daily.14": `migrations:
  internal-signer:
    serviceAccount:
      automountServiceAccountToken: true
      permissions:
        - resources: [secrets]
          resourceNames: [wandb-internal-signer]
          verbs: [get, update]
  gorilla: {}
`,
		"0.85.1": `migrations:
  internal-signer: {}
  gorilla: {}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, version+".yaml"), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, version := range []string{"0.86.0-daily.14", "0.85.1"} {
		t.Run(version, func(t *testing.T) {
			m, err := manifest.LoadManifestFromFile(context.Background(), "file://"+filepath.ToSlash(root), version)
			if err != nil {
				t.Fatal(err)
			}
			identity := m.Migrations["internal-signer"].ServiceAccount
			if version == "0.85.1" {
				if identity != nil {
					t.Fatal("legacy manifest acquired dedicated identity")
				}
				return
			}
			if identity == nil || identity.Name != "" || len(identity.Permissions) != 1 || len(identity.Permissions[0].ResourceNames) != 1 || identity.Permissions[0].ResourceNames[0] != "wandb-internal-signer" {
				t.Fatalf("signer identity was not decoded: %+v", identity)
			}
			if identity.AutomountServiceAccountToken == nil || !*identity.AutomountServiceAccountToken {
				t.Fatal("signer manifest must enable token automounting")
			}
			if len(m.Migrations["internal-signer"].VolumeMounts) != 0 {
				t.Fatal("signer manifest must not declare explicit credential volumes")
			}
			if m.Migrations["gorilla"].ServiceAccount != nil {
				t.Fatal("ordinary migration acquired signer identity")
			}
		})
	}
}
