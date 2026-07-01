package bufstream

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// storageProvider identifies which object-store backend Bufstream should use for
// its data. It is derived from the object-store connection string's scheme so
// the same code path serves every provider the platform supports.
type storageProvider string

const (
	providerS3    storageProvider = "s3"
	providerGCS   storageProvider = "gcs"
	providerAzure storageProvider = "azure"
)

// storageConnInfo holds the resolved object-store values needed to render the
// Bufstream data storage config. Provider selects the rendered backend; the
// remaining fields are populated as relevant for that provider.
type storageConnInfo struct {
	Provider storageProvider
	// URI is the provider-native storage location, e.g. "s3://bucket",
	// "gs://bucket/prefix", or "https://acct.blob.core.windows.net/container".
	URI string
	// Bucket is the bare bucket/container name, used by the S3 bucket-ensure
	// init container.
	Bucket string
	// Endpoint overrides the S3 API endpoint for S3-compatible providers
	// (SeaweedFS, MinIO, …). Empty for AWS S3, GCS, and Azure.
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
	// ForcePathStyle is required by most non-AWS S3-compatible providers.
	ForcePathStyle bool
}

// hasStaticCredentials reports whether explicit access/secret keys were provided.
// When false (e.g. AWS IAM roles or GCS workload identity) credentials are
// sourced from the pod's ambient identity instead of an operator-managed secret.
func (s storageConnInfo) hasStaticCredentials() bool {
	return s.AccessKey != "" && s.SecretKey != ""
}

type dataSource struct {
	EnvVar string `yaml:"env_var,omitempty"`
}

type bufstreamListener struct {
	Name             string `yaml:"name"`
	ListenAddress    string `yaml:"listen_address"`
	AdvertiseAddress string `yaml:"advertise_address,omitempty"`
}

type bufstreamKafka struct {
	Listeners []bufstreamListener `yaml:"listeners"`
}

type bufstreamS3 struct {
	URI             string      `yaml:"uri"`
	Region          string      `yaml:"region,omitempty"`
	Endpoint        string      `yaml:"endpoint,omitempty"`
	AccessKeyID     *dataSource `yaml:"access_key_id,omitempty"`
	SecretAccessKey *dataSource `yaml:"secret_access_key,omitempty"`
	ForcePathStyle  bool        `yaml:"force_path_style"`
}

type bufstreamAzure struct {
	URI             string      `yaml:"uri"`
	AccessKeyID     *dataSource `yaml:"access_key_id,omitempty"`
	SecretAccessKey *dataSource `yaml:"secret_access_key,omitempty"`
}

type bufstreamData struct {
	S3    *bufstreamS3    `yaml:"s3,omitempty"`
	GCS   string          `yaml:"gcs,omitempty"`
	Azure *bufstreamAzure `yaml:"azure,omitempty"`
}

type bufstreamEtcd struct {
	Addresses []string `yaml:"addresses"`
}

type bufstreamMetadata struct {
	Etcd bufstreamEtcd `yaml:"etcd"`
}

type bufstreamListenAddr struct {
	ListenAddress string `yaml:"listen_address"`
}

type bufstreamConfig struct {
	Version  string              `yaml:"version"`
	Cluster  string              `yaml:"cluster"`
	Kafka    bufstreamKafka      `yaml:"kafka"`
	Data     bufstreamData       `yaml:"data"`
	Metadata bufstreamMetadata   `yaml:"metadata"`
	Debug    bufstreamListenAddr `yaml:"debug"`
	Admin    bufstreamListenAddr `yaml:"admin"`
}

// renderBufstreamConfig produces the bufstream.yaml contents for a broker that
// stores data in the configured object-store provider and metadata in the given
// etcd cluster endpoints.
func renderBufstreamConfig(clusterName, advertiseHost string, etcdAddresses []string, storage storageConnInfo) (string, error) {
	// Isolate Bufstream's objects under a dedicated key prefix (the cluster name)
	// so they never collide with W&B artifact data, which shares the same bucket.
	// storage is passed by value, so this only affects the rendered config.
	storage.URI = strings.TrimSuffix(storage.URI, "/") + "/" + clusterName

	data, err := renderData(storage)
	if err != nil {
		return "", err
	}

	cfg := bufstreamConfig{
		Version: "v1beta1",
		Cluster: clusterName,
		Kafka: bufstreamKafka{
			Listeners: []bufstreamListener{
				{
					Name:             "external",
					ListenAddress:    fmt.Sprintf("0.0.0.0:%d", KafkaListenerPort),
					AdvertiseAddress: fmt.Sprintf("%s:%d", advertiseHost, KafkaListenerPort),
				},
			},
		},
		Data: data,
		Metadata: bufstreamMetadata{
			Etcd: bufstreamEtcd{Addresses: etcdAddresses},
		},
		Debug: bufstreamListenAddr{ListenAddress: fmt.Sprintf("0.0.0.0:%d", DebugPort)},
		Admin: bufstreamListenAddr{ListenAddress: fmt.Sprintf("0.0.0.0:%d", AdminPort)},
	}

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal bufstream config: %w", err)
	}
	return string(out), nil
}

// renderData maps the resolved object-store connection onto Bufstream's
// provider-specific data storage config.
func renderData(storage storageConnInfo) (bufstreamData, error) {
	switch storage.Provider {
	case providerS3:
		return bufstreamData{S3: renderS3Storage(storage)}, nil
	case providerGCS:
		// GCS authenticates via workload identity / Application Default
		// Credentials, so only the bucket URI is configured.
		return bufstreamData{GCS: storage.URI}, nil
	case providerAzure:
		return bufstreamData{Azure: renderAzureStorage(storage)}, nil
	default:
		return bufstreamData{}, fmt.Errorf("unsupported object-store provider %q", storage.Provider)
	}
}

func renderS3Storage(storage storageConnInfo) *bufstreamS3 {
	region := storage.Region
	if region == "" {
		region = "us-east-1"
	}
	s3 := &bufstreamS3{
		URI:            storage.URI,
		Region:         region,
		Endpoint:       storage.Endpoint,
		ForcePathStyle: storage.ForcePathStyle,
	}
	if storage.hasStaticCredentials() {
		s3.AccessKeyID = &dataSource{EnvVar: EnvStorageAccessKeyID}
		s3.SecretAccessKey = &dataSource{EnvVar: EnvStorageSecretAccessKey}
	}
	return s3
}

func renderAzureStorage(storage storageConnInfo) *bufstreamAzure {
	az := &bufstreamAzure{URI: storage.URI}
	if storage.hasStaticCredentials() {
		az.AccessKeyID = &dataSource{EnvVar: EnvStorageAccessKeyID}
		az.SecretAccessKey = &dataSource{EnvVar: EnvStorageSecretAccessKey}
	}
	return az
}
