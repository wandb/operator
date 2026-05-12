package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	defaultOutPath          = "hack/testing-manifests/wandb/.generated/tilt-wandb-cr.yaml"
	defaultName             = "wandb"
	defaultNamespace        = "wandb"
	defaultHostname         = "http://localhost:8080"
	defaultIngressHostname  = "http://wandb.localhost:8080"
	localManifestRepository = "file:///server-manifest"
	defaultManifestSource   = "published"
	defaultVersion          = "0.80.0"
	defaultSize             = v2.SizeDev
	defaultRetentionPolicy  = v2.DetachOnDelete
)

type Options struct {
	OutPath           string
	CRFile            string
	Name              string
	Namespace         string
	Hostname          string
	Version           string
	Size              string
	RetentionPolicy   string
	LicenseFile       string
	ManifestSource    string
	ObservabilityMode string
	NetworkMode       string
	GatewayClass      string
	IngressClass      string
	CreateCA          bool
	CreateCASet       bool
	IssuerName        string
}

func main() {
	opts := Options{}
	flag.StringVar(&opts.OutPath, "out", defaultOutPath, "Path to write the generated WeightsAndBiases CR YAML")
	flag.StringVar(&opts.CRFile, "cr-file", "", "Optional base WeightsAndBiases CR YAML to patch")
	flag.StringVar(&opts.Name, "name", defaultName, "WeightsAndBiases resource name")
	flag.StringVar(&opts.Namespace, "namespace", defaultNamespace, "WeightsAndBiases resource namespace")
	flag.StringVar(&opts.Hostname, "hostname", defaultHostname, "W&B hostname")
	flag.StringVar(&opts.Version, "version", defaultVersion, "W&B version")
	flag.StringVar(&opts.Size, "size", string(defaultSize), "W&B size")
	flag.StringVar(&opts.RetentionPolicy, "retention-policy", string(defaultRetentionPolicy), "Retention policy on delete")
	flag.StringVar(&opts.LicenseFile, "license-file", "", "Path to W&B license file")
	flag.StringVar(&opts.ManifestSource, "manifest-source", defaultManifestSource, "Server manifest source: published or local")
	flag.StringVar(&opts.ObservabilityMode, "observability-mode", "off", "Observability mode: off, full, or forward")
	flag.StringVar(&opts.NetworkMode, "network-mode", "gateway", "Networking mode: gateway or ingress")
	flag.StringVar(&opts.GatewayClass, "gateway-class", "nginx", "GatewayClass name for gateway mode")
	flag.StringVar(&opts.IngressClass, "ingress-class", "nginx", "IngressClass name for ingress mode")
	flag.BoolVar(&opts.CreateCA, "create-ca", true, "Use the generated W&B CA issuer for HTTPS hostnames")
	flag.StringVar(&opts.IssuerName, "issuer-name", "", "Existing cert-manager issuer for HTTPS hostnames")
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "create-ca" {
			opts.CreateCASet = true
		}
	})

	if err := Run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Run(opts Options) error {
	cr, err := BuildCR(opts)
	if err != nil {
		return err
	}

	data, err := marshalCRYAML(cr)
	if err != nil {
		return fmt.Errorf("marshal CR YAML: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(opts.OutPath), 0o755); err != nil {
		return fmt.Errorf("create generated directory: %w", err)
	}
	if err := os.WriteFile(opts.OutPath, data, 0o644); err != nil {
		return fmt.Errorf("write generated CR: %w", err)
	}
	return nil
}

func marshalCRYAML(cr *v2.WeightsAndBiases) ([]byte, error) {
	data, err := json.Marshal(cr)
	if err != nil {
		return nil, err
	}

	obj := map[string]interface{}{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	delete(obj, "status")

	pruned, _ := pruneEmpty(obj)
	return yaml.Marshal(pruned)
}

func pruneEmpty(value interface{}) (interface{}, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case string:
		return typed, typed != ""
	case []interface{}:
		items := []interface{}{}
		for _, item := range typed {
			if pruned, keep := pruneEmpty(item); keep {
				items = append(items, pruned)
			}
		}
		return items, len(items) > 0
	case map[string]interface{}:
		for key, item := range typed {
			if pruned, keep := pruneEmpty(item); keep {
				typed[key] = pruned
			} else {
				delete(typed, key)
			}
		}
		return typed, len(typed) > 0
	default:
		return typed, true
	}
}

func BuildCR(opts Options) (*v2.WeightsAndBiases, error) {
	applyDefaults(&opts)

	cr, err := baseCR(opts.CRFile)
	if err != nil {
		return nil, err
	}

	ensureTypeMeta(cr)
	patchMetadata(cr, opts)
	patchScalarSpec(cr, opts)
	if err := patchManifestRepository(cr, opts.ManifestSource); err != nil {
		return nil, err
	}
	if err := patchLicense(cr, opts.LicenseFile); err != nil {
		return nil, err
	}
	if err := patchNetworking(cr, opts); err != nil {
		return nil, err
	}
	if err := patchTelemetry(cr, opts.ObservabilityMode); err != nil {
		return nil, err
	}

	return cr, nil
}

func applyDefaults(opts *Options) {
	if opts.OutPath == "" {
		opts.OutPath = defaultOutPath
	}
	if opts.Name == "" {
		opts.Name = defaultName
	}
	if opts.Namespace == "" {
		opts.Namespace = defaultNamespace
	}
	if opts.Hostname == "" {
		opts.Hostname = defaultHostname
	}
	if opts.Version == "" {
		opts.Version = defaultVersion
	}
	if opts.Size == "" {
		opts.Size = string(defaultSize)
	}
	if opts.RetentionPolicy == "" {
		opts.RetentionPolicy = string(defaultRetentionPolicy)
	}
	if opts.ManifestSource == "" {
		opts.ManifestSource = defaultManifestSource
	}
	if opts.ObservabilityMode == "" {
		opts.ObservabilityMode = "off"
	}
	if opts.NetworkMode == "" {
		opts.NetworkMode = "gateway"
	}
	if opts.GatewayClass == "" {
		opts.GatewayClass = "nginx"
	}
	if opts.IngressClass == "" {
		opts.IngressClass = "nginx"
	}
	if !opts.CreateCASet {
		opts.CreateCA = true
	}
}

func baseCR(crFile string) (*v2.WeightsAndBiases, error) {
	if crFile != "" {
		data, err := os.ReadFile(crFile)
		if err != nil {
			return nil, fmt.Errorf("read cr-file: %w", err)
		}

		cr := &v2.WeightsAndBiases{}
		if err := yaml.Unmarshal(data, cr); err != nil {
			return nil, fmt.Errorf("parse cr-file: %w", err)
		}
		return cr, nil
	}

	return &v2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps.wandb.com/v2",
			Kind:       "WeightsAndBiases",
		},
		Spec: v2.WeightsAndBiasesSpec{
			Wandb: v2.WandbAppSpec{
				Features:            map[string]bool{"proxy": true},
				InternalServiceAuth: v2.InternalServiceAuth{Enabled: boolPtr(false)},
			},
			MySQL:       v2.MySQLSpec{ManagedMysql: &v2.ManagedMysqlSpec{}},
			Redis:       v2.RedisSpec{ManagedRedis: &v2.ManagedRedisSpec{}},
			Kafka:       v2.KafkaSpec{ManagedKafka: &v2.ManagedKafkaSpec{}},
			ObjectStore: v2.ObjectStoreSpec{ManagedObjectStore: &v2.ManagedObjectStoreSpec{}},
			ClickHouse:  v2.ClickHouseSpec{ManagedClickHouse: &v2.ManagedClickHouseSpec{}},
		},
	}, nil
}

