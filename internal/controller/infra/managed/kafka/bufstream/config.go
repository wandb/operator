package bufstream

import (
	"fmt"

	"gopkg.in/yaml.v3"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/objectstore"
)

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
func renderBufstreamConfig(clusterName, advertiseHost string, etcdAddresses []string, storage objectstore.ConnInfo) (string, error) {
	// Isolate Bufstream's objects under a dedicated key prefix (the cluster name)
	// so they never collide with W&B artifact data, which shares the same bucket.
	// storage is passed by value, so this only affects the rendered config.
	uri := storage.ProviderURI()

	if storage.Path != "" {
		uri = fmt.Sprintf("%s/%s", uri, storage.Path)
	}

	uri = fmt.Sprintf("%s/%s", uri, clusterName)

	data, err := renderData(storage, uri)
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
func renderData(storage objectstore.ConnInfo, uri string) (bufstreamData, error) {
	switch storage.Provider {
	case apiv2.ObjectStoreProviderS3:
		return bufstreamData{S3: renderS3Storage(storage, uri)}, nil
	case apiv2.ObjectStoreProviderGCS:
		// GCS authenticates via workload identity / ADC, so only the bucket URI is configured.
		return bufstreamData{GCS: uri}, nil
	case apiv2.ObjectStoreProviderAzure:
		return bufstreamData{Azure: renderAzureStorage(storage, uri)}, nil
	default:
		return bufstreamData{}, fmt.Errorf("unsupported object-store provider %q", storage.Provider)
	}
}

func renderS3Storage(storage objectstore.ConnInfo, uri string) *bufstreamS3 {
	region := storage.Region
	if region == "" {
		region = objectstore.DefaultRegion
	}

	s3 := &bufstreamS3{
		URI:            uri,
		Region:         region,
		Endpoint:       storage.EndpointURL(),
		ForcePathStyle: storage.ForcePathStyle,
	}
	if storage.HasStaticCredentials() {
		s3.AccessKeyID = &dataSource{EnvVar: EnvStorageAccessKeyID}
		s3.SecretAccessKey = &dataSource{EnvVar: EnvStorageSecretAccessKey}
	}
	return s3
}

func renderAzureStorage(storage objectstore.ConnInfo, uri string) *bufstreamAzure {
	az := &bufstreamAzure{URI: uri}
	if storage.HasStaticCredentials() {
		az.AccessKeyID = &dataSource{EnvVar: EnvStorageAccessKeyID}
		az.SecretAccessKey = &dataSource{EnvVar: EnvStorageSecretAccessKey}
	}
	return az
}
