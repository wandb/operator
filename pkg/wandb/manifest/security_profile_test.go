package manifest_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	manifest "github.com/wandb/operator/pkg/wandb/manifest"
)

func TestWorkloadSecurityProfileDecodeAndMultiFileMerge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	version := "security-profile-test"
	versionDir := filepath.Join(root, version)
	if err := os.Mkdir(versionDir, 0o755); err != nil {
		t.Fatalf("create manifest directory: %v", err)
	}

	manifestYAML := []byte(`
requiredOperatorVersion: ^2.0.0
applications:
  api:
    name: api
    image:
      repository: example/api
    securityProfile:
      runAsNonRoot: false
      readOnlyRootFilesystem: true
  legacy-compatible-empty:
    name: legacy-compatible-empty
    image:
      repository: example/legacy-compatible-empty
    securityProfile: {}
migrations:
  gorilla:
    image:
      repository: example/migrate
    securityProfile:
      runAsNonRoot: true
      readOnlyRootFilesystem: false
  legacy-compatible-empty:
    image:
      repository: example/legacy-compatible-empty
    securityProfile: {}
`)
	sizingYAML := []byte(`
applications:
  api:
    sizing:
      default:
        replicas: 1
`)
	if err := os.WriteFile(filepath.Join(versionDir, "manifest.yaml"), manifestYAML, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "sizing.yaml"), sizingYAML, 0o600); err != nil {
		t.Fatalf("write sizing: %v", err)
	}

	loaded, err := manifest.LoadManifestFromFile(
		context.Background(),
		"file://"+filepath.ToSlash(root),
		version,
	)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	appProfile := loaded.Applications["api"].SecurityProfile
	if appProfile == nil || appProfile.RunAsNonRoot == nil || *appProfile.RunAsNonRoot {
		t.Fatalf("application runAsNonRoot = %#v, want explicit false", appProfile)
	}
	if appProfile.ReadOnlyRootFilesystem == nil || !*appProfile.ReadOnlyRootFilesystem {
		t.Fatalf("application readOnlyRootFilesystem = %#v, want true", appProfile)
	}
	if loaded.Applications["api"].Sizing["default"].Replicas != 1 {
		t.Fatal("sizing fragment was not merged with the profiled application")
	}
	emptyAppProfile := loaded.Applications["legacy-compatible-empty"].SecurityProfile
	if emptyAppProfile == nil || emptyAppProfile.RunAsNonRoot != nil || emptyAppProfile.ReadOnlyRootFilesystem != nil {
		t.Fatalf("empty application security profile = %#v, want non-nil profile with nil settings", emptyAppProfile)
	}

	migrationProfile := loaded.Migrations["gorilla"].SecurityProfile
	if migrationProfile == nil || migrationProfile.RunAsNonRoot == nil || !*migrationProfile.RunAsNonRoot {
		t.Fatalf("migration runAsNonRoot = %#v, want true", migrationProfile)
	}
	if migrationProfile.ReadOnlyRootFilesystem == nil || *migrationProfile.ReadOnlyRootFilesystem {
		t.Fatalf("migration readOnlyRootFilesystem = %#v, want explicit false", migrationProfile)
	}
	emptyMigrationProfile := loaded.Migrations["legacy-compatible-empty"].SecurityProfile
	if emptyMigrationProfile == nil || emptyMigrationProfile.RunAsNonRoot != nil || emptyMigrationProfile.ReadOnlyRootFilesystem != nil {
		t.Fatalf("empty migration security profile = %#v, want non-nil profile with nil settings", emptyMigrationProfile)
	}
}

