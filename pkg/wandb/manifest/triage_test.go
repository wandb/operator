package manifest_test

import (
	"testing"

	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	"sigs.k8s.io/yaml"
)

func TestTriageActionDecodes(t *testing.T) {
	t.Parallel()

	var decoded serverManifest.Manifest
	input := []byte(`
applications:
  weave-trace:
    triage:
      containerName: weave-trace
      args: [python, -m, weave_triage]
      timeoutSeconds: 600
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
      actions:
        - name: default
          description: Run all diagnostics
`)
	if err := yaml.Unmarshal(input, &decoded); err != nil {
		t.Fatalf("decode manifest triage action: %v", err)
	}
	triage := decoded.Applications["weave-trace"].Triage
	if triage.ContainerName != "weave-trace" || triage.TimeoutSeconds != 600 ||
		len(triage.Actions) != 1 || triage.Actions[0].Name != "default" {
		t.Fatalf("decoded triage = %#v", triage)
	}
}
