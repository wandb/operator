/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=wandb
//+kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
//+kubebuilder:printcolumn:name="MySQL",type=string,JSONPath=`.status.mysqlStatus.default.state`
//+kubebuilder:printcolumn:name="Redis",type=string,JSONPath=`.status.redisStatus.default.state`
//+kubebuilder:printcolumn:name="Kafka",type=string,JSONPath=`.status.kafkaStatus.state`
//+kubebuilder:printcolumn:name="ObjectStore",type=string,JSONPath=`.status.objectStoreStatus.default.state`
//+kubebuilder:printcolumn:name="ClickHouse",type=string,JSONPath=`.status.clickhouseStatus.default.state`

// WeightsAndBiases is the Schema for the weightsandbiases API.
type WeightsAndBiases struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WeightsAndBiasesSpec   `json:"spec,omitempty"`
	Status WeightsAndBiasesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WeightsAndBiasesList contains a list of WeightsAndBiases.
type WeightsAndBiasesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WeightsAndBiases `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WeightsAndBiases{}, &WeightsAndBiasesList{})
}

// DefaultInstanceName is the reserved map key identifying the fallback instance
// for each multi-instance infrastructure type (MySQL, Redis, ObjectStore,
// ClickHouse). When an application requests an instance that is not provisioned,
// the operator resolves to this instance instead.
const DefaultInstanceName = "default"

// ResolveInstance returns the entry for key, falling back to the
// DefaultInstanceName entry when key is empty or absent. The boolean reports
// whether a value was found.
func ResolveInstance[T any](m map[string]T, key string) (T, bool) {
	if key == "" {
		key = DefaultInstanceName
	}
	if v, ok := m[key]; ok {
		return v, true
	}
	if v, ok := m[DefaultInstanceName]; ok {
		return v, true
	}
	var zero T
	return zero, false
}

type Size string

const (
	SizeDev     Size = "dev"
	SizeMicro   Size = "micro"
	SizeSmall   Size = "small"
	SizeMedium  Size = "medium"
	SizeLarge   Size = "large"
	SizeXLarge  Size = "xlarge"
	SizeXXLarge Size = "xxlarge"
)

type OnDeletePolicy string

const (
	// DetachOnDelete removes ownership of infrastructure CRs so they survive WandB CR deletion
	DetachOnDelete OnDeletePolicy = "detach"
	// PurgeOnDelete will delete all associated resources upon deletion
	PurgeOnDelete OnDeletePolicy = "purge"
)

type RetentionPolicy struct {
	// +kubebuilder:default="detach"
	OnDelete OnDeletePolicy `json:"onDelete" default:"detach"`
}

// WeightsAndBiasesSpec defines the desired state of WeightsAndBiases.
type WeightsAndBiasesSpec struct {
	// Size is akin to high-level environment info
	// +kubebuilder:validation:Enum=dev;micro;small;medium;large;xlarge;xxlarge
	Size Size `json:"size,omitempty"`
	// RequireLimits By default, only resource requests are set for deployments, set to true to also set resource limits
	RequireLimits bool `json:"requireLimits,omitempty"`

	RetentionPolicy RetentionPolicy `json:"retentionPolicy"`

	// Global holds settings shared across all managed components.
	// +optional
	Global GlobalSpec `json:"global,omitempty"`

	Wandb WandbAppSpec `json:"wandb,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`

	// MySQL, Redis, ObjectStore and ClickHouse are keyed by instance name. The
	// reserved DefaultInstanceName key identifies the fallback instance used when
	// an application requests an instance that is not provisioned.
	MySQL       map[string]MySQLSpec       `json:"mysql,omitempty"`
	Redis       map[string]RedisSpec       `json:"redis,omitempty"`
	Kafka       KafkaSpec                  `json:"kafka,omitempty"`
	ObjectStore map[string]ObjectStoreSpec `json:"objectStore,omitempty"`
	ClickHouse  map[string]ClickHouseSpec  `json:"clickhouse,omitempty"`

	// Networking configures how the W&B application is exposed externally.
	// +optional
	Networking NetworkingSpec `json:"networking,omitempty"`
}

