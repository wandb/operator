package manifest

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
)

func TestManifestVersionMetadata(t *testing.T) {
	for _, value := range []string{"0", "-1", "null", "", `"1"`, `""`, "true", "1.0", "[1]", "{version: 1}", "2147483648", "+1", "01", "0x1"} {
		t.Run(value, func(t *testing.T) {
			_, err := decodeManifestFiles(map[string][]byte{"manifest.yaml": []byte("manifestVersion: " + value + "\n")})
			var invalid *InvalidManifestVersionError
			require.ErrorAs(t, err, &invalid)
			require.Equal(t, "manifest.yaml", invalid.File)
		})
	}
	for _, data := range []string{
		"manifestVersion: 1\nmanifestVersion: 1\n",
		"ManifestVersion: 2\n",
		"defaults: &defaults {manifestVersion: 2}\n<<: *defaults\n",
		"value: &version 2\nmanifestVersion: *version\n",
		"key: &key manifestVersion\n? *key\n: 2\n",
	} {
		_, err := decodeManifestFiles(map[string][]byte{"manifest.yaml": []byte(data)})
		var invalid *InvalidManifestVersionError
		require.ErrorAs(t, err, &invalid)
	}
}

func TestManifestVersionAcrossFragments(t *testing.T) {
	for _, tc := range []struct {
		name        string
		root        string
		fragment    string
		unsupported bool
		invalid     bool
		explicit    bool
	}{
		{name: "legacy"},
		{name: "explicit", root: "manifestVersion: 1\n", explicit: true},
		{name: "same declarations", root: "manifestVersion: 1\n", fragment: "manifestVersion: 1\n", explicit: true},
		{name: "metadata in later file", fragment: "manifestVersion: 1\n", explicit: true},
		{name: "unversioned fragment inherits", root: "manifestVersion: 2\n", unsupported: true},
		{name: "conflict", root: "manifestVersion: 1\n", fragment: "manifestVersion: 2\n", invalid: true},
		{name: "invalid fragment", root: "manifestVersion: 1\n", fragment: "manifestVersion: null\n", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := decodeManifestFiles(map[string][]byte{
				"manifest.yaml": []byte(tc.root + "features: {queue: true}\n"),
				"sizing.yaml":   []byte(tc.fragment + "mysql: {default: {}}\n"),
			})
			switch {
			case tc.unsupported:
				var unsupported *UnsupportedManifestVersionError
				require.ErrorAs(t, err, &unsupported)
				require.Equal(t, 2, unsupported.Version)
				require.Equal(t, []int{1}, unsupported.Supported)
			case tc.invalid:
				var invalid *InvalidManifestVersionError
				require.ErrorAs(t, err, &invalid)
			default:
				require.NoError(t, err)
				require.Equal(t, 1, m.ManifestVersion)
				require.Equal(t, tc.explicit, m.VersionExplicit())
				require.True(t, m.Features["queue"])
				require.Contains(t, m.Mysql, "default")
			}
		})
	}
	versions := SupportedVersions()
	require.Equal(t, []int{1}, versions)
	versions[0] = 2
	require.Equal(t, []int{1}, SupportedVersions())
	require.Error(t, ValidateVersion(0))
}

func TestManifestVersionBeforePayloadDecoding(t *testing.T) {
	_, err := decodeManifestFiles(map[string][]byte{"manifest.yaml": []byte("manifestVersion: 2\napplications: [future, format]\n")})
	var unsupported *UnsupportedManifestVersionError
	require.ErrorAs(t, err, &unsupported)
	for _, data := range []string{"applications: [broken", "manifestVersion: 1\napplications: [wrong, shape]\n", "manifestVersion: 1\n---\nmanifestVersion: 2\n"} {
		_, err := decodeManifestFiles(map[string][]byte{"manifest.yaml": []byte(data)})
		var decode *ManifestDecodeError
		require.ErrorAs(t, err, &decode)
	}
}

func TestExplicitVersion1PreservesLegacyPayload(t *testing.T) {
	paths, err := filepath.Glob("../../../hack/testing-manifests/server-manifest/0.83.0-clickhouse-keeper.2/*.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	files := make(map[string][]byte, len(paths))
	for _, path := range paths {
		files[filepath.Base(path)], err = os.ReadFile(path)
		require.NoError(t, err)
	}
	legacy, err := decodeManifestFiles(files)
	require.NoError(t, err)
	require.False(t, legacy.VersionExplicit())
	files["manifest.yaml"] = append(files["manifest.yaml"], []byte("\nmanifestVersion: 1\n")...)
	explicit, err := decodeManifestFiles(files)
	require.NoError(t, err)
	require.True(t, explicit.VersionExplicit())
	explicit.versionExplicit = false // Only diagnostic provenance may differ.
	require.Equal(t, legacy, explicit)
}

func TestManifestVersionLocalAndOCI(t *testing.T) {
	for i, prefix := range []string{"", "manifestVersion: 1\n", "manifestVersion: 2\n", "manifestVersion: null\n"} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			files := map[string][]byte{
				"manifest.yaml": []byte(prefix + "features: {queue: true}\nrequiredOperatorVersion: 'not a semver constraint'\n"),
				"sizing.yaml":   []byte("mysql: {default: {}}\n"),
			}
			dir := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(dir, "server"), 0755))
			for name, data := range files {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "server", name), data, 0600))
			}
			expected, expectedErr := LoadManifestFromFile(context.Background(), "file://"+dir, "server")
			store, descriptor := manifestOCI(t, files)
			// Reusing the same store exercises decoding on warm loads as well.
			for range 2 {
				actual, err := processManifest(context.Background(), store, descriptor, slog.Default())
				if expectedErr != nil {
					require.IsType(t, expectedErr, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, expected, actual)
				}
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, "single.yaml"), files["manifest.yaml"], 0600))
			single, err := LoadManifestFromFile(context.Background(), "file://"+dir, "single")
			if expectedErr != nil {
				require.IsType(t, expectedErr, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, expected.ManifestVersion, single.ManifestVersion)
			}
		})
	}
}

func TestManifestMissingLayerDoesNotDefaultToVersion1(t *testing.T) {
	missing := content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, []byte("missing version metadata"))
	store, descriptor := manifestOCI(t, map[string][]byte{"sizing.yaml": []byte("mysql: {}\n")}, missing)
	_, err := processManifest(context.Background(), store, descriptor, slog.Default())
	require.ErrorContains(t, err, "fetch manifest layer")
}

func manifestOCI(t *testing.T, files map[string][]byte, extraLayers ...ocispec.Descriptor) (*memory.Store, ocispec.Descriptor) {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, data := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(data))}))
		_, err := tw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	store := memory.New()
	layer := content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, buf.Bytes())
	require.NoError(t, store.Push(context.Background(), layer, bytes.NewReader(buf.Bytes())))
	index := ocispec.Manifest{Layers: append([]ocispec.Descriptor{layer}, extraLayers...)}
	data, err := json.Marshal(index)
	require.NoError(t, err)
	descriptor := content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, data)
	require.NoError(t, store.Push(context.Background(), descriptor, bytes.NewReader(data)))
	return store, descriptor
}
