package controller

import (
	"bytes"
	"context"
	"strings"
	"testing"

	appsv1 "github.com/wandb/operator/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestReconcileDoesNotLogCredentials(t *testing.T) {
	r, wandb, _ := newUserSpecReconciler(t, "deployer")
	r.Debug = true
	ctx := context.Background()
	setSpec := func(dbPassword string) {
		t.Helper()
		if err := r.Get(ctx, client.ObjectKeyFromObject(wandb), wandb); err != nil {
			t.Fatal(err)
		}
		wandb.Spec.Chart = appsv1.Object{Object: map[string]interface{}{
			"url": "https://charts.example.invalid", "name": "operator-wandb", "version": "1.0.0",
			"username": "chart-user", "password": "chart-pass-1",
		}}
		global := wandb.Spec.Values.Object["global"].(map[string]interface{})
		global["mysql"] = map[string]interface{}{"password": dbPassword}
		global["extraEnv"] = map[string]interface{}{
			"GORILLA_HISTORY_STORE": "http://wandb-parquet:8087/_goRPC_,mysql://wandb:store-pass-1@mysql:3306/wandb_local",
		}
		wandb.Spec.Values.Object["parquet"] = map[string]interface{}{"env": map[string]interface{}{
			"GORILLA_HISTORY_STORAGE_ENGINE_ADDRESS": "clickhouse://wandb:dsn-pass-1@clickhouse:9000",
		}}
		wandb.Spec.Values.Object["app"] = map[string]interface{}{"extraEnv": []interface{}{
			map[string]interface{}{"name": "SMTP_PASSWORD", "value": "env-pass-1"},
			map[string]interface{}{"name": "REDIS", "value": "redis://:url-pass-1@redis:6379"},
		}}
		if err := r.Update(ctx, wandb); err != nil {
			t.Fatal(err)
		}
	}

	var logs bytes.Buffer
	ctx = ctrllog.IntoContext(ctx, zap.New(zap.UseFlagOptions(&zap.Options{Development: true}), zap.WriteTo(&logs)))
	request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(wandb)}
	// The second pass changes a password so the active/desired diff is logged.
	for _, dbPassword := range []string{"db-pass-1", "db-pass-2"} {
		setSpec(dbPassword)
		if _, err := r.Reconcile(ctx, request); err != nil {
			t.Fatal(err)
		}
	}

	out := logs.String()
	for _, want := range []string{"Changes found", "Desired spec", "cr.example.invalid", "redis://***@redis:6379"} {
		if !strings.Contains(out, want) {
			t.Errorf("logs do not contain %q", want)
		}
	}
	for _, secret := range []string{"chart-pass-1", "db-pass-1", "db-pass-2", "dsn-pass-1", "env-pass-1", "url-pass-1", "store-pass-1"} {
		if strings.Contains(out, secret) {
			t.Errorf("logs contain credential %q", secret)
		}
	}
}