// GlobalSpec holds settings shared across every managed component.
type GlobalSpec struct {
	// ImageRegistry, when set, retargets the container images to this registry.
	// Intended for air-gapped installs whose nodes cannot reach public registries; pair it
	// with a registry pre-populated by `wsm registry mirror`.
	ImageRegistry string `json:"imageRegistry,omitempty"`

	// CustomCACerts contains PEM-encoded CA certificates that should be trusted
	// by W&B application workloads.
	// +optional
	CustomCACerts []string `json:"customCACerts,omitempty"`

	// CACertsConfigMap references a ConfigMap in the W&B namespace whose keys
	// contain CA certificates. Keys should use a .crt suffix so standard CA
	// update tooling can discover them.
	// +optional
	CACertsConfigMap string `json:"caCertsConfigMap,omitempty"`

	// Proxy configures the forward-proxy egress settings injected into the
	// application workloads (app Deployments, their init containers, and
	// migration Jobs). The operator emits HTTP_PROXY/HTTPS_PROXY/NO_PROXY and
	// their lowercase variants; NO_PROXY is always the operator-computed
	// in-cluster exclusions merged over the user-supplied noProxy entries, so
	// in-cluster datastore/service traffic never hairpins through the proxy.
	// Pair with CustomCACerts for a TLS-intercepting proxy.
	// +optional
	Proxy *ProxySpec `json:"proxy,omitempty"`
}

// ProxySpec is the forward-proxy configuration under spec.global.proxy.
type ProxySpec struct {
	// HTTPProxy is the proxy URL for plain HTTP egress (HTTP_PROXY/http_proxy).
	// +optional
	HTTPProxy *ProxyValue `json:"httpProxy,omitempty"`

	// HTTPSProxy is the proxy URL for HTTPS egress (HTTPS_PROXY/https_proxy).
	// +optional
	HTTPSProxy *ProxyValue `json:"httpsProxy,omitempty"`

	// NoProxy holds EXTRA no-proxy entries appended to the operator-computed
	// in-cluster exclusions. Use it for external endpoints (e.g. a BYOB object
	// store) that must bypass the proxy. Entries must be comma-free; the
	// operator owns the join.
	// +optional
	NoProxy []string `json:"noProxy,omitempty"`
}

// ProxyValue is a value-or-secret union mirroring corev1.EnvVar semantics:
// exactly one of Value or ValueFrom must be set. Credential-bearing proxy URLs
// (http://user:pass@host:port) MUST use ValueFrom; the webhook rejects userinfo
// in a literal Value so credentials never land in the CR / etcd / kubectl output.
type ProxyValue struct {
	// Value is a literal proxy URL. Must not contain userinfo (credentials).
	// +optional
	Value string `json:"value,omitempty"`

	// ValueFrom sources the proxy URL from a Secret key (may embed credentials).
	// +optional
	ValueFrom *ProxyValueSource `json:"valueFrom,omitempty"`
}

