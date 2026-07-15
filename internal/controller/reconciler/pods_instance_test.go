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

func mysqlURLSelector(secretName string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
		Key:                  "url",
	}
}

func wandbWithTwoMysqlInstances() *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wb", Namespace: "default"},
		Status: apiv2.WeightsAndBiasesStatus{
			MySQLStatus: map[string]apiv2.MysqlInfraStatus{
				apiv2.DefaultInstanceName: {Connection: apiv2.MysqlConnection{URL: mysqlURLSelector("default-conn")}},
				"analytics":               {Connection: apiv2.MysqlConnection{URL: mysqlURLSelector("analytics-conn")}},
			},
		},
	}
}

func resolveSingleMysqlEnv(t *testing.T, wandb *apiv2.WeightsAndBiases, instance string) corev1.EnvVar {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	envs := []serverManifest.EnvVar{
		{Name: "MYSQL", Sources: []serverManifest.EnvSource{{Type: "mysql", Name: instance}}},
	}
	resolved, err := resolveEnvvars(context.Background(), client, wandb, serverManifest.Manifest{}, nil, envs)
	if err != nil {
		t.Fatalf("resolveEnvvars returned error: %v", err)
	}
	return mustFindEnvVar(t, resolved, "MYSQL")
}

func TestResolveEnvvarsMysqlRoutesToNamedInstance(t *testing.T) {
	env := resolveSingleMysqlEnv(t, wandbWithTwoMysqlInstances(), "analytics")
	if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected secret-backed env var, got %+v", env)
	}
	if got := env.ValueFrom.SecretKeyRef.Name; got != "analytics-conn" {
		t.Fatalf("expected analytics-conn, got %q", got)
	}
}

func TestResolveEnvvarsMysqlEmptyInstanceUsesDefault(t *testing.T) {
	env := resolveSingleMysqlEnv(t, wandbWithTwoMysqlInstances(), "")
	if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected secret-backed env var, got %+v", env)
	}
	if got := env.ValueFrom.SecretKeyRef.Name; got != "default-conn" {
		t.Fatalf("expected default-conn, got %q", got)
	}
}

func TestResolveEnvvarsMysqlMissingInstanceFallsBackToDefault(t *testing.T) {
	env := resolveSingleMysqlEnv(t, wandbWithTwoMysqlInstances(), "does-not-exist")
	if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected secret-backed env var, got %+v", env)
	}
	if got := env.ValueFrom.SecretKeyRef.Name; got != "default-conn" {
		t.Fatalf("expected fallback to default-conn, got %q", got)
	}
}
