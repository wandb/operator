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

type WBSpecProfile string

const (
	WBSpecProfileDev     WBSpecProfile = "dev"
	WBSpecProfileStaging WBSpecProfile = "staging"
	WBSpecProfileProd    WBSpecProfile = "prod"
)

type WBDatabaseType string

const (
	WBDatabaseTypePercona WBDatabaseType = "percona"
)

type WBStateType string

const (
	WBStateReady          WBStateType = "Ready"
	WBStateError          WBStateType = "Error"
	WBStateInfraUpdate    WBStateType = "InfraUpdate"
	WBStateDeleting       WBStateType = "Deleting"
	WBStateDeletionPaused WBStateType = "DeletionPaused"
)

type WBInfraStatusType string

const (
	WBInfraStatusReady    WBInfraStatusType = "Ready"
	WBInfraStatusDisabled WBInfraStatusType = "Disabled"
	WBInfraStatusError    WBInfraStatusType = "Error"
	WBInfraStatusPending  WBInfraStatusType = "Pending"
	WBInfraStatusMissing  WBInfraStatusType = "Missing"
	WBInfraStatusDeleting WBInfraStatusType = "Deleting"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=wandb
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="Database",type=string,JSONPath=`.status.databaseStatus.state`
//+kubebuilder:printcolumn:name="Redis",type=string,JSONPath=`.status.redisStatus.state`
//+kubebuilder:printcolumn:name="Kafka",type=string,JSONPath=`.status.kafkaStatus.state`
//+kubebuilder:printcolumn:name="ObjStorage",type=string,JSONPath=`.status.objStorageStatus.state`

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

// WeightsAndBiasesSpec defines the desired state of WeightsAndBiases.
type WeightsAndBiasesSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Profile is akin to high-level environment info
	Profile string `json:"profile,omitempty"`

	Database   WBDatabaseSpec   `json:"database,omitempty"`
	Redis      WBRedisSpec      `json:"redis,omitempty"`
	Kafka      WBKafkaSpec      `json:"kafka,omitempty"`
	ObjStorage WBObjStorageSpec `json:"objStorage,omitempty"`
}

type WBDatabaseSpec struct {
	Enabled     bool           `json:"enabled"`
	Type        WBDatabaseType `json:"type,omitempty" default:"percona"`
	StorageSize string         `json:"storageSize,omitempty"`
	Backup      WBBackupSpec   `json:"backup,omitempty"`
}

type WBRedisSpec struct {
	Enabled     bool   `json:"enabled"`
	StorageSize string `json:"storageSize,omitempty"`
}

type WBKafkaSpec struct {
	Enabled     bool              `json:"enabled"`
	StorageSize string            `json:"storageSize,omitempty"`
	Replicas    int32             `json:"replicas,omitempty"`
	Backup      WBKafkaBackupSpec `json:"backup,omitempty"`
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
}

type WBObjStorageBackupSpec struct {
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

type WBDatabaseStatus struct {
	Ready          bool           `json:"ready"`
	State          string         `json:"state,omitempty" default:"Missing"`
	LastReconciled metav1.Time    `json:"lastReconciled,omitempty"`
	BackupStatus   WBBackupStatus `json:"backupStatus,omitempty"`
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

// WeightsAndBiasesStatus defines the observed state of WeightsAndBiases.
type WeightsAndBiasesStatus struct {
	State              WBStateType        `json:"state,omitempty"`
	Message            string             `json:"message,omitempty"`
	DatabaseStatus     WBDatabaseStatus   `json:"databaseStatus,omitempty"`
	RedisStatus        WBRedisStatus      `json:"redisStatus,omitempty"`
	KafkaStatus        WBKafkaStatus      `json:"kafkaStatus,omitempty"`
	ObjStorageStatus   WBObjStorageStatus `json:"objStorageStatus,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration"`
}

type WBRedisStatus struct {
	Ready          bool        `json:"ready"`
	State          string      `json:"state,omitempty" default:"Missing"`
	LastReconciled metav1.Time `json:"lastReconciled,omitempty"`
}

type WBKafkaStatus struct {
	Ready          bool           `json:"ready"`
	State          string         `json:"state,omitempty" default:"Missing"`
	LastReconciled metav1.Time    `json:"lastReconciled,omitempty"`
	BackupStatus   WBBackupStatus `json:"backupStatus,omitempty"`
}

type WBObjStorageStatus struct {
	Ready          bool           `json:"ready"`
	State          string         `json:"state,omitempty" default:"Missing"`
	LastReconciled metav1.Time    `json:"lastReconciled,omitempty"`
	BackupStatus   WBBackupStatus `json:"backupStatus,omitempty"`
}