// ProxyValueSource mirrors corev1.EnvVarSource (the secret case): the proxy URL
// is read from a Secret key.
type ProxyValueSource struct {
	// SecretKeyRef selects a key of a Secret in the W&B namespace.
	// +optional
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

type NetworkingMode string

const (
	NetworkingModeNone       NetworkingMode = ""
	NetworkingModeIngress    NetworkingMode = "ingress"
	NetworkingModeGatewayAPI NetworkingMode = "gateway"
)

type NetworkingSpec struct {
	// Mode selects the networking strategy: "Ingress" or "GatewayAPI".
	// Empty/unset means no operator-managed ingress (preserves current NodePort behavior).
	// +kubebuilder:validation:Enum="";ingress;gateway
	Mode NetworkingMode `json:"mode,omitempty"`

	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`

	// +optional
	GatewayAPI *GatewayAPIConfig `json:"gatewayAPI,omitempty"`

	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`

	// Annotations applied to all generated Ingress or Gateway resources.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type IngressConfig struct {
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// Name overrides the generated Ingress resource name. When empty the
	// operator defaults to the CR name.
	// +optional
	Name string `json:"name,omitempty"`
}

type GatewayAPIConfig struct {
	Gateway GatewayConfig `json:"gateway"`

	// ListenerName selects which listener on the Gateway to attach HTTPRoutes to.
	// +optional
	ListenerName *string `json:"listenerName,omitempty"`
}

type GatewayConfig struct {
	// Managed controls whether the operator creates and manages the Gateway resource.
	// When false (default), gatewayRef must reference an existing Gateway.
	// +kubebuilder:default=false
	Managed bool `json:"managed,omitempty"`

	// +optional
	GatewayRef *GatewayReference `json:"gatewayRef,omitempty"`

	// GatewayClassName is required when managed=true.
	// +optional
	GatewayClassName *string `json:"gatewayClassName,omitempty"`

	// Listeners defines the listeners on a managed Gateway.
	// If empty and managed=true, a default HTTPS listener is created from
	// spec.wandb.hostname and spec.networking.tls.
	// +optional
	Listeners []GatewayListener `json:"listeners,omitempty"`

	// Annotations passed to the managed Gateway resource.
	// +optional
	InfrastructureAnnotations map[string]string `json:"infrastructureAnnotations,omitempty"`
}

type GatewayReference struct {
	Name string `json:"name"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type GatewayListener struct {
	Name     string `json:"name"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
	// +optional
	Hostname *string `json:"hostname,omitempty"`
	// +optional
	TLS *ListenerTLSConfig `json:"tls,omitempty"`
}

type ListenerTLSConfig struct {
	// +optional
	Mode *string `json:"mode,omitempty"`
	// +optional
	CertificateRef *SecretRef `json:"certificateRef,omitempty"`
}

type SecretRef struct {
	Name string `json:"name"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type TLSConfig struct {
	// +optional
	SecretName string `json:"secretName,omitempty"`
	// +optional
	CertManager *CertManagerConfig `json:"certManager,omitempty"`
}

type CertManagerConfig struct {
	// +optional
	ClusterIssuer string `json:"clusterIssuer,omitempty"`
	// +optional
	Issuer string `json:"issuer,omitempty"`
}

func (w *WeightsAndBiases) GetRetentionPolicy(spec ManagedInfraSpec) RetentionPolicy {
	if spec.RetentionPolicy != nil {
		return *spec.RetentionPolicy
	}
	return w.Spec.RetentionPolicy
}

func (w *WeightsAndBiases) GetAffinity(spec ManagedInfraSpec) *corev1.Affinity {
	if spec.Affinity != nil {
		return spec.Affinity
	}
	return w.Spec.Affinity
}

func (w *WeightsAndBiases) GetTolerations(spec ManagedInfraSpec) *[]corev1.Toleration {
	if spec.Tolerations != nil {
		return spec.Tolerations
	}
	return w.Spec.Tolerations
}

// ValidMysqlReplicaCount reports whether r is a count Moco accepts: a positive odd number.
func ValidMysqlReplicaCount(r int32) bool {
	return r > 0 && r%2 == 1
}

// WandbAppSpec defines the configuration for the Wandb application deployment.
type WandbAppSpec struct {
	Hostname            string              `json:"hostname"`
	License             string              `json:"license,omitempty"`
	ManifestRepository  string              `json:"manifestRepository,omitempty"`
	Version             string              `json:"version"`
	Features            map[string]bool     `json:"features"`
	InternalServiceAuth InternalServiceAuth `json:"internalServiceAuth,omitempty"`
	BucketProxy         bool                `json:"bucketProxy"`

	ServiceAccount ServiceAccountSpec `json:"serviceAccount,omitempty"`

	// Probes configures default health probes for W&B application workload
	// containers generated by the operator. The full Kubernetes Probe shape is
	// exposed so operators can tune timings and handlers for their environment.
	// Explicit probes on generated containers remain authoritative; these values
	// only fill missing probes or missing probe fields.
	// +optional
	Probes WandbProbeDefaults `json:"probes,omitempty"`

	// +optional
	AdditionalHostnames []string `json:"additionalHostnames,omitempty"`

	// +optional
	OIDC OidcSpec `json:"oidc,omitempty"`

	// LegacyOverrides holds env/resource overrides extracted from v1
	// spec.values, keyed by manifest application name plus the reserved
	// "global" key (env only, applied to every application). Unknown keys are
	// logged and ignored. Conversion-owned; prefer first-class fields over
	// hand-editing.
	// +optional
	LegacyOverrides map[string]LegacyOverrides `json:"legacyOverrides,omitempty"`
}

// LegacyOverridesGlobalKey is the reserved LegacyOverrides key whose env
// applies to every application and migration job.
const LegacyOverridesGlobalKey = "global"

// DefaultManifestRepository is used when spec.wandb.manifestRepository is
// unset — by the defaulting webhook and by v1 conversion (which runs first).
const DefaultManifestRepository = "oci://us-docker.pkg.dev/wandb-production/public/wandb/server-manifest"

// LegacyOverrides holds v1-derived overrides for one application (or "global").
type LegacyOverrides struct {
	// Env is applied last, replacing same-named manifest or injected vars.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources overlays sizing-derived resources per field; limits are still
	// gated by spec.requireLimits.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

type WandbProbeDefaults struct {
	// StartupProbe defines default startup probe values for generated W&B
	// application containers. If the handler is omitted, the operator derives
	// one from the container readiness probe, then liveness probe.
	// +optional
	StartupProbe *corev1.Probe `json:"startupProbe,omitempty"`

	// LivenessProbe defines default liveness probe values for generated W&B
	// application containers.
	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`

	// ReadinessProbe defines default readiness probe values for generated W&B
	// application containers.
	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`
}

type ServiceAccountSpec struct {
	// +kubebuilder:default=true
	Create *bool `json:"create"`
	// +kubebuilder:default="wandb"
	ServiceAccountName string            `json:"serviceAccountName,omitempty"`
	Annotations        map[string]string `json:"annotations,omitempty"`
}

type InternalServiceAuth struct {
	Enabled    *bool  `json:"enabled,omitempty"`
	OIDCIssuer string `json:"oidcIssuer,omitempty"`
}

// OidcSpec defines the structure for OpenID Connect (OIDC) configuration used in Wandb application deployments.
type OidcSpec struct {
	ClientId     corev1.SecretKeySelector `json:"clientId,omitempty"`
	ClientSecret corev1.SecretKeySelector `json:"clientSecret,omitempty"`
	IssuerUrl    corev1.SecretKeySelector `json:"issuerUrl,omitempty"`
	AuthMethod   corev1.SecretKeySelector `json:"authMethod,omitempty"`

	SessionLength string `json:"sessionLength,omitempty"`
}

type ManagedInfraSpec struct {
	RetentionPolicy *RetentionPolicy `json:"retentionPolicy,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`
}

// MySQLSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type MySQLSpec struct {
	ManagedMysql  *ManagedMysqlSpec `json:"managedMysql,omitempty"`
	ExternalMysql *MysqlConnection  `json:"externalMysql,omitempty"`
}

