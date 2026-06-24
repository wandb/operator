package bufstream

const (
	KafkaModuleName = "kafka"

	// BufstreamImage is the Bufstream broker image. Brokers are stateless; all
	// data lives in object storage and metadata in etcd. Buf publishes images to
	// this public OCI registry (Docker Hub mirrors are no longer maintained).
	BufstreamImage = "us-docker.pkg.dev/buf-images-1/buf/images/bufstream:0.4.15"

	// EtcdImage is the metadata store image. Uses the official upstream etcd
	// image, which is configured via native ETCD_* environment variables.
	EtcdImage = "quay.io/coreos/etcd:v3.5.31"

	// BucketEnsureImage runs a one-shot init container that creates the
	// object-store bucket Bufstream expects, since Bufstream does not create it
	// itself and reads from it on startup.
	BucketEnsureImage = "amazon/aws-cli:2.35.10"

	// Kafka-compatible listener exposed by Bufstream.
	KafkaListenerPort = 9092
	// Admin RPC interface.
	AdminPort = 9089
	// Debug interface that also serves Prometheus metrics.
	DebugPort = 9090

	// etcd client port.
	EtcdClientPort = 2379
	// etcd peer port.
	EtcdPeerPort = 2380

	// EtcdReplicas is the size of the etcd cluster. etcd requires an odd member
	// count for quorum; three tolerates a single member failure, which is the
	// minimum viable topology for production. It is intentionally not
	// customer-configurable.
	EtcdReplicas = 3

	// BufstreamReplicas is the default number of stateless Bufstream brokers.
	// Two brokers tolerate a single broker (or node) failure without an outage;
	// because all durable state lives in object storage and etcd, brokers can be
	// scaled freely. Used when no explicit replica count is configured.
	BufstreamReplicas = 2

	// EtcdClusterToken namespaces the etcd cluster's bootstrap so that distinct
	// W&B installs cannot accidentally cross-join.
	EtcdClusterToken = "wandb-etcd"

	// Mount point for the rendered bufstream.yaml.
	ConfigMountPath = "/etc/bufstream"
	ConfigFileName  = "bufstream.yaml"

	// etcd data directory and PVC template name.
	EtcdDataDir        = "/etcd-data"
	EtcdDataVolumeName = "data"

	// Env vars used to inject object-store credentials into the broker. Names are
	// provider-neutral because only one provider is active per deployment and the
	// Bufstream config simply references whichever env var holds the value.
	EnvStorageAccessKeyID     = "BUFSTREAM_STORAGE_ACCESS_KEY_ID"
	EnvStorageSecretAccessKey = "BUFSTREAM_STORAGE_SECRET_ACCESS_KEY"
)

const (
	// Condition types surfaced on the Kafka infra status.
	EtcdApplicationType      = "EtcdApplication"
	BufstreamApplicationType = "BufstreamApplication"
	KafkaConnectionInfoType  = "KafkaConnectionInfo"
	KafkaReportedReadyType   = "KafkaReportedReady"
	ObjectStoreReadyType     = "ObjectStoreReady"
)

const (
	ApplicationResourceType = "Application"
	ConfigMapResourceType   = "ConfigMap"
	SecretResourceType      = "Secret"
	AppConnTypeName         = "KafkaAppConn"
)