func ensureTypeMeta(cr *v2.WeightsAndBiases) {
	if cr.APIVersion == "" {
		cr.APIVersion = "apps.wandb.com/v2"
	}
	if cr.Kind == "" {
		cr.Kind = "WeightsAndBiases"
	}
}

func patchMetadata(cr *v2.WeightsAndBiases, opts Options) {
	cr.Name = opts.Name
	cr.Namespace = opts.Namespace
	if cr.Labels == nil {
		cr.Labels = map[string]string{}
	}
	cr.Labels["app.kubernetes.io/managed-by"] = "tilt"
}

func patchScalarSpec(cr *v2.WeightsAndBiases, opts Options) {
	cr.Spec.Wandb.Hostname = effectiveHostname(opts)
	cr.Spec.Wandb.Version = opts.Version
	if cr.Spec.Wandb.Features == nil {
		cr.Spec.Wandb.Features = map[string]bool{}
	}
	cr.Spec.Wandb.Features["proxy"] = true
	cr.Spec.Wandb.InternalServiceAuth = v2.InternalServiceAuth{Enabled: boolPtr(false)}
	cr.Spec.Size = v2.Size(opts.Size)
	cr.Spec.RetentionPolicy.OnDelete = v2.OnDeletePolicy(opts.RetentionPolicy)
}

