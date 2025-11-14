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

type WBDatabaseType string

const (
	WBDatabaseTypePercona WBDatabaseType = "percona"
)

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=wandb
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="Database",type=string,JSONPath=`.status.databaseStatus.state`
//+kubebuilder:printcolumn:name="Redis",type=string,JSONPath=`.status.redisStatus.state`
//+kubebuilder:printcolumn:name="Kafka",type=string,JSONPath=`.status.kafkaStatus.state`
//+kubebuilder:printcolumn:name="ObjStorage",type=string,JSONPath=`.status.objStorageStatus.state`
//+kubebuilder:printcolumn:name="ClickHouse",type=string,JSONPath=`.status.clickhouseStatus.state`
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

	Database   WBDatabaseSpec   `json:"database,omitempty"`
	Redis      WBRedisSpec      `json:"redis,omitempty"`
	Kafka      WBKafkaSpec      `json:"kafka,omitempty"`
	ObjStorage WBObjStorageSpec `json:"objStorage,omitempty"`
	ClickHouse WBClickHouseSpec `json:"clickhouse,omitempty"`
}

type WBDatabaseSpec struct {
	Enabled     bool           `json:"enabled"`
	Type        WBDatabaseType `json:"type,omitempty" default:"percona"`
	StorageSize string         `json:"storageSize,omitempty"`
	Backup      WBBackupSpec   `json:"backup,omitempty"`
	// Namespace is the target namespace for database resources.
	// If not specified, defaults to the WeightsAndBiases resource's namespace.
	Namespace string `json:"namespace,omitempty"`
}

type WBRedisSpec struct {
	Enabled     bool                 `json:"enabled"`
	StorageSize string               `json:"storageSize,omitempty"`
	Config      *WBRedisConfig       `json:"config,omitempty"`
	Sentinel    *WBRedisSentinelSpec `json:"sentinel,omitempty"`
	// Namespace is the target namespace for Redis resources.
	// If not specified, defaults to the WeightsAndBiases resource's namespace.
	Namespace string `json:"namespace,omitempty"`
}

type WBRedisConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type WBRedisSentinelSpec struct {
	Enabled bool                   `json:"enabled"`
	Config  *WBRedisSentinelConfig `json:"config,omitempty"`
}

type WBRedisSentinelConfig struct {
	MasterName string                      `json:"masterName,omitempty"`
	Resources  corev1.ResourceRequirements `json:"resources,omitempty"`
}

type WBKafkaSpec struct {
	Enabled     bool              `json:"enabled"`
	StorageSize string            `json:"storageSize,omitempty"`
	Replicas    int32             `json:"replicas,omitempty"`
	Backup      WBKafkaBackupSpec `json:"backup,omitempty"`
	// Namespace is the target namespace for Kafka resources.
	// If not specified, defaults to the WeightsAndBiases resource's namespace.
	Namespace string `json:"namespace,omitempty"`
}

type WBKafkaBackupSpec struct {
	Enabled        bool                    `json:"enabled,omitempty"`
	StorageName    string                  `json:"storageName,omitempty"`
	StorageType    WBBackupStorageType     `json:"storageType,omitempty"`
	Filesystem     *WBBackupFilesystemSpec `json:"filesystem,omitempty"`
	TimeoutSeconds int                     `json:"timeoutSeconds,omitempty"`
}

type WBObjStorageSpec struct {
	Enabled     bool                   `json:"enabled"`
	StorageSize string                 `json:"storageSize,omitempty"`
	Replicas    int32                  `json:"replicas,omitempty"`
	Backup      WBObjStorageBackupSpec `json:"backup,omitempty"`
	// Namespace is the target namespace for object storage resources.
	// If not specified, defaults to the WeightsAndBiases resource's namespace.
	Namespace string `json:"namespace,omitempty"`
}

type WBObjStorageBackupSpec struct {
	Enabled        bool                    `json:"enabled,omitempty"`
	StorageName    string                  `json:"storageName,omitempty"`
	StorageType    WBBackupStorageType     `json:"storageType,omitempty"`
	Filesystem     *WBBackupFilesystemSpec `json:"filesystem,omitempty"`
	TimeoutSeconds int                     `json:"timeoutSeconds,omitempty"`
}

