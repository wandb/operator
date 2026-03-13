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
//+kubebuilder:printcolumn:name="MySQL",type=string,JSONPath=`.status.mysqlStatus.state`
//+kubebuilder:printcolumn:name="Redis",type=string,JSONPath=`.status.redisStatus.state`
//+kubebuilder:printcolumn:name="Kafka",type=string,JSONPath=`.status.kafkaStatus.state`
//+kubebuilder:printcolumn:name="Minio",type=string,JSONPath=`.status.minioStatus.state`
//+kubebuilder:printcolumn:name="ClickHouse",type=string,JSONPath=`.status.clickhouseStatus.state`

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

	Wandb WandbAppSpec `json:"wandb,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`

	MySQL      MySQLSpec      `json:"mysql,omitempty"`
	Redis      RedisSpec      `json:"redis,omitempty"`
	Kafka      KafkaSpec      `json:"kafka,omitempty"`
	Minio      MinioSpec      `json:"minio,omitempty"`
	ClickHouse ClickHouseSpec `json:"clickhouse,omitempty"`

	// Networking configures how the W&B application is exposed externally.
	// +optional
	Networking NetworkingSpec `json:"networking,omitempty"`
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

	// Annotations applied to all generated Ingress or HTTPRoute resources.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type IngressConfig struct {
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`
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

	// Annotations applied to the managed Gateway resource.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
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

func (w *WeightsAndBiases) GetRetentionPolicy(spec InfraSpec) RetentionPolicy {
	if spec.RetentionPolicy != nil {
		return *spec.RetentionPolicy
	}
	return w.Spec.RetentionPolicy
}

func (w *WeightsAndBiases) GetAffinity(spec InfraSpec) *corev1.Affinity {
	if spec.Affinity != nil {
		return spec.Affinity
	}
	return w.Spec.Affinity
}

func (w *WeightsAndBiases) GetTolerations(spec InfraSpec) *[]corev1.Toleration {
	if spec.Tolerations != nil {
		return spec.Tolerations
	}
	return w.Spec.Tolerations
}

// WandbAppSpec defines the configuration for the Wandb application deployment.
type WandbAppSpec struct {
	Hostname            string              `json:"hostname"`
	License             string              `json:"license,omitempty"`
	ManifestRepository  string              `json:"manifestRepository,omitempty"`
	Version             string              `json:"version"`
	Features            map[string]bool     `json:"features"`
	InternalServiceAuth InternalServiceAuth `json:"internalServiceAuth,omitempty"`

	ServiceAccount ServiceAccountSpec `json:"serviceAccount,omitempty"`

	// +optional
	AdditionalHostnames []string `json:"additionalHostnames,omitempty"`

	// +optional
	OIDC OidcSpec `json:"oidc,omitempty"`
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
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	IssuerUrl    string `json:"issuerUrl"`
	AuthMethod   string `json:"authMethod"`
}

type MYSQLType string

const (
	MySQLTypeMariadb MYSQLType = "mariadb"
	MySQLTypePercona MYSQLType = "percona"
	MySQLTypeMysql   MYSQLType = "mysql"
)

type InfraSpec struct {
	Enabled         bool             `json:"enabled"`
	RetentionPolicy *RetentionPolicy `json:"retentionPolicy,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`
}

// MySQLSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type MySQLSpec struct {
	InfraSpec `json:",inline"`

	DeploymentType MYSQLType   `json:"deploymentType"`
	StorageSize    string      `json:"storageSize,omitempty"`
	Replicas       int32       `json:"replicas,omitempty"`
	Config         MySQLConfig `json:"config,omitempty"`
	Namespace      string      `json:"namespace,omitempty"`
	Name           string      `json:"name,omitempty"`
	Telemetry      Telemetry   `json:"telemetry,omitempty"`
}

type MySQLConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// Telemetry defines telemetry configuration for infrastructure components
type Telemetry struct {
	// +kubebuilder:default=true
	Enabled bool `json:"enabled" default:"true"`
}

// RedisSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type RedisSpec struct {
	InfraSpec `json:",inline"`

	StorageSize string            `json:"storageSize,omitempty"`
	Config      RedisConfig       `json:"config,omitempty"`
	Sentinel    RedisSentinelSpec `json:"sentinel,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Name        string            `json:"name,omitempty"`
	Telemetry   Telemetry         `json:"telemetry,omitempty"`
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

// KafkaSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type KafkaSpec struct {
	InfraSpec `json:",inline"`

	StorageSize      string      `json:"storageSize,omitempty"`
	Replicas         int32       `json:"replicas,omitempty"`
	Config           KafkaConfig `json:"config,omitempty"`
	Namespace        string      `json:"namespace,omitempty"`
	Name             string      `json:"name,omitempty"`
	Telemetry        Telemetry   `json:"telemetry,omitempty"`
	SkipDataRecovery bool        `json:"skipDataRecovery,omitempty"`
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

// MinioSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type MinioSpec struct {
	InfraSpec `json:",inline"`

	StorageSize string      `json:"storageSize,omitempty"`
	Replicas    int32       `json:"replicas,omitempty"`
	Config      MinioConfig `json:"config,omitempty"`
	Namespace   string      `json:"namespace,omitempty"`
	Name        string      `json:"name,omitempty"`
	Telemetry   Telemetry   `json:"telemetry,omitempty"`
}

type MinioConfig struct {
	Resources           corev1.ResourceRequirements `json:"resources,omitempty"`
	RootUser            string                      `json:"rootUser,omitempty"`
	MinioBrowserSetting string                      `json:"minioBrowserSetting,omitempty"`
}

// ClickHouseSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type ClickHouseSpec struct {
	InfraSpec `json:",inline"`

	StorageSize string           `json:"storageSize,omitempty"`
	Replicas    int32            `json:"replicas,omitempty"`
	Version     string           `json:"version,omitempty"`
	Config      ClickHouseConfig `json:"config,omitempty"`
	Namespace   string           `json:"namespace,omitempty"`
	Name        string           `json:"name,omitempty"`
	Telemetry   Telemetry        `json:"telemetry,omitempty"`
}

type ClickHouseConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// WeightsAndBiasesStatus defines the observed state of WeightsAndBiases.
type WeightsAndBiasesStatus struct {
	Ready            bool          `json:"ready"`
	Wandb            WandbStatus   `json:"wandb,omitempty"`
	MySQLStatus      WBInfraStatus `json:"mysqlStatus,omitempty"`
	RedisStatus      WBInfraStatus `json:"redisStatus,omitempty"`
	KafkaStatus      WBInfraStatus `json:"kafkaStatus,omitempty"`
	MinioStatus      WBInfraStatus `json:"minioStatus,omitempty"`
	ClickHouseStatus WBInfraStatus `json:"clickhouseStatus,omitempty"`
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
	Name      string   `json:"name,omitempty"`
	Ready     bool     `json:"ready,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
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

	// +kubebuilder:default:={}
	MySQLInit MigrationJobStatus `json:"mysqlInit,omitempty"`
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
	Connection WBInfraConnection  `json:"connection,omitempty"`
}

type WBInfraConnection struct {
	URL corev1.SecretKeySelector `json:"url,omitempty"`
}