func TestLegacyManifestFixturesHaveNoWorkloadSecurityProfiles(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("../../../hack/testing-manifests/server-manifest")
	if err != nil {
		t.Fatalf("resolve manifest fixture root: %v", err)
	}

	for _, version := range []string{
		"0.83.0-clickhouse-keeper.1",
		"0.83.0-clickhouse-keeper.2",
	} {
		version := version
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			loaded, err := manifest.LoadManifestFromFile(
				context.Background(),
				"file://"+filepath.ToSlash(root),
				version,
			)
			if err != nil {
				t.Fatalf("load legacy manifest: %v", err)
			}

			for name, app := range loaded.Applications {
				if app.SecurityProfile != nil {
					t.Fatalf("legacy application %q unexpectedly has security profile %#v", name, app.SecurityProfile)
				}
			}
			for name, migration := range loaded.Migrations {
				if migration.SecurityProfile != nil {
					t.Fatalf("legacy migration %q unexpectedly has security profile %#v", name, migration.SecurityProfile)
				}
			}
		})
	}
}

// Exercise the public loader so filename ordering and both merge strategies are covered.
func TestSecurityProfileFragmentPrecedence(t *testing.T) {
	for _, test := range []struct {
		name, overlay     string
		nonRoot, readOnly *bool
	}{
		{"omitted", "", profileBool(true), profileBool(true)},
		{"empty", "    securityProfile: {}", profileBool(true), profileBool(true)},
		{"false overrides", "    securityProfile: {runAsNonRoot: false, readOnlyRootFilesystem: false}", profileBool(false), profileBool(false)},
		{"partial inherits", "    securityProfile: {runAsNonRoot: false}", profileBool(false), profileBool(true)},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			versionDir := filepath.Join(root, "test")
			if err := os.Mkdir(versionDir, 0o700); err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"00-base.yaml": `applications:
  api:
    name: api
    image: {repository: example/api}
    securityProfile: {runAsNonRoot: true, readOnlyRootFilesystem: true}
migrations:
  migrate:
    image: {repository: example/old}
    args: [old]
    securityProfile: {runAsNonRoot: true, readOnlyRootFilesystem: true}
`,
				"10-overlay.yaml": fmt.Sprintf(`applications:
  api:
    sizing: {default: {replicas: 2}}
%s
migrations:
  migrate:
    image: {repository: example/replacement}
%s
`, test.overlay, test.overlay),
			}
			for name, body := range files {
				if err := os.WriteFile(filepath.Join(versionDir, name), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			loaded, err := manifest.LoadManifestFromFile(context.Background(), "file://"+filepath.ToSlash(root), "test")
			if err != nil {
				t.Fatal(err)
			}
			app := loaded.Applications["api"]
			checkProfile(t, app.SecurityProfile, test.nonRoot, test.readOnly)
			if app.Image.Repository != "example/api" || app.Sizing["default"].Replicas != 2 {
				t.Fatalf("lost application fields: %#v", app)
			}
			migration := loaded.Migrations["migrate"]
			if migration.Image.Repository != "example/replacement" || len(migration.Args) != 0 {
				t.Fatalf("migration was not replaced: %#v", migration)
			}
			switch test.name {
			case "omitted", "empty":
				checkProfile(t, migration.SecurityProfile, nil, nil)
			case "partial inherits":
				checkProfile(t, migration.SecurityProfile, profileBool(false), nil)
			default:
				checkProfile(t, migration.SecurityProfile, profileBool(false), profileBool(false))
			}
		})
	}
}

func profileBool(value bool) *bool { return &value }

func checkProfile(t *testing.T, profile *manifest.WorkloadSecurityProfile, nonRoot, readOnly *bool) {
	t.Helper()
	var gotNonRoot, gotReadOnly *bool
	if profile != nil {
		gotNonRoot, gotReadOnly = profile.RunAsNonRoot, profile.ReadOnlyRootFilesystem
	}
	for _, pair := range []struct {
		name      string
		got, want *bool
	}{{"runAsNonRoot", gotNonRoot, nonRoot}, {"readOnlyRootFilesystem", gotReadOnly, readOnly}} {
		if (pair.got == nil) != (pair.want == nil) || pair.got != nil && *pair.got != *pair.want {
			t.Errorf("%s = %v, want %v", pair.name, pair.got, pair.want)
		}
	}
}
