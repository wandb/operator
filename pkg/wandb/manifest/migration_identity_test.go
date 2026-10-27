package manifest_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wandb/operator/pkg/wandb/manifest"
)

func TestMigrationServiceAccountManifest(t *testing.T) {
	tests := []struct {
		name                  string
		manifestYAML          string
		wantDedicatedIdentity bool
	}{
		{
			name: "signer declares dedicated API identity",
			manifestYAML: `migrations:
  internal-signer:
    serviceAccount:
      automountServiceAccountToken: true
      permissions:
        - resources: [secrets]
          resourceNames: [wandb-internal-signer]
          verbs: [get, update]
  gorilla: {}
`,
			wantDedicatedIdentity: true,
		},
		{
			name: "legacy migrations omit dedicated identity",
			manifestYAML: `migrations:
  internal-signer: {}
  gorilla: {}
`,
			wantDedicatedIdentity: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "test.yaml"), []byte(test.manifestYAML), 0o600))
			loaded, err := manifest.LoadManifestFromFile(context.Background(), "file://"+filepath.ToSlash(root), "test")
			require.NoError(t, err)
			require.Nil(t, loaded.Migrations["gorilla"].ServiceAccount, "ordinary migration must retain the shared identity")
			identity := loaded.Migrations["internal-signer"].ServiceAccount
			if !test.wantDedicatedIdentity {
				require.Nil(t, identity, "legacy manifest must not acquire a dedicated identity")
				return
			}
			require.NotNil(t, identity)
			require.Empty(t, identity.Name, "omitted name lets the operator derive a dedicated account name")
			require.Len(t, identity.Permissions, 1)
			require.Equal(t, []string{"wandb-internal-signer"}, identity.Permissions[0].ResourceNames)
			require.NotNil(t, identity.AutomountServiceAccountToken)
			require.True(t, *identity.AutomountServiceAccountToken)
			require.Empty(t, loaded.Migrations["internal-signer"].VolumeMounts, "signer uses automatic credentials rather than explicit token volumes")
		})
	}
}