func patchManifestRepository(cr *v2.WeightsAndBiases, manifestSource string) error {
	switch normalizeManifestSource(manifestSource) {
	case "published":
		cr.Spec.Wandb.ManifestRepository = ""
	case "local":
		cr.Spec.Wandb.ManifestRepository = localManifestRepository
	default:
		return fmt.Errorf("manifest-source must be one of: published, local")
	}
	return nil
}

func patchLicense(cr *v2.WeightsAndBiases, licenseFile string) error {
	if licenseFile == "" {
		return nil
	}

	data, err := os.ReadFile(licenseFile)
	if err != nil {
		return fmt.Errorf("read license-file: %w", err)
	}
	cr.Spec.Wandb.License = strings.TrimSpace(string(data))
	return nil
}

func patchNetworking(cr *v2.WeightsAndBiases, opts Options) error {
	mode := normalizeNetworkMode(opts.NetworkMode)
	switch mode {
	case "gateway":
		if opts.GatewayClass == "" {
			return fmt.Errorf("gateway network mode requires gateway-class")
		}
		cr.Spec.Networking.Mode = v2.NetworkingModeGatewayAPI
		cr.Spec.Networking.Ingress = nil
		cr.Spec.Networking.GatewayAPI = &v2.GatewayAPIConfig{
			Gateway: v2.GatewayConfig{
				Managed:          true,
				GatewayClassName: stringPtr(opts.GatewayClass),
			},
		}
	case "ingress":
		if opts.IngressClass == "" {
			return fmt.Errorf("ingress network mode requires ingress-class")
		}
		cr.Spec.Networking.Mode = v2.NetworkingModeIngress
		cr.Spec.Networking.GatewayAPI = nil
		cr.Spec.Networking.Ingress = &v2.IngressConfig{
			IngressClassName: stringPtr(opts.IngressClass),
		}
	default:
		return fmt.Errorf("network-mode must be one of: gateway, ingress")
	}

	if strings.HasPrefix(cr.Spec.Wandb.Hostname, "https://") {
		if opts.CreateCA {
			cr.Spec.Networking.TLS = &v2.TLSConfig{
				SecretName: opts.Name + "-tls-secret",
				CertManager: &v2.CertManagerConfig{
					Issuer: opts.Name + "-ca-issuer",
				},
			}
		} else if opts.IssuerName != "" {
			cr.Spec.Networking.TLS = &v2.TLSConfig{
				SecretName: opts.Name + "-tls-secret",
				CertManager: &v2.CertManagerConfig{
					Issuer: opts.IssuerName,
				},
			}
		} else {
			return fmt.Errorf("https hostname requires create-ca=true or issuer-name")
		}
	} else {
		cr.Spec.Networking.TLS = nil
	}

	return nil
}

func patchTelemetry(cr *v2.WeightsAndBiases, observabilityMode string) error {
	mode := normalizeObservabilityMode(observabilityMode)
	var enabled bool
	switch mode {
	case "off":
		enabled = false
	case "full", "forward":
		enabled = true
	default:
		return fmt.Errorf("observability-mode must be one of: off, full, forward")
	}

	if cr.Spec.MySQL.ManagedMysql != nil {
		cr.Spec.MySQL.ManagedMysql.Telemetry.Enabled = enabled
	}
	if cr.Spec.Redis.ManagedRedis != nil {
		cr.Spec.Redis.ManagedRedis.Telemetry.Enabled = enabled
	}
	if cr.Spec.Kafka.ManagedKafka != nil {
		cr.Spec.Kafka.ManagedKafka.Telemetry.Enabled = enabled
	}
	if cr.Spec.ObjectStore.ManagedObjectStore != nil {
		cr.Spec.ObjectStore.ManagedObjectStore.Telemetry.Enabled = enabled
	}
	if cr.Spec.ClickHouse.ManagedClickHouse != nil {
		cr.Spec.ClickHouse.ManagedClickHouse.Telemetry.Enabled = enabled
	}
	return nil
}

func effectiveHostname(opts Options) string {
	if normalizeNetworkMode(opts.NetworkMode) == "ingress" && opts.Hostname == defaultHostname {
		return defaultIngressHostname
	}
	return opts.Hostname
}

func normalizeNetworkMode(mode string) string {
	switch strings.ToLower(mode) {
	case "gateway":
		return "gateway"
	case "ingress":
		return "ingress"
	default:
		return mode
	}
}

func normalizeObservabilityMode(mode string) string {
	if mode == "" {
		return "off"
	}
	return strings.ToLower(mode)
}

func normalizeManifestSource(source string) string {
	if source == "" {
		return defaultManifestSource
	}
	return strings.ToLower(source)
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
