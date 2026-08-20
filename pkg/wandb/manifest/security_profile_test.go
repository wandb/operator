package manifest_test

import (
	"context"
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
