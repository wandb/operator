package controller

import (
	"testing"

	"github.com/wandb/operator/pkg/wandb/spec"
)

func TestManagedSpecConfigurationMatchesExpectedDifferences(t *testing.T) {
	managedSpec, deployerSpec := testExpectedDifferenceSpecs()

	matches, err := managedSpecConfigurationMatches(managedSpec, deployerSpec)
	if err != nil {
		t.Fatalf("managedSpecConfigurationMatches returned an error: %v", err)
	}
	if !matches {
		t.Fatal("expected managed metadata and ESO credential references to match Deployer literals")
	}
}

func TestManagedSpecConfigurationRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, map[string]interface{})
	}{
		{
			name: "unsupported cloud provider",
			mutate: func(t *testing.T, managed map[string]interface{}) {
				testSetNestedValue(t, managed, "digitalocean", "global", "cloudProvider")
			},
		},
		{
			name: "missing cloud provider",
			mutate: func(t *testing.T, managed map[string]interface{}) {
				testDeleteNestedValue(t, managed, "global", "cloudProvider")
			},
		},
		{
			name: "cloud provider disagrees with Deployer cloud tag",
			mutate: func(t *testing.T, managed map[string]interface{}) {
				testSetNestedValue(t, managed, "aws", "global", "cloudProvider")
			},
		},
		{
			name: "empty customer namespace",
			mutate: func(t *testing.T, managed map[string]interface{}) {
				testSetNestedValue(t, managed, "", "global", "extraEnv", "TAG_CUSTOMER_NS")
			},
		},
		{
			name: "missing customer namespace",
			mutate: func(t *testing.T, managed map[string]interface{}) {
				testDeleteNestedValue(t, managed, "global", "extraEnv", "TAG_CUSTOMER_NS")
			},
		},
		{
			name: "non-string customer namespace",
			mutate: func(t *testing.T, managed map[string]interface{}) {
				testSetNestedValue(t, managed, true, "global", "extraEnv", "TAG_CUSTOMER_NS")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			managedSpec, deployerSpec := testExpectedDifferenceSpecs()
			test.mutate(t, managedSpec.rawValues)

			matches, err := managedSpecConfigurationMatches(managedSpec, deployerSpec)
			if err == nil {
				t.Fatal("managedSpecConfigurationMatches accepted invalid managed metadata")
			}
			if matches {
				t.Fatal("managedSpecConfigurationMatches matched invalid managed metadata")
			}
		})
	}
}

func testExpectedDifferenceSpecs() (*managedSpecSource, *spec.Spec) {
	managedValues := map[string]interface{}{
		"app": map[string]interface{}{
			"enabled": true,
			"env": map[string]interface{}{
				"GORILLA_ORB_USAGE_EVENT_REPORTER_SECRET": testSecretKeyRef("orb-api-key", "api-key"),
			},
		},
		"global": map[string]interface{}{
			"cloudProvider": "gcp",
			"extraEnv": map[string]interface{}{
				"SERVER_FLAG_ENABLE_CORE_WEAVE_OBSERVABILITY": "true",
				"TAG_CLOUD":       "GCP",
				"TAG_CUSTOMER_NS": "wandb-abridge",
			},
		},
		"glue": map[string]interface{}{
			"env": map[string]interface{}{
				"GORILLA_ORB_USAGE_EVENT_REPORTER_SECRET": testSecretKeyRef("orb-api-key", "api-key"),
			},
		},
		"otel": map[string]interface{}{
			"daemonset": map[string]interface{}{
				"config": map[string]interface{}{
					"exporters": map[string]interface{}{
						"datadog": map[string]interface{}{
							"api": map[string]interface{}{
								"key":  "${env:DD_API_KEY}",
								"site": "us5.datadoghq.com",
							},
						},
					},
				},
				"extraEnvFrom": map[string]interface{}{
					"DD_API_KEY": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"key":  "api-key",
							"name": "datadog-api-key",
						},
					},
				},
			},
		},
	}
	deployerValues := map[string]interface{}{
		"app": map[string]interface{}{
			"enabled": true,
			"extraEnv": map[string]interface{}{
				"GORILLA_ORB_USAGE_EVENT_REPORTER_SECRET": "deployer-orb-credential",
			},
			"image": map[string]interface{}{"tag": "deployer-owned"},
		},
		"global": map[string]interface{}{
			"extraEnv": map[string]interface{}{
				"SERVER_FLAG_ENABLE_CORE_WEAVE_OBSERVABILITY": "true",
				"TAG_CLOUD": "GCP",
			},
		},
		"glue": map[string]interface{}{
			"env": map[string]interface{}{
				"GORILLA_ORB_USAGE_EVENT_REPORTER_SECRET": "deployer-orb-credential",
			},
		},
		"otel": map[string]interface{}{
			"daemonset": map[string]interface{}{
				"config": map[string]interface{}{
					"exporters": map[string]interface{}{
						"datadog": map[string]interface{}{
							"api": map[string]interface{}{
								"key":  "deployer-datadog-credential",
								"site": "us5.datadoghq.com",
							},
						},
					},
				},
			},
		},
	}
	managedSpec := &managedSpecSource{
		spec:      testManagedSpec(managedValues),
		rawChart:  map[string]interface{}{"name": "operator-wandb", "url": "https://charts.wandb.ai", "version": "0.43.5"},
		rawValues: managedValues,
	}
	return managedSpec, testManagedSpec(deployerValues)
}

func testSecretKeyRef(name, key string) map[string]interface{} {
	return map[string]interface{}{
		"valueFrom": map[string]interface{}{
			"secretKeyRef": map[string]interface{}{
				"name": name,
				"key":  key,
			},
		},
	}
}

func testSetNestedValue(t *testing.T, root map[string]interface{}, value interface{}, path ...string) {
	t.Helper()
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]interface{})
		if !ok {
			t.Fatalf("test fixture path %v does not contain an object at %q", path, key)
		}
		current = next
	}
	current[path[len(path)-1]] = value
}

func testDeleteNestedValue(t *testing.T, root map[string]interface{}, path ...string) {
	t.Helper()
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]interface{})
		if !ok {
			t.Fatalf("test fixture path %v does not contain an object at %q", path, key)
		}
		current = next
	}
	delete(current, path[len(path)-1])
}
