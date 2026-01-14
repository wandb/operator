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

type WBSize string

const (
	WBSizeDev   WBSize = "dev"
	WBSizeSmall WBSize = "small"
)

type WBOnDeletePolicy string

const (
	// WBPreserveOnDelete will keep the resources necessary recreate with the same connection and data
	WBPreserveOnDelete WBOnDeletePolicy = "preserve"
	// WBPurgeOnDelete will delete all associated resources upon deletion
	WBPurgeOnDelete WBOnDeletePolicy = "purge"
)

type WBRetentionPolicy struct {
	// +kubebuilder:default="preserve"
	OnDelete WBOnDeletePolicy `json:"onDelete" default:"preserve"`
}

// WeightsAndBiasesSpec defines the desired state of WeightsAndBiases.
type WeightsAndBiasesSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Size is akin to high-level environment info
	Size WBSize `json:"size,omitempty"`

	RetentionPolicy WBRetentionPolicy `json:"retentionPolicy"`

	Wandb WandbAppSpec `json:"wandb,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`

	MySQL      WBMySQLSpec      `json:"mysql,omitempty"`
	Redis      WBRedisSpec      `json:"redis,omitempty"`
	Kafka      WBKafkaSpec      `json:"kafka,omitempty"`
	Minio      WBMinioSpec      `json:"minio,omitempty"`
	ClickHouse WBClickHouseSpec `json:"clickhouse,omitempty"`
}

// WandbAppSpec defines the configuration for the Wandb application deployment.
type WandbAppSpec struct {
	Hostname string `json:"hostname"`
	License  string `json:"license,omitempty"`

	Version string `json:"version"`

	Features map[string]bool `json:"features"`

	// +optional
	AdditionalHostnames []string `json:"additionalHostnames,omitempty"`

	// +optional
	OIDC WandbOIDCSpec `json:"oidc,omitempty"`
}

// WandbOIDCSpec defines the structure for OpenID Connect (OIDC) configuration used in Wandb application deployments.
type WandbOIDCSpec struct {
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	IssuerUrl    string `json:"issuerUrl"`
	AuthMethod   string `json:"authMethod"`
}

// WBMySQLSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type WBMySQLSpec struct {
	Enabled     	bool          `json:"enabled"`
	StorageSize 	string        `json:"storageSize,omitempty"`
	Replicas    	int32         `json:"replicas,omitempty"`
	Config      	WBMySQLConfig `json:"config,omitempty"`
	Namespace   	string        `json:"namespace,omitempty"`
	Name        	string        `json:"name,omitempty"`
	Telemetry   	Telemetry     `json:"telemetry,omitempty"`
	RetentionPolicy *WBRetentionPolicy `json:"retentionPolicy,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`
}

type WBMySQLConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// Telemetry defines telemetry configuration for infrastructure components
type Telemetry struct {
	// +kubebuilder:default=true
	Enabled bool `json:"enabled" default:"true"`
}

// WBRedisSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type WBRedisSpec struct {
	Enabled     	bool                `json:"enabled"`
	StorageSize 	string              `json:"storageSize,omitempty"`
	Config      	WBRedisConfig       `json:"config,omitempty"`
	Sentinel    	WBRedisSentinelSpec `json:"sentinel,omitempty"`
	Namespace   	string              `json:"namespace,omitempty"`
	Name        	string              `json:"name,omitempty"`
	Telemetry   	Telemetry           `json:"telemetry,omitempty"`
	RetentionPolicy *WBRetentionPolicy `json:"retentionPolicy,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`
}

type WBRedisConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type WBRedisSentinelSpec struct {
	Enabled bool                  `json:"enabled"`
	Config  WBRedisSentinelConfig `json:"config,omitempty"`
}

type WBRedisSentinelConfig struct {
	MasterName string                      `json:"masterName,omitempty"`
	Resources  corev1.ResourceRequirements `json:"resources,omitempty"`
}

// WBKafkaSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type WBKafkaSpec struct {
	Enabled     	bool          `json:"enabled"`
	StorageSize 	string        `json:"storageSize,omitempty"`
	Replicas    	int32         `json:"replicas,omitempty"`
	Config      	WBKafkaConfig `json:"config,omitempty"`
	Namespace   	string        `json:"namespace,omitempty"`
	Name        	string        `json:"name,omitempty"`
	Telemetry   	Telemetry     `json:"telemetry,omitempty"`
	RetentionPolicy *WBRetentionPolicy `json:"retentionPolicy,omitempty"`

	SkipDataRecovery bool `json:"skipDataRecovery,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`
}

type WBKafkaConfig struct {
	Resources         corev1.ResourceRequirements `json:"resources,omitempty"`
	ReplicationConfig WBKafkaReplicationConfig    `json:"replicationConfig,omitempty"`
}

type WBKafkaReplicationConfig struct {
	DefaultReplicationFactor int32 `json:"defaultReplicationFactor,omitempty"`
	MinInSyncReplicas        int32 `json:"minInSyncReplicas,omitempty"`
	OffsetsTopicRF           int32 `json:"offsetsTopicRF,omitempty"`
	TransactionStateRF       int32 `json:"transactionStateISR,omitempty"`
	TransactionStateISR      int32 `json:"transactionStateRF,omitempty"`
}

// WBMinioSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type WBMinioSpec struct {
	Enabled     	bool          `json:"enabled"`
	StorageSize 	string        `json:"storageSize,omitempty"`
	Replicas    	int32         `json:"replicas,omitempty"`
	Config      	WBMinioConfig `json:"config,omitempty"`
	Namespace   	string        `json:"namespace,omitempty"`
	Name        	string        `json:"name,omitempty"`
	Telemetry   	Telemetry     `json:"telemetry,omitempty"`
	RetentionPolicy *WBRetentionPolicy `json:"retentionPolicy,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`
}

type WBMinioConfig struct {
	Resources           corev1.ResourceRequirements `json:"resources,omitempty"`
	RootUser            string                      `json:"rootUser,omitempty"`
	MinioBrowserSetting string                      `json:"minioBrowserSetting,omitempty"`
}

// WBClickHouseSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type WBClickHouseSpec struct {
	Enabled     	bool               `json:"enabled"`
	StorageSize 	string             `json:"storageSize,omitempty"`
	Replicas    	int32              `json:"replicas,omitempty"`
	Version     	string             `json:"version,omitempty"`
	Config      	WBClickHouseConfig `json:"config,omitempty"`
	Namespace   	string             `json:"namespace,omitempty"`
	Name        	string             `json:"name,omitempty"`
	Telemetry   	Telemetry          `json:"telemetry,omitempty"`
	RetentionPolicy *WBRetentionPolicy `json:"retentionPolicy,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`
}

type WBClickHouseConfig struct {
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
}

type WandbStatus struct {
	Hostname string `json:"hostname"`

	// +kubebuilder:default:={}
	Applications map[string]ApplicationStatus `json:"applications,omitempty"`
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
