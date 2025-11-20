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
	"k8s.io/apimachinery/pkg/api/resource"
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
//+kubebuilder:webhook:path=/mutate-apps-wandb-com-v2-weightsandbiases,mutating=true,failurePolicy=fail,sideEffects=None,groups=apps.wandb.com,resources=weightsandbiases,verbs=create;update,versions=v2,name=mweightsandbiases.wandb.com,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-apps-wandb-com-v2-weightsandbiases,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.wandb.com,resources=weightsandbiases,verbs=create;update;delete,versions=v2,name=vweightsandbiases.wandb.com,admissionReviewVersions=v1

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
	Enabled         bool                  `json:"enabled"`
	SentinelName    string                `json:"sentinelName,omitempty"`
	ReplicationName string                `json:"replicationName,omitempty"`
	Config          WBRedisSentinelConfig `json:"config,omitempty"`
}

type WBRedisSentinelConfig struct {
	MasterName string                      `json:"masterName,omitempty"`
	Resources  corev1.ResourceRequirements `json:"resources,omitempty"`
}

// WBKafkaSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type WBKafkaSpec struct {
	Enabled     bool              `json:"enabled"`
	StorageSize string            `json:"storageSize,omitempty"`
	Replicas    int32             `json:"replicas,omitempty"`
	Config      WBKafkaConfig     `json:"config,omitempty"`
	Backup      WBKafkaBackupSpec `json:"backup,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Name        string            `json:"name,omitempty"`
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

type WBKafkaBackupSpec struct {
	Enabled        bool                    `json:"enabled,omitempty"`
	StorageName    string                  `json:"storageName,omitempty"`
	StorageType    WBBackupStorageType     `json:"storageType,omitempty"`
	Filesystem     *WBBackupFilesystemSpec `json:"filesystem,omitempty"`
	TimeoutSeconds int                     `json:"timeoutSeconds,omitempty"`
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
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
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

type WBBackupSpec struct {
	Enabled        bool                    `json:"enabled,omitempty"`
	StorageName    string                  `json:"storageName,omitempty"`
	StorageType    WBBackupStorageType     `json:"storageType,omitempty"`
	S3             *WBBackupS3Spec         `json:"s3,omitempty"`
	Filesystem     *WBBackupFilesystemSpec `json:"filesystem,omitempty"`
	TimeoutSeconds int                     `json:"timeoutSeconds,omitempty"`
}

type WBBackupStorageType string

const (
	WBBackupStorageTypeS3         WBBackupStorageType = "s3"
	WBBackupStorageTypeFilesystem WBBackupStorageType = "filesystem"
)

type WBBackupS3Spec struct {
	Bucket            string `json:"bucket"`
	Region            string `json:"region,omitempty"`
	CredentialsSecret string `json:"credentialsSecret,omitempty"`
	EndpointURL       string `json:"endpointUrl,omitempty"`
}

type WBBackupFilesystemSpec struct {
	StorageSize      resource.Quantity                   `json:"storageSize,omitempty"`
	StorageClassName string                              `json:"storageClassName,omitempty"`
	AccessModes      []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// WeightsAndBiasesStatus defines the observed state of WeightsAndBiases.
type WeightsAndBiasesStatus struct {
	State              WBStateType        `json:"state,omitempty"`
	MySQLStatus        WBMySQLStatus      `json:"mysqlStatus,omitempty"`
	RedisStatus        WBRedisStatus      `json:"redisStatus,omitempty"`
	KafkaStatus        WBKafkaStatus      `json:"kafkaStatus,omitempty"`
	MinioStatus        WBMinioStatus      `json:"minioStatus,omitempty"`
	ClickHouseStatus   WBClickHouseStatus `json:"clickhouseStatus,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration"`
}

type WBStateType string

const (
	WBStateError    WBStateType = "Error"
	WBStateDegraded WBStateType = "Degraded"
	WBStateUpdating WBStateType = "Updating"
	WBStateDeleting WBStateType = "Deleting"
	WBStateOffline  WBStateType = "Offline"
	WBStateReady    WBStateType = "Ready"
	WBStateUnknown  WBStateType = "Unknown"
)

var statePrecedence = map[WBStateType]int{
	WBStateError:    1,
	WBStateDegraded: 2,
	WBStateUpdating: 3,
	WBStateDeleting: 4,
	WBStateOffline:  5,
	WBStateReady:    50,
	WBStateUnknown:  99,
}

// IsWorseThan is used to determine the case where, given a series of
// states describing a system, select the state that is the worst and use that
// to as the overall state of the system.
func (left WBStateType) IsWorseThan(right WBStateType) bool {
	var leftValue = statePrecedence[WBStateUnknown]
	var rightValue = statePrecedence[WBStateUnknown]
	var ok bool
	if _, ok = statePrecedence[left]; ok {
		leftValue = statePrecedence[left]
	}
	if _, ok = statePrecedence[right]; ok {
		rightValue = statePrecedence[right]
	}
	return leftValue < rightValue
}

type WBStatusCondition struct {
	State   WBStateType `json:"state"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
}

