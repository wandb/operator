package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const clickHouseConnSecret = "wandb-clickhouse-connection"

func clickHouseSel(key string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: clickHouseConnSecret},
		Key:                  key,
	}
}

// wandbWithClickHouseConnection publishes a connection whose topology selectors
// are whatever the infra layer resolved — set for managed and for an external
// ClickHouse that declared it, unset otherwise.
func wandbWithClickHouseConnection(conn apiv2.ClickHouseConnection) *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Status: apiv2.WeightsAndBiasesStatus{
			ClickHouseStatus: map[string]apiv2.ClickHouseInfraStatus{
				apiv2.DefaultInstanceName: {Connection: conn},
			},
		},
	}
}

func fullClickHouseConnection() apiv2.ClickHouseConnection {
	return apiv2.ClickHouseConnection{
		URL:         clickHouseSel("url"),
		Host:        clickHouseSel("Host"),
		HTTPPort:    clickHouseSel("HTTPPort"),
		TCPPort:     clickHouseSel("TCPPort"),
		Username:    clickHouseSel("User"),
		Password:    clickHouseSel("Password"),
		Database:    clickHouseSel("Database"),
		Replicated:  clickHouseSel("Replicated"),
		ClusterName: clickHouseSel("ClusterName"),
	}
}

// resolveClickHouseEnv resolves a single manifest env var backed by a clickhouse
// source field.
func resolveClickHouseEnv(t *testing.T, wandb *apiv2.WeightsAndBiases, name, field string) (corev1.EnvVar, bool) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	envs := []serverManifest.EnvVar{
		{Name: name, Sources: []serverManifest.EnvSource{{
			Type:  "clickhouse",
			Name:  apiv2.DefaultInstanceName,
			Field: field,
		}}},
	}
	resolved, err := resolveEnvvars(context.Background(), client, wandb, serverManifest.Manifest{}, nil, envs)
	if err != nil {
		t.Fatalf("resolveEnvvars returned error: %v", err)
	}
	for _, env := range resolved {
		if env.Name == name {
			return env, true
		}
	}
	return corev1.EnvVar{}, false
}

// Weave reads replication out of the connection Secret, the same way it reads
// host and database.
func TestResolveEnvvarsClickHouseReplicationFromConnectionSecret(t *testing.T) {
	wandb := wandbWithClickHouseConnection(fullClickHouseConnection())

	for _, tc := range []struct {
		field string
		key   string
	}{
		{"replicated", "Replicated"},
		{"replicated-cluster", "ClusterName"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			env, found := resolveClickHouseEnv(t, wandb, "WF_CLICKHOUSE_TEST", tc.field)
			if !found {
				t.Fatalf("expected an env var for field %q", tc.field)
			}
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Fatalf("expected a secret-backed env var, got %+v", env)
			}
			if got := env.ValueFrom.SecretKeyRef.Name; got != clickHouseConnSecret {
				t.Errorf("expected secret %q, got %q", clickHouseConnSecret, got)
			}
			if got := env.ValueFrom.SecretKeyRef.Key; got != tc.key {
				t.Errorf("expected key %q, got %q", tc.key, got)
			}
		})
	}
}

// An external ClickHouse that declared no topology publishes no selector.
// Mounting a key that isn't in the Secret keeps the container from starting, so
// the env var has to be dropped and the application left on its own default.
func TestResolveEnvvarsClickHouseReplicationSkippedWhenUnpublished(t *testing.T) {
	conn := fullClickHouseConnection()
	conn.Replicated = corev1.SecretKeySelector{}
	conn.ClusterName = corev1.SecretKeySelector{}
	wandb := wandbWithClickHouseConnection(conn)

	for _, field := range []string{"replicated", "replicated-cluster"} {
		t.Run(field, func(t *testing.T) {
			if _, found := resolveClickHouseEnv(t, wandb, "WF_CLICKHOUSE_TEST", field); found {
				t.Error("expected no env var when the connection publishes no topology")
			}
		})
	}
}

// The rest of the connection must keep resolving whether or not topology is
// published.
func TestResolveEnvvarsClickHouseHostUnaffectedByTopology(t *testing.T) {
	conn := fullClickHouseConnection()
	conn.Replicated = corev1.SecretKeySelector{}
	conn.ClusterName = corev1.SecretKeySelector{}

	env, found := resolveClickHouseEnv(t, wandbWithClickHouseConnection(conn), "WF_CLICKHOUSE_HOST", "host")
	if !found {
		t.Fatal("expected WF_CLICKHOUSE_HOST to resolve")
	}
	if env.ValueFrom.SecretKeyRef.Key != "Host" {
		t.Errorf("expected key Host, got %q", env.ValueFrom.SecretKeyRef.Key)
	}
}