type ManagedMysqlSpec struct {
	ManagedInfraSpec `json:",inline"`

	StorageSize string      `json:"storageSize,omitempty"`
	Replicas    int32       `json:"replicas,omitempty"`
	Config      MySQLConfig `json:"config,omitempty"`
	Namespace   string      `json:"namespace,omitempty"`
	Name        string      `json:"name,omitempty"`
	Telemetry   Telemetry   `json:"telemetry,omitempty"`
}

type MysqlConnection struct {
	// required
	Host     corev1.SecretKeySelector `json:"host,omitempty"`
	Port     corev1.SecretKeySelector `json:"port,omitempty"`
	Database corev1.SecretKeySelector `json:"database,omitempty"`
	Username corev1.SecretKeySelector `json:"username,omitempty"`
	Password corev1.SecretKeySelector `json:"password,omitempty"`

	// optional
	Tls     corev1.SecretKeySelector `json:"tls,omitempty"`
	SslCa   corev1.SecretKeySelector `json:"sslCa,omitempty"`
	SslCert corev1.SecretKeySelector `json:"sslCert,omitempty"`
	SslKey  corev1.SecretKeySelector `json:"sslKey,omitempty"`

	// generated by operator
	URL corev1.SecretKeySelector `json:"url,omitempty"`
}

type MySQLConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// Telemetry defines telemetry configuration for infrastructure components
type Telemetry struct {
	// +kubebuilder:default=true
	Enabled bool `json:"enabled" default:"true"`
}

// RedisSpec defines the desired state of the Redis infrastructure component.
type RedisSpec struct {
	ManagedRedis  *ManagedRedisSpec `json:"managedRedis,omitempty"`
	ExternalRedis *RedisConnection  `json:"externalRedis,omitempty"`
}

type ManagedRedisSpec struct {
	ManagedInfraSpec `json:",inline"`

	StorageSize string            `json:"storageSize,omitempty"`
	Config      RedisConfig       `json:"config,omitempty"`
	Sentinel    RedisSentinelSpec `json:"sentinel,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Name        string            `json:"name,omitempty"`
	Telemetry   Telemetry         `json:"telemetry,omitempty"`
}

type RedisConnection struct {
	Host     corev1.SecretKeySelector `json:"host,omitempty"`
	Port     corev1.SecretKeySelector `json:"port,omitempty"`
	Password corev1.SecretKeySelector `json:"password,omitempty"`
	Tls      corev1.SecretKeySelector `json:"tls,omitempty"`
	SslCa    corev1.SecretKeySelector `json:"sslCa,omitempty"`

	URL corev1.SecretKeySelector `json:"url,omitempty"`
}

type RedisConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type RedisSentinelSpec struct {
	Enabled bool                `json:"enabled"`
	Config  RedisSentinelConfig `json:"config,omitempty"`
}

type RedisSentinelConfig struct {
	MasterName string                      `json:"masterName,omitempty"`
	Resources  corev1.ResourceRequirements `json:"resources,omitempty"`
}

// KafkaSpec defines the desired state of the Kafka infrastructure component.
// Kafka is managed-only (backed by Bufstream); there is no external Kafka option.
type KafkaSpec struct {
	ManagedKafka *ManagedKafkaSpec `json:"managedKafka,omitempty"`
}

type ManagedKafkaSpec struct {
	ManagedInfraSpec `json:",inline"`

	StorageSize      string      `json:"storageSize,omitempty"`
	Replicas         int32       `json:"replicas,omitempty"`
	Config           KafkaConfig `json:"config,omitempty"`
	Namespace        string      `json:"namespace,omitempty"`
	Name             string      `json:"name,omitempty"`
	Telemetry        Telemetry   `json:"telemetry,omitempty"`
	SkipDataRecovery bool        `json:"skipDataRecovery,omitempty"`
}

type KafkaConnection struct {
	Host           corev1.SecretKeySelector `json:"host,omitempty"`
	Port           corev1.SecretKeySelector `json:"port,omitempty"`
	BrokerEndpoint corev1.SecretKeySelector `json:"brokerEndpoint,omitempty"`
	ClusterID      corev1.SecretKeySelector `json:"clusterID,omitempty"`

	URL corev1.SecretKeySelector `json:"url,omitempty"`
}

type KafkaConfig struct {
	Resources         corev1.ResourceRequirements `json:"resources,omitempty"`
	ReplicationConfig KafkaReplicationConfig      `json:"replicationConfig,omitempty"`
}

type KafkaReplicationConfig struct {
	DefaultReplicationFactor int32 `json:"defaultReplicationFactor,omitempty"`
	MinInSyncReplicas        int32 `json:"minInSyncReplicas,omitempty"`
	OffsetsTopicRF           int32 `json:"offsetsTopicRF,omitempty"`
	TransactionStateRF       int32 `json:"transactionStateISR,omitempty"`
	TransactionStateISR      int32 `json:"transactionStateRF,omitempty"`
}

// ObjectStoreSpec defines the desired state of the object store infrastructure component.
type ObjectStoreSpec struct {
	ManagedObjectStore  *ManagedObjectStoreSpec `json:"managedObjectStore,omitempty"`
	ExternalObjectStore *ObjectStoreConnection  `json:"externalObjectStore,omitempty"`
}

type ManagedObjectStoreSpec struct {
	ManagedInfraSpec       `json:",inline"`
	SeaweedObjectStoreSpec SeaweedObjectStoreSpec `json:"SeaweedObjectStoreSpec,omitempty"`
	StorageSize            string                 `json:"storageSize,omitempty"`
	Replicas               int32                  `json:"replicas,omitempty"`
	Copies                 int32                  `json:"copies,omitempty"`
	Config                 ObjectStoreConfig      `json:"config,omitempty"`
	Namespace              string                 `json:"namespace,omitempty"`
	Name                   string                 `json:"name,omitempty"`
	Telemetry              Telemetry              `json:"telemetry,omitempty"`
}

type SeaweedObjectStoreSpec struct {
	TlsEnabled bool `json:"tlsEnabled,omitempty"`
	// FilerStorageSize sizes the filer's metadata index disk. It grows with the
	// number of objects, not their total size, so bump it for large object counts.
	// Defaults to 20Gi when unset.
	FilerStorageSize string `json:"filerStorageSize,omitempty"`
}

// ObjectStoreProvider selects the object store backend for an external object store.
type ObjectStoreProvider string

const (
	ObjectStoreProviderS3    ObjectStoreProvider = "s3"
	ObjectStoreProviderGCS   ObjectStoreProvider = "gcs"
	ObjectStoreProviderAzure ObjectStoreProvider = "azure"
)

type ObjectStoreConnection struct {
	// Provider selects the externalObjectStore backend (s3, gcs, or azure) from a secret key; defaults to s3 when absent.
	Provider corev1.SecretKeySelector `json:"provider,omitempty"`

	Endpoint  corev1.SecretKeySelector `json:"endpoint,omitempty"`
	Port      corev1.SecretKeySelector `json:"port,omitempty"`
	AccessKey corev1.SecretKeySelector `json:"accessKey,omitempty"`
	SecretKey corev1.SecretKeySelector `json:"secretKey,omitempty"`
	Bucket    corev1.SecretKeySelector `json:"bucket,omitempty"`
	// Path is an optional key prefix within the bucket under which W&B stores its data.
	Path           corev1.SecretKeySelector `json:"path,omitempty"`
	Region         corev1.SecretKeySelector `json:"region,omitempty"`
	TlsEnabled     corev1.SecretKeySelector `json:"tlsEnabled,omitempty"`
	ForcePathStyle corev1.SecretKeySelector `json:"forcePathStyle,omitempty"`
	URL            corev1.SecretKeySelector `json:"url,omitempty"`
}

type ObjectStoreConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	AccessKey string                      `json:"accessKey,omitempty"`

	// Deprecated: Use AccessKey instead. Kept for backward compatibility during migration.
	RootUser string `json:"rootUser,omitempty"`
	// Deprecated: No longer used. Kept to avoid schema validation failures on upgrade.
	MinioBrowserSetting string `json:"minioBrowserSetting,omitempty"`
}

// ClickHouseSpec defines the desired state of the ClickHouse infrastructure component.
type ClickHouseSpec struct {
	ManagedClickHouse  *ManagedClickHouseSpec `json:"managedClickhouse,omitempty"`
	ExternalClickHouse *ClickHouseConnection  `json:"externalClickhouse,omitempty"`
}

type ManagedClickHouseSpec struct {
	ManagedInfraSpec `json:",inline"`

	StorageSize string           `json:"storageSize,omitempty"`
	Replicas    int32            `json:"replicas,omitempty"`
	Version     string           `json:"version,omitempty"`
	Config      ClickHouseConfig `json:"config,omitempty"`
	Namespace   string           `json:"namespace,omitempty"`
	Name        string           `json:"name,omitempty"`
	Telemetry   Telemetry        `json:"telemetry,omitempty"`

	// ObjectStorage configures the S3-backed disk that holds ClickHouse table
	// data in the configured W&B object store (managed SeaweedFS or external
	// bucket). Managed ClickHouse always stores table data in object storage;
	// StorageSize sizes the local PV used only for metadata, system tables, and
	// the S3 read cache.
	ObjectStorage ClickHouseObjectStorageSpec `json:"objectStorage,omitempty"`

	// Keeper configures the ClickHouse Keeper ensemble that coordinates
	// ReplicatedMergeTree replication across ClickHouse replicas.
	Keeper ClickHouseKeeperSpec `json:"keeper,omitempty"`
}

// ClickHouseObjectStorageSpec configures object-store-backed storage for managed
// ClickHouse.
type ClickHouseObjectStorageSpec struct {
	// Prefix is the key prefix within the bucket under which ClickHouse stores
	// its data. Lets multiple consumers share a single bucket. Defaults to
	// "clickhouse/".
	Prefix string `json:"prefix,omitempty"`

	// Insecure connects to the object store over HTTP instead of HTTPS. It only
	// applies to external object stores that do not advertise a scheme; the
	// managed object store's scheme is taken from its connection. Defaults to
	// false (HTTPS).
	Insecure bool `json:"insecure,omitempty"`
}

// ClickHouseKeeperSpec configures the managed ClickHouse Keeper ensemble.
type ClickHouseKeeperSpec struct {
	// Replicas is the number of Keeper nodes. Use an odd number (1, 3, 5) so the
	// ensemble can form a quorum. Defaults to 3.
	Replicas int32 `json:"replicas,omitempty"`

	// StorageSize is the persistent volume size for each Keeper node's raft log
	// and snapshots. Keeper state is small; defaults to a modest value.
	StorageSize string `json:"storageSize,omitempty"`

	// Config holds resource requirements for the Keeper pods.
	Config ClickHouseConfig `json:"config,omitempty"`
}

type ClickHouseConnection struct {
	Host     corev1.SecretKeySelector `json:"host,omitempty"`
	TCPPort  corev1.SecretKeySelector `json:"tcpPort,omitempty"`
	HTTPPort corev1.SecretKeySelector `json:"httpPort,omitempty"`
	Database corev1.SecretKeySelector `json:"database,omitempty"`
	Username corev1.SecretKeySelector `json:"username,omitempty"`
	Password corev1.SecretKeySelector `json:"password,omitempty"`

	URL corev1.SecretKeySelector `json:"url,omitempty"`
}

type ClickHouseConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// WeightsAndBiasesStatus defines the observed state of WeightsAndBiases.
type WeightsAndBiasesStatus struct {
	Ready bool        `json:"ready"`
	Wandb WandbStatus `json:"wandb,omitempty"`
	// MySQLStatus, RedisStatus, ObjectStoreStatus and ClickHouseStatus are keyed
	// by instance name, mirroring the corresponding spec maps.
	MySQLStatus       map[string]MysqlInfraStatus       `json:"mysqlStatus,omitempty"`
	RedisStatus       map[string]RedisInfraStatus       `json:"redisStatus,omitempty"`
	KafkaStatus       KafkaInfraStatus                  `json:"kafkaStatus,omitempty"`
	ObjectStoreStatus map[string]ObjectStoreInfraStatus `json:"objectStoreStatus,omitempty"`
	ClickHouseStatus  map[string]ClickHouseInfraStatus  `json:"clickhouseStatus,omitempty"`
	TelemetryStatus   TelemetryInfraStatus              `json:"telemetryStatus,omitempty"`
	// GeneratedSecrets stores references to secrets generated by the operator
	// from the server manifest's generatedSecrets section. The key is the
	// logical secret name from the manifest, and the value is a SecretKeySelector
	// referencing the concrete Secret and key that holds the generated value.
	GeneratedSecrets   map[string]corev1.SecretKeySelector `json:"generatedSecrets,omitempty"`
	ObservedGeneration int64                               `json:"observedGeneration"`

	// +optional
	GatewayStatus *GatewayStatusSummary `json:"gatewayStatus,omitempty"`
	// +optional
	IngressStatus *IngressStatusSummary `json:"ingressStatus,omitempty"`
}

type GatewayStatusSummary struct {
	Name       string            `json:"name,omitempty"`
	Ready      bool              `json:"ready,omitempty"`
	Addresses  []string          `json:"addresses,omitempty"`
	GatewayRef *GatewayReference `json:"gatewayRef,omitempty"`
}

type IngressStatusSummary struct {
	Name                string                       `json:"name,omitempty"`
	LoadBalancerIngress []corev1.LoadBalancerIngress `json:"loadBalancerIngress,omitempty"`
}

type WandbStatus struct {
	Hostname string `json:"hostname"`

	// +kubebuilder:default:={}
	Applications map[string]ApplicationStatus `json:"applications,omitempty"`

	Migration WandbMigrationStatus `json:"migration,omitempty"`

	// MySQLInit tracks the per-instance database-initialization job, keyed by
	// managed MySQL instance name.
	// +kubebuilder:default:={}
	MySQLInit map[string]MigrationJobStatus `json:"mysqlInit,omitempty"`
}

type WandbMigrationStatus struct {
	Version            string                        `json:"version,omitempty"`
	LastSuccessVersion string                        `json:"lastSuccessVersion,omitempty"`
	Ready              bool                          `json:"ready,omitempty"`
	Reason             string                        `json:"reason,omitempty"`
	Jobs               map[string]MigrationJobStatus `json:"jobs,omitempty"`
}

type MigrationJobStatus struct {
	Name      string `json:"name,omitempty"`
	Succeeded bool   `json:"succeeded,omitempty"`
	Failed    bool   `json:"failed,omitempty"`
	Message   string `json:"message,omitempty"`
}

type WBInfraStatus struct {
	Ready      bool               `json:"ready"`
	State      string             `json:"state,omitempty" default:"Unknown"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type MysqlInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Connection    MysqlConnection `json:"connection,omitempty"`
}

type RedisInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Connection    RedisConnection `json:"connection,omitempty"`
}

type KafkaInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Connection    KafkaConnection `json:"connection,omitempty"`
}

type ObjectStoreInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Connection    ObjectStoreConnection `json:"connection,omitempty"`
}

type ClickHouseInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Connection    ClickHouseConnection `json:"connection,omitempty"`
}

type TelemetryInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Mode          string                    `json:"mode,omitempty"`
	Connection    TelemetryConnectionStatus `json:"connection,omitempty"`
}

type TelemetryConnectionStatus struct {
	ManagedNamespace      string `json:"managedNamespace,omitempty"`
	ConnectionSecret      string `json:"connectionSecret,omitempty"`
	Protocol              string `json:"protocol,omitempty"`
	MetricsExporter       string `json:"metricsExporter,omitempty"`
	LogsExporter          string `json:"logsExporter,omitempty"`
	TracesExporter        string `json:"tracesExporter,omitempty"`
	MetricsEndpoint       string `json:"metricsEndpoint,omitempty"`
	LogsEndpoint          string `json:"logsEndpoint,omitempty"`
	TracesEndpoint        string `json:"tracesEndpoint,omitempty"`
	ServiceName           string `json:"serviceName,omitempty"`
	ResourceAttributes    string `json:"resourceAttributes,omitempty"`
	GorillaTracer         string `json:"gorillaTracer,omitempty"`
	StatsdAddress         string `json:"statsdAddress,omitempty"`
	DatadogTraceAgentURL  string `json:"datadogTraceAgentURL,omitempty"`
	DatadogTraceAgentHost string `json:"datadogTraceAgentHost,omitempty"`
	DatadogTraceAgentPort string `json:"datadogTraceAgentPort,omitempty"`
}
