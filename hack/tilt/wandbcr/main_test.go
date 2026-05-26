package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/wandb/operator/api/v2"
	"sigs.k8s.io/yaml"
)

func TestBuildCRDefaultGateway(t *testing.T) {
	cr, err := BuildCR(Options{})
	if err != nil {
		t.Fatal(err)
	}

	if cr.APIVersion != "apps.wandb.com/v2" || cr.Kind != "WeightsAndBiases" {
		t.Fatalf("unexpected type meta: %s %s", cr.APIVersion, cr.Kind)
	}
	if cr.Name != defaultName || cr.Namespace != defaultNamespace {
		t.Fatalf("unexpected metadata: %s/%s", cr.Namespace, cr.Name)
	}
	if cr.Spec.Wandb.ManifestRepository != "" {
		t.Fatalf("manifest repository = %q", cr.Spec.Wandb.ManifestRepository)
	}
	if cr.Spec.Wandb.Version != defaultVersion {
		t.Fatalf("version = %q", cr.Spec.Wandb.Version)
	}
	if !cr.Spec.Wandb.Features["proxy"] {
		t.Fatalf("proxy feature not enabled")
	}
	if cr.Spec.Wandb.InternalServiceAuth.Enabled == nil || *cr.Spec.Wandb.InternalServiceAuth.Enabled {
		t.Fatalf("internal service auth should be explicitly disabled")
	}
	if cr.Spec.MySQL.ManagedMysql == nil || cr.Spec.MySQL.ManagedMysql.Telemetry.Enabled {
		t.Fatalf("mysql telemetry should be disabled by default")
	}
	if cr.Spec.Redis.ManagedRedis == nil || cr.Spec.Redis.ManagedRedis.Telemetry.Enabled {
		t.Fatalf("redis telemetry should be disabled by default")
	}
	if cr.Spec.Kafka.ManagedKafka == nil || cr.Spec.Kafka.ManagedKafka.Telemetry.Enabled {
		t.Fatalf("kafka telemetry should be disabled by default")
	}
	if cr.Spec.ObjectStore.ManagedObjectStore == nil || cr.Spec.ObjectStore.ManagedObjectStore.Telemetry.Enabled {
		t.Fatalf("object store telemetry should be disabled by default")
	}
	if cr.Spec.ClickHouse.ManagedClickHouse == nil || cr.Spec.ClickHouse.ManagedClickHouse.Telemetry.Enabled {
		t.Fatalf("clickhouse telemetry should be disabled by default")
	}
	if cr.Spec.Networking.Mode != v2.NetworkingModeGatewayAPI {
		t.Fatalf("networking mode = %q", cr.Spec.Networking.Mode)
	}
	if cr.Spec.Networking.GatewayAPI == nil || cr.Spec.Networking.GatewayAPI.Gateway.GatewayClassName == nil || *cr.Spec.Networking.GatewayAPI.Gateway.GatewayClassName != "nginx" {
		t.Fatalf("gateway class was not set")
	}
}

func TestBuildCRLocalManifestSource(t *testing.T) {
	cr, err := BuildCR(Options{ManifestSource: "local"})
	if err != nil {
		t.Fatal(err)
	}

	if cr.Spec.Wandb.ManifestRepository != localManifestRepository {
		t.Fatalf("manifest repository = %q", cr.Spec.Wandb.ManifestRepository)
	}
}