type WBClickHouseSpec struct {
	Enabled     bool                   `json:"enabled"`
	StorageSize string                 `json:"storageSize,omitempty"`
	Replicas    int32                  `json:"replicas,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Backup      WBClickHouseBackupSpec `json:"backup,omitempty"`
	// Namespace is the target namespace for ClickHouse resources.
	// If not specified, defaults to the WeightsAndBiases resource's namespace.
	Namespace string `json:"namespace,omitempty"`
}

type WBClickHouseBackupSpec struct {
	Enabled        bool                    `json:"enabled,omitempty"`
	StorageName    string                  `json:"storageName,omitempty"`
	StorageType    WBBackupStorageType     `json:"storageType,omitempty"`
	Filesystem     *WBBackupFilesystemSpec `json:"filesystem,omitempty"`
	TimeoutSeconds int                     `json:"timeoutSeconds,omitempty"`
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
	DatabaseStatus     WBDatabaseStatus   `json:"databaseStatus,omitempty"`
	RedisStatus        WBRedisStatus      `json:"redisStatus,omitempty"`
	KafkaStatus        WBKafkaStatus      `json:"kafkaStatus,omitempty"`
	ObjStorageStatus   WBObjStorageStatus `json:"objStorageStatus,omitempty"`
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

// WorseThan is used to determine the case where, given a series of
// states describing a system, select the state that is the worst and use that
// to as the overall state of the system.
func (left WBStateType) WorseThan(right WBStateType) bool {
	var leftValue = statePrecedence[WBStateUnknown]
	var rightValue = statePrecedence[WBStateUnknown]
	var ok bool
	if _, ok = statePrecedence[left]; ok {
		leftValue = statePrecedence[left]
	}
	if _, ok = statePrecedence[right]; ok {
		rightValue = statePrecedence[right]
	}
	return leftValue > rightValue
}

type WBStatusDetail struct {
	State   WBStateType `json:"state"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
}

type WBDatabaseStatus struct {
	Ready          bool             `json:"ready"`
	State          WBStateType      `json:"state,omitempty" default:"Unknown"`
	Details        []WBStatusDetail `json:"details,omitempty"`
	LastReconciled metav1.Time      `json:"lastReconciled,omitempty"`
	BackupStatus   WBBackupStatus   `json:"backupStatus,omitempty"`
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
	Ready          bool              `json:"ready"`
	State          WBStateType       `json:"state,omitempty" default:"Unknown"`
	Details        []WBStatusDetail  `json:"details,omitempty"`
	LastReconciled metav1.Time       `json:"lastReconciled,omitempty"`
	Connection     WBRedisConnection `json:"connection,omitempty"`
}

type WBRedisConnection struct {
	RedisHost         string `json:"REDIS_HOST,omitempty"`
	RedisPort         string `json:"REDIS_PORT,omitempty"`
	RedisSentinelHost string `json:"REDIS_SENTINEL_HOST,omitempty"`
	RedisSentinelPort string `json:"REDIS_SENTINEL_PORT,omitempty"`
	RedisMasterName   string `json:"REDIS_MASTER_NAME,omitempty"`
}

type WBKafkaStatus struct {
	Ready          bool             `json:"ready"`
	State          WBStateType      `json:"state,omitempty" default:"Unknown"`
	Details        []WBStatusDetail `json:"details,omitempty"`
	LastReconciled metav1.Time      `json:"lastReconciled,omitempty"`
	BackupStatus   WBBackupStatus   `json:"backupStatus,omitempty"`
}

type WBObjStorageStatus struct {
	Ready          bool             `json:"ready"`
	State          WBStateType      `json:"state,omitempty" default:"Unknown"`
	Details        []WBStatusDetail `json:"details,omitempty"`
	LastReconciled metav1.Time      `json:"lastReconciled,omitempty"`
	BackupStatus   WBBackupStatus   `json:"backupStatus,omitempty"`
}

type WBClickHouseStatus struct {
	Ready          bool             `json:"ready"`
	State          WBStateType      `json:"state,omitempty" default:"Unknown"`
	Details        []WBStatusDetail `json:"details,omitempty"`
	LastReconciled metav1.Time      `json:"lastReconciled,omitempty"`
	BackupStatus   WBBackupStatus   `json:"backupStatus,omitempty"`
}
