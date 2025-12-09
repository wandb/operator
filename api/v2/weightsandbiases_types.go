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
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
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
// +kubebuilder:storageversion

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

// WeightsAndBiasesSpec defines the desired state of WeightsAndBiases.
type WeightsAndBiasesSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Size is akin to high-level environment info
	Size WBSize `json:"size,omitempty"`

	MySQL      WBMySQLSpec      `json:"mysql,omitempty"`
	Redis      WBRedisSpec      `json:"redis,omitempty"`
	Kafka      WBKafkaSpec      `json:"kafka,omitempty"`
	Minio      WBMinioSpec      `json:"minio,omitempty"`
	ClickHouse WBClickHouseSpec `json:"clickhouse,omitempty"`
}

// WBMySQLSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type WBMySQLSpec struct {
	Enabled     bool          `json:"enabled"`
	StorageSize string        `json:"storageSize,omitempty"`
	Replicas    int32         `json:"replicas,omitempty"`
	Config      WBMySQLConfig `json:"config,omitempty"`
	Namespace   string        `json:"namespace,omitempty"`
	Name        string        `json:"name,omitempty"`
}

type WBMySQLConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// WBRedisSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type WBRedisSpec struct {
	Enabled     bool                `json:"enabled"`
	StorageSize string              `json:"storageSize,omitempty"`
	Config      WBRedisConfig       `json:"config,omitempty"`
	Sentinel    WBRedisSentinelSpec `json:"sentinel,omitempty"`
	Namespace   string              `json:"namespace,omitempty"`
	Name        string              `json:"name,omitempty"`
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
	Enabled     bool          `json:"enabled"`
	StorageSize string        `json:"storageSize,omitempty"`
	Replicas    int32         `json:"replicas,omitempty"`
	Config      WBKafkaConfig `json:"config,omitempty"`
	Namespace   string        `json:"namespace,omitempty"`
	Name        string        `json:"name,omitempty"`
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
	Enabled     bool          `json:"enabled"`
	StorageSize string        `json:"storageSize,omitempty"`
	Replicas    int32         `json:"replicas,omitempty"`
	Config      WBMinioConfig `json:"config,omitempty"`
	Namespace   string        `json:"namespace,omitempty"`
	Name        string        `json:"name,omitempty"`
}

type WBMinioConfig struct {
	Resources           corev1.ResourceRequirements `json:"resources,omitempty"`
	RootUser            string                      `json:"rootUser,omitempty"`
	MinioBrowserSetting string                      `json:"minioBrowserSetting,omitempty"`
}

// WBClickHouseSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type WBClickHouseSpec struct {
	Enabled     bool               `json:"enabled"`
	StorageSize string             `json:"storageSize,omitempty"`
	Replicas    int32              `json:"replicas,omitempty"`
	Version     string             `json:"version,omitempty"`
	Config      WBClickHouseConfig `json:"config,omitempty"`
	Namespace   string             `json:"namespace,omitempty"`
	Name        string             `json:"name,omitempty"`
}

type WBClickHouseConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// WeightsAndBiasesStatus defines the observed state of WeightsAndBiases.
type WeightsAndBiasesStatus struct {
	State              string             `json:"state,omitempty"`
	MySQLStatus        WBMySQLStatus      `json:"mysqlStatus,omitempty"`
	RedisStatus        WBRedisStatus      `json:"redisStatus,omitempty"`
	KafkaStatus        WBKafkaStatus      `json:"kafkaStatus,omitempty"`
	MinioStatus        WBMinioStatus      `json:"minioStatus,omitempty"`
	ClickHouseStatus   WBClickHouseStatus `json:"clickhouseStatus,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration"`
}

type WBMySQLStatus struct {
	Ready          bool               `json:"ready"`
	State          string             `json:"state,omitempty" default:"Unknown"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
	LastReconciled metav1.Time        `json:"lastReconciled,omitempty"`
	Connection     WBInfraConnection  `json:"connection,omitempty"`
}

type WBRedisStatus struct {
	Ready          bool               `json:"ready"`
	State          string             `json:"state,omitempty" default:"Unknown"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
	LastReconciled metav1.Time        `json:"lastReconciled,omitempty"`
	Connection     WBInfraConnection  `json:"connection,omitempty"`
}

type WBKafkaStatus struct {
	Ready          bool               `json:"ready"`
	State          string             `json:"state,omitempty" default:"Unknown"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
	LastReconciled metav1.Time        `json:"lastReconciled,omitempty"`
	Connection     WBInfraConnection  `json:"connection,omitempty"`
}

type WBMinioStatus struct {
	Ready          bool               `json:"ready"`
	State          string             `json:"state,omitempty" default:"Unknown"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
	LastReconciled metav1.Time        `json:"lastReconciled,omitempty"`
	Connection     WBInfraConnection  `json:"connection,omitempty"`
}

type WBClickHouseStatus struct {
	Ready          bool               `json:"ready"`
	State          string             `json:"state,omitempty" default:"Unknown"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
	LastReconciled metav1.Time        `json:"lastReconciled,omitempty"`
	LastObserved   metav1.Time        `json:"lastObserved,omitempty"`
	Connection     WBInfraConnection  `json:"connection,omitempty"`
}

type WBInfraConnection struct {
	URL corev1.SecretKeySelector `json:"url,omitempty"`
}