type WBMySQLStatus struct {
	Ready          bool                `json:"ready"`
	State          WBStateType         `json:"state,omitempty" default:"Unknown"`
	Conditions     []WBStatusCondition `json:"conditions,omitempty"`
	LastReconciled metav1.Time         `json:"lastReconciled,omitempty"`
	Connection     WBMySQLConnection   `json:"connection,omitempty"`
	// Deprecated: BackupStatus is not implemented in the refactored MySQL code
	BackupStatus WBBackupStatus `json:"backupStatus,omitempty"`
}

type WBMySQLConnection struct {
	MySQLHost string `json:"MYSQL_HOST,omitempty"`
	MySQLPort string `json:"MYSQL_PORT,omitempty"`
	MySQLUser string `json:"MYSQL_USER,omitempty"`
}

type WBBackupStatus struct {
	BackupName     string       `json:"backupName,omitempty"`
	StartedAt      *metav1.Time `json:"startedAt,omitempty"`
	CompletedAt    *metav1.Time `json:"completedAt,omitempty"`
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`
	State          string       `json:"state,omitempty"`
	Message        string       `json:"message,omitempty"`
	RequeueAfter   int64        `json:"requeueAfter,omitempty"`
}

type WBRedisStatus struct {
	Ready          bool                `json:"ready"`
	State          WBStateType         `json:"state,omitempty" default:"Unknown"`
	Conditions     []WBStatusCondition `json:"conditions,omitempty"`
	LastReconciled metav1.Time         `json:"lastReconciled,omitempty"`
	Connection     WBRedisConnection   `json:"connection,omitempty"`
}

type WBRedisConnection struct {
	RedisHost         string `json:"REDIS_HOST,omitempty"`
	RedisPort         string `json:"REDIS_PORT,omitempty"`
	RedisSentinelHost string `json:"REDIS_SENTINEL_HOST,omitempty"`
	RedisSentinelPort string `json:"REDIS_SENTINEL_PORT,omitempty"`
	RedisMasterName   string `json:"REDIS_MASTER_NAME,omitempty"`
}

type WBKafkaConnection struct {
	KafkaHost string `json:"KAFKA_HOST,omitempty"`
	KafkaPort string `json:"KAFKA_PORT,omitempty"`
}

type WBKafkaStatus struct {
	Ready          bool                `json:"ready"`
	State          WBStateType         `json:"state,omitempty" default:"Unknown"`
	Conditions     []WBStatusCondition `json:"conditions,omitempty"`
	LastReconciled metav1.Time         `json:"lastReconciled,omitempty"`
	Connection     WBKafkaConnection   `json:"connection,omitempty"`
	BackupStatus   WBBackupStatus      `json:"backupStatus,omitempty"`
}

type WBMinioStatus struct {
	Ready          bool                `json:"ready"`
	State          WBStateType         `json:"state,omitempty" default:"Unknown"`
	Conditions     []WBStatusCondition `json:"conditions,omitempty"`
	LastReconciled metav1.Time         `json:"lastReconciled,omitempty"`
	Connection     WBMinioConnection   `json:"connection,omitempty"`
	// Deprecated: BackupStatus is not implemented in the refactored Minio code
	BackupStatus WBBackupStatus `json:"backupStatus,omitempty"`
}

type WBMinioConnection struct {
	MinioHost      string `json:"MINIO_HOST,omitempty"`
	MinioPort      string `json:"MINIO_PORT,omitempty"`
	MinioAccessKey string `json:"MINIO_ACCESS_KEY,omitempty"`
}

type WBClickHouseStatus struct {
	Ready          bool                   `json:"ready"`
	State          WBStateType            `json:"state,omitempty" default:"Unknown"`
	Conditions     []WBStatusCondition    `json:"conditions,omitempty"`
	LastReconciled metav1.Time            `json:"lastReconciled,omitempty"`
	Connection     WBClickHouseConnection `json:"connection,omitempty"`
	// Deprecated: BackupStatus is not implemented in the refactored ClickHouse code
	BackupStatus WBBackupStatus `json:"backupStatus,omitempty"`
}

type WBClickHouseConnection struct {
	ClickHouseHost string `json:"CLICKHOUSE_HOST,omitempty"`
	ClickHousePort string `json:"CLICKHOUSE_PORT,omitempty"`
	ClickHouseUser string `json:"CLICKHOUSE_USER,omitempty"`
}