func TestBuildCRInvalidManifestSourceReturnsError(t *testing.T) {
	_, err := BuildCR(Options{ManifestSource: "testing-manifests"})
	if err == nil {
		t.Fatal("expected manifest source error")
	}
	if !strings.Contains(err.Error(), "manifest-source") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildCRIngressRewritesDefaultHostname(t *testing.T) {
	cr, err := BuildCR(Options{NetworkMode: "ingress"})
	if err != nil {
		t.Fatal(err)
	}

	if cr.Spec.Wandb.Hostname != defaultIngressHostname {
		t.Fatalf("hostname = %q", cr.Spec.Wandb.Hostname)
	}
	if cr.Spec.Networking.Mode != v2.NetworkingModeIngress {
		t.Fatalf("networking mode = %q", cr.Spec.Networking.Mode)
	}
	if cr.Spec.Networking.Ingress == nil || cr.Spec.Networking.Ingress.IngressClassName == nil || *cr.Spec.Networking.Ingress.IngressClassName != "nginx" {
		t.Fatalf("ingress class was not set")
	}
	if cr.Spec.Networking.GatewayAPI != nil {
		t.Fatalf("gateway config should be cleared in ingress mode")
	}
}

func TestBuildCRInvalidNetworkModeReturnsError(t *testing.T) {
	_, err := BuildCR(Options{NetworkMode: "networking-ingress-local"})
	if err == nil {
		t.Fatal("expected network mode error")
	}
	if !strings.Contains(err.Error(), "network-mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildCRFullObservabilityEnablesManagedTelemetry(t *testing.T) {
	cr, err := BuildCR(Options{ObservabilityMode: "full"})
	if err != nil {
		t.Fatal(err)
	}

	if !cr.Spec.MySQL.ManagedMysql.Telemetry.Enabled ||
		!cr.Spec.Redis.ManagedRedis.Telemetry.Enabled ||
		!cr.Spec.Kafka.ManagedKafka.Telemetry.Enabled ||
		!cr.Spec.ObjectStore.ManagedObjectStore.Telemetry.Enabled ||
		!cr.Spec.ClickHouse.ManagedClickHouse.Telemetry.Enabled {
		t.Fatalf("managed telemetry was not enabled")
	}
}

func TestBuildCRInvalidObservabilityModeReturnsError(t *testing.T) {
	_, err := BuildCR(Options{ObservabilityMode: "on"})
	if err == nil {
		t.Fatal("expected observability mode error")
	}
	if !strings.Contains(err.Error(), "observability-mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildCRPatchesBaseCRAndPreservesUnrelatedFields(t *testing.T) {
	dir := t.TempDir()
	crFile := filepath.Join(dir, "base.yaml")
	base := `apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: custom
  namespace: custom-ns
  labels:
    keep: me
spec:
  requireLimits: true
  wandb:
    hostname: http://old.example
    manifestRepository: file:///old-manifest
    version: old
    additionalHostnames:
      - extra.example
  networking:
    mode: ingress
    ingress:
      ingressClassName: old
`
	if err := os.WriteFile(crFile, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	cr, err := BuildCR(Options{
		CRFile:          crFile,
		Name:            "patched",
		Namespace:       "patched-ns",
		Hostname:        "http://new.example",
		Version:         "1.2.3",
		Size:            "micro",
		RetentionPolicy: "purge",
		NetworkMode:     "gateway",
		GatewayClass:    "custom-gateway",
	})
	if err != nil {
		t.Fatal(err)
	}

	if cr.Name != "patched" || cr.Namespace != "patched-ns" {
		t.Fatalf("metadata was not patched: %s/%s", cr.Namespace, cr.Name)
	}
	if cr.Labels["keep"] != "me" || cr.Labels["app.kubernetes.io/managed-by"] != "tilt" {
		t.Fatalf("labels not preserved/patched: %#v", cr.Labels)
	}
	if !cr.Spec.RequireLimits {
		t.Fatalf("unrelated spec field was not preserved")
	}
	if len(cr.Spec.Wandb.AdditionalHostnames) != 1 || cr.Spec.Wandb.AdditionalHostnames[0] != "extra.example" {
		t.Fatalf("additional hostnames were not preserved: %#v", cr.Spec.Wandb.AdditionalHostnames)
	}
	if cr.Spec.Wandb.Hostname != "http://new.example" || cr.Spec.Wandb.Version != "1.2.3" {
		t.Fatalf("wandb fields not patched: %#v", cr.Spec.Wandb)
	}
	if cr.Spec.Wandb.ManifestRepository != "" {
		t.Fatalf("manifest repository should be cleared for published source: %q", cr.Spec.Wandb.ManifestRepository)
	}
	if cr.Spec.Size != v2.SizeMicro || cr.Spec.RetentionPolicy.OnDelete != v2.PurgeOnDelete {
		t.Fatalf("size/retention not patched: %q %q", cr.Spec.Size, cr.Spec.RetentionPolicy.OnDelete)
	}
	if cr.Spec.Networking.Mode != v2.NetworkingModeGatewayAPI || cr.Spec.Networking.Ingress != nil {
		t.Fatalf("networking not patched to gateway: %#v", cr.Spec.Networking)
	}
}

func TestBuildCRUnreadableLicenseFileReturnsError(t *testing.T) {
	_, err := BuildCR(Options{LicenseFile: filepath.Join(t.TempDir(), "missing-license")})
	if err == nil {
		t.Fatal("expected license file error")
	}
}

func TestBuildCRHTTPSUsesGeneratedCAIssuer(t *testing.T) {
	cr, err := BuildCR(Options{Hostname: "https://wandb.example"})
	if err != nil {
		t.Fatal(err)
	}

	if cr.Spec.Networking.TLS == nil || cr.Spec.Networking.TLS.CertManager == nil {
		t.Fatalf("TLS config was not set")
	}
	if cr.Spec.Networking.TLS.SecretName != "wandb-tls-secret" {
		t.Fatalf("secret name = %q", cr.Spec.Networking.TLS.SecretName)
	}
	if cr.Spec.Networking.TLS.CertManager.Issuer != "wandb-ca-issuer" {
		t.Fatalf("issuer = %q", cr.Spec.Networking.TLS.CertManager.Issuer)
	}
}

func TestBuildCRHTTPSUsesExplicitIssuer(t *testing.T) {
	cr, err := BuildCR(Options{
		Name:        "custom",
		Hostname:    "https://wandb.example",
		CreateCA:    false,
		CreateCASet: true,
		IssuerName:  "existing-issuer",
	})
	if err != nil {
		t.Fatal(err)
	}

	if cr.Spec.Networking.TLS == nil || cr.Spec.Networking.TLS.CertManager == nil {
		t.Fatalf("TLS config was not set")
	}
	if cr.Spec.Networking.TLS.SecretName != "custom-tls-secret" {
		t.Fatalf("secret name = %q", cr.Spec.Networking.TLS.SecretName)
	}
	if cr.Spec.Networking.TLS.CertManager.Issuer != "existing-issuer" {
		t.Fatalf("issuer = %q", cr.Spec.Networking.TLS.CertManager.Issuer)
	}
}

func TestRunWritesStableYAML(t *testing.T) {
	out := filepath.Join(t.TempDir(), "generated", "wandb.yaml")
	if err := Run(Options{OutPath: out}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	var cr v2.WeightsAndBiases
	if err := yaml.Unmarshal(data, &cr); err != nil {
		t.Fatal(err)
	}
	if cr.Name != defaultName || cr.Spec.Networking.Mode != v2.NetworkingModeGatewayAPI {
		t.Fatalf("unexpected generated CR: %s %#v", cr.Name, cr.Spec.Networking)
	}
	rendered := string(data)
	if strings.Contains(rendered, "\nstatus:") || strings.Contains(rendered, "\noidc:") || strings.Contains(rendered, "manifestRepository") {
		t.Fatalf("generated CR contains empty runtime/defaulted fields:\n%s", rendered)
	}
}
