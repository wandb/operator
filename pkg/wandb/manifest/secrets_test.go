package manifest_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wandb/operator/pkg/wandb/manifest"
)

func TestManifestSecretDeclarationsDecodeAndMerge(t *testing.T) {
	root := t.TempDir()
	versionDir := filepath.Join(root, "test")
	require.NoError(t, os.Mkdir(versionDir, 0o700))
	files := map[string]string{
		"manifest.yaml": `secrets:
  - name: signer
    useExactName: true
  - name: session
    generated:
      length: 24
      type: password
`,
		"sizing.yaml": `secrets:
  - name: default
    generated: {}
generatedSecrets:
  - name: legacy
    length: 12
    useExactName: true
`,
	}
	for name, contents := range files {
		require.NoError(t, os.WriteFile(filepath.Join(versionDir, name), []byte(contents), 0o600), "write %s", name)
	}
	loaded, err := manifest.LoadManifestFromFile(context.Background(), "file://"+filepath.ToSlash(root), "test")
	require.NoError(t, err)
	declarations := loaded.SecretDeclarations()
	require.Len(t, declarations, 4, "declarations must merge across both manifest files")

	t.Run("empty Secret retains exact name without generation", func(t *testing.T) {
		signer := declarations[0]
		require.Equal(t, "signer", signer.Name)
		require.True(t, signer.UseExactName)
		require.Nil(t, signer.Generated)
	})
	t.Run("nested generation settings decode", func(t *testing.T) {
		session := declarations[1]
		require.Equal(t, "session", session.Name)
		require.NotNil(t, session.Generated)
		require.Equal(t, 24, session.Generated.Length)
		require.Equal(t, "password", session.Generated.CharacterType)
	})
	t.Run("explicit empty generated configuration enables defaults", func(t *testing.T) {
		defaultSecret := declarations[2]
		require.Equal(t, "default", defaultSecret.Name)
		require.NotNil(t, defaultSecret.Generated, "generated: {} requests generation; an omitted generated field does not")
	})
	t.Run("legacy declarations normalize without losing settings", func(t *testing.T) {
		legacy := declarations[3]
		require.Equal(t, "legacy", legacy.Name)
		require.NotNil(t, legacy.Generated)
		require.Equal(t, 12, legacy.Generated.Length)
		require.True(t, legacy.UseExactName)
	})
}
