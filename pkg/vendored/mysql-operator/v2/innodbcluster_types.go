/*
Copyright 2026.

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
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// +kubebuilder:object:generate=true
// InnoDBClusterSpec defines the desired state of InnoDBCluster
// +kubebuilder:object:generate=true
type InnoDBClusterSpec struct {
	SecretName                 string                        `json:"secretName"`
	TLSCASecretName            string                        `json:"tlsCASecretName,omitempty"`
	TLSSecretName              string                        `json:"tlsSecretName,omitempty"`
	TLSUseSelfSigned           bool                          `json:"tlsUseSelfSigned,omitempty"`
	Version                    string                        `json:"version,omitempty"`
	Edition                    string                        `json:"edition,omitempty"`
	ImageRepository            string                        `json:"imageRepository,omitempty"`
	ImagePullPolicy            corev1.PullPolicy             `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets           []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	ServiceAccountName         string                        `json:"serviceAccountName,omitempty"`
	BaseServerId               int32                         `json:"baseServerId,omitempty"`
	DatadirVolumeClaimTemplate *corev1.PersistentVolumeClaim `json:"datadirVolumeClaimTemplate,omitempty"`
	DatadirPermissions         *DatadirPermissions           `json:"datadirPermissions,omitempty"`
	Mycnf                      string                        `json:"mycnf,omitempty"`
	Instances                  int32                         `json:"instances,omitempty"`
	PodSpec                    *corev1.PodSpec               `json:"podSpec,omitempty"`
	PodAnnotations             map[string]string             `json:"podAnnotations,omitempty"`
	PodLabels                  map[string]string             `json:"podLabels,omitempty"`
	Keyring                    *KeyringSpec                  `json:"keyring,omitempty"`
	InitDB                     *InitDBSpec                   `json:"initDB,omitempty"`
	Router                     *RouterSpec                   `json:"router,omitempty"`
	InstanceService            *ServiceConfig                `json:"instanceService,omitempty"`
	Service                    *ServiceConfig                `json:"service,omitempty"`
	Metrics                    *MetricsSpec                  `json:"metrics,omitempty"`
	BackupProfiles             []BackupProfile               `json:"backupProfiles,omitempty"`
	BackupSchedules            []BackupSchedule              `json:"backupSchedules,omitempty"`
	Logs                       *LogsSpec                     `json:"logs,omitempty"`
	ReadReplicas               []ReadReplicaSpec             `json:"readReplicas,omitempty"`
	ServiceFqdnTemplate        string                        `json:"serviceFqdnTemplate,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:generate=true
type DatadirPermissions struct {
	SetRightsUsingInitContainer bool   `json:"setRightsUsingInitContainer,omitempty"`
	FSGroupChangePolicy         string `json:"fsGroupChangePolicy,omitempty"`
}

// +kubebuilder:object:generate=true
type KeyringSpec struct {
	KMIP          *KeyringKMIPSpec          `json:"kmip,omitempty"`
	File          *KeyringFileSpec          `json:"file,omitempty"`
	EncryptedFile *KeyringEncryptedFileSpec `json:"encryptedFile,omitempty"`
	HashiCorp     *KeyringHashiCorpSpec     `json:"hashicorp,omitempty"`
	OCI           *KeyringOCISpec           `json:"oci,omitempty"`
}

// +kubebuilder:object:generate=true
type KeyringKMIPSpec struct {
	Configuration string   `json:"configuration"`
	CacheKeys     bool     `json:"cacheKeys,omitempty"`
	Server        string   `json:"server"`
	StandbyServer []string `json:"standbyServer,omitempty"`
}

// +kubebuilder:object:generate=true
type KeyringFileSpec struct {
	FileName string              `json:"fileName,omitempty"`
	ReadOnly bool                `json:"readOnly,omitempty"`
	Storage  corev1.VolumeSource `json:"storage"`
}

// +kubebuilder:object:generate=true
type KeyringEncryptedFileSpec struct {
	FileName string              `json:"fileName,omitempty"`
	ReadOnly bool                `json:"readOnly,omitempty"`
	Password string              `json:"password"`
	Storage  corev1.VolumeSource `json:"storage"`
}

// +kubebuilder:object:generate=true
type KeyringHashiCorpSpec struct {
	CACertificate string               `json:"caCertificate,omitempty"`
	Caching       bool                 `json:"caching,omitempty"`
	ServerUrl     string               `json:"serverUrl"`
	StorePath     string               `json:"storePath"`
	Auth          KeyringHashiCorpAuth `json:"auth"`
}

// +kubebuilder:object:generate=true
type KeyringHashiCorpAuth struct {
	AppRole *KeyringHashiCorpAppRole `json:"approle,omitempty"`
	Token   *KeyringHashiCorpToken   `json:"token,omitempty"`
}

// +kubebuilder:object:generate=true
type KeyringHashiCorpAppRole struct {
	AuthenticationPath string `json:"authenticationPath,omitempty"`
	AuthSecret         string `json:"authSecret"`
}

// +kubebuilder:object:generate=true
type KeyringHashiCorpToken struct {
	TokenSecret string `json:"tokenSecret"`
}

// +kubebuilder:object:generate=true
type KeyringOCISpec struct {
	User           string               `json:"user"`
	KeySecret      string               `json:"keySecret"`
	KeyFingerprint string               `json:"keyFingerprint"`
	Tenancy        string               `json:"tenancy"`
	Compartment    string               `json:"compartment,omitempty"`
	VirtualVault   string               `json:"virtualVault,omitempty"`
	MasterKey      string               `json:"masterKey,omitempty"`
	Endpoints      *KeyringOCIEndpoints `json:"endpoints,omitempty"`
	CACertificate  string               `json:"caCertificate,omitempty"`
}

// +kubebuilder:object:generate=true
type KeyringOCIEndpoints struct {
	Encryption string `json:"encryption,omitempty"`
	Management string `json:"management,omitempty"`
	Vaults     string `json:"vaults,omitempty"`
	Secrets    string `json:"secrets,omitempty"`
}

// +kubebuilder:object:generate=true
type InitDBSpec struct {
	Clone *InitDBCloneSpec `json:"clone,omitempty"`
	Dump  *InitDBDumpSpec  `json:"dump,omitempty"`
	MEB   *InitDBMEBSpec   `json:"meb,omitempty"`
}

// +kubebuilder:object:generate=true
type InitDBCloneSpec struct {
	DonorUrl     string                      `json:"donorUrl"`
	RootUser     string                      `json:"rootUser,omitempty"`
	SecretKeyRef corev1.LocalObjectReference `json:"secretKeyRef"`
}

// +kubebuilder:object:generate=true
type InitDBDumpSpec struct {
	Name    string                          `json:"name,omitempty"`
	Path    string                          `json:"path,omitempty"`
	Options map[string]runtime.RawExtension `json:"options,omitempty"`
	Storage InitDBDumpStorage               `json:"storage"`
}

// +kubebuilder:object:generate=true
type InitDBDumpStorage struct {
	OCIObjectStorage      *OCIObjectStorageSpec             `json:"ociObjectStorage,omitempty"`
	S3                    *S3StorageSpec                    `json:"s3,omitempty"`
	Azure                 *AzureStorageSpec                 `json:"azure,omitempty"`
	PersistentVolumeClaim *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
}

// +kubebuilder:object:generate=true
type OCIObjectStorageSpec struct {
	BucketName  string `json:"bucketName"`
	Prefix      string `json:"prefix"`
	Credentials string `json:"credentials"`
}

// +kubebuilder:object:generate=true
type S3StorageSpec struct {
	BucketName string `json:"bucketName"`
	Prefix     string `json:"prefix"`
	Config     string `json:"config"`
	Profile    string `json:"profile,omitempty"`
	Endpoint   string `json:"endpoint,omitempty"`
}

// +kubebuilder:object:generate=true
type AzureStorageSpec struct {
	ContainerName string `json:"containerName"`
	Prefix        string `json:"prefix"`
	Config        string `json:"config"`
}

// +kubebuilder:object:generate=true
type InitDBMEBSpec struct {
	Storage            InitDBMEBStorage `json:"storage"`
	FullBackup         string           `json:"fullBackup"`
	IncrementalBackups []string         `json:"incrementalBackups,omitempty"`
	PITR               *PITRSpec        `json:"pitr,omitempty"`
	ExtraOptions       []string         `json:"extraOptions,omitempty"`
}

// +kubebuilder:object:generate=true
type InitDBMEBStorage struct {
	OCIObjectStorage *InitDBMEBOCIStorageSpec `json:"ociObjectStorage,omitempty"`
	S3               *InitDBMEBS3StorageSpec  `json:"s3,omitempty"`
}

// +kubebuilder:object:generate=true
type InitDBMEBOCIStorageSpec struct {
	Credentials string `json:"credentials"`
}

// +kubebuilder:object:generate=true
type InitDBMEBS3StorageSpec struct {
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	ObjectKeyPrefix string `json:"objectKeyPrefix"`
	Credentials     string `json:"credentials"`
	Host            string `json:"host,omitempty"`
}

// +kubebuilder:object:generate=true
type PITRSpec struct {
	BackupFile string   `json:"backupFile"`
	BinlogName string   `json:"binlogName,omitempty"`
	End        *PITREnd `json:"end,omitempty"`
}

// +kubebuilder:object:generate=true
type PITREnd struct {
	AfterGtids  string `json:"afterGtids,omitempty"`
	BeforeGtids string `json:"beforeGtids,omitempty"`
}

// +kubebuilder:object:generate=true
type RouterSpec struct {
	Instances        int32             `json:"instances,omitempty"`
	TLSSecretName    string            `json:"tlsSecretName,omitempty"`
	Version          string            `json:"version,omitempty"`
	PodSpec          *corev1.PodSpec   `json:"podSpec,omitempty"`
	PodAnnotations   map[string]string `json:"podAnnotations,omitempty"`
	PodLabels        map[string]string `json:"podLabels,omitempty"`
	BootstrapOptions []string          `json:"bootstrapOptions,omitempty"`
	Options          []string          `json:"options,omitempty"`
	RoutingOptions   *RoutingOptions   `json:"routingOptions,omitempty"`
}

// +kubebuilder:object:generate=true
type RoutingOptions struct {
	InvalidatedClusterPolicy string `json:"invalidated_cluster_policy,omitempty"`
	StatsUpdatesFrequency    int32  `json:"stats_updates_frequency,omitempty"`
	ReadOnlyTargets          string `json:"read_only_targets,omitempty"`
}

// +kubebuilder:object:generate=true
type ServiceConfig struct {
	Type        corev1.ServiceType `json:"type,omitempty"`
	Annotations map[string]string  `json:"annotations,omitempty"`
	Labels      map[string]string  `json:"labels,omitempty"`
	DefaultPort string             `json:"defaultPort,omitempty"`
}

// +kubebuilder:object:generate=true
type MetricsSpec struct {
	Enable      bool                            `json:"enable"`
	Image       string                          `json:"image"`
	Options     []string                        `json:"options,omitempty"`
	WebConfig   string                          `json:"webConfig,omitempty"`
	TLSSecret   string                          `json:"tlsSecret,omitempty"`
	Monitor     bool                            `json:"monitor,omitempty"`
	MonitorSpec map[string]runtime.RawExtension `json:"monitorSpec,omitempty"`
}

// +kubebuilder:object:generate=true
type BackupProfile struct {
	Name           string            `json:"name"`
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	PodLabels      map[string]string `json:"podLabels,omitempty"`
	DumpInstance   *DumpInstanceSpec `json:"dumpInstance,omitempty"`
	MEB            *MEBSpec          `json:"meb,omitempty"`
	Snapshot       *SnapshotSpec     `json:"snapshot,omitempty"`
}

// +kubebuilder:object:generate=true
type DumpInstanceSpec struct {
	DumpOptions map[string]runtime.RawExtension `json:"dumpOptions,omitempty"`
	Storage     DumpInstanceStorage             `json:"storage,omitempty"`
}

// +kubebuilder:object:generate=true
type DumpInstanceStorage struct {
	OCIObjectStorage      *OCIObjectStorageSpec             `json:"ociObjectStorage,omitempty"`
	S3                    *S3StorageSpec                    `json:"s3,omitempty"`
	Azure                 *AzureStorageSpec                 `json:"azure,omitempty"`
	PersistentVolumeClaim *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
}

// +kubebuilder:object:generate=true
type MEBSpec struct {
	Storage      MEBStorage `json:"storage"`
	ExtraOptions []string   `json:"extraOptions,omitempty"`
}

// +kubebuilder:object:generate=true
type MEBStorage struct {
	S3  *InitDBMEBS3StorageSpec `json:"s3,omitempty"`
	OCI *MEBOCIStorageSpec      `json:"oci,omitempty"`
}

// +kubebuilder:object:generate=true
type MEBOCIStorageSpec struct {
	BucketName  string `json:"bucketName"`
	Prefix      string `json:"prefix"`
	Namespace   string `json:"namespace"`
	Credentials string `json:"credentials"`
}

// +kubebuilder:object:generate=true
type SnapshotSpec struct {
	Storage DumpInstanceStorage `json:"storage,omitempty"`
}

// +kubebuilder:object:generate=true
type BackupSchedule struct {
	Name              string         `json:"name"`
	Schedule          string         `json:"schedule"`
	BackupProfileName string         `json:"backupProfileName,omitempty"`
	BackupProfile     *BackupProfile `json:"backupProfile,omitempty"`
	DeleteBackupData  bool           `json:"deleteBackupData,omitempty"`
	Enabled           bool           `json:"enabled,omitempty"`
	TimeZone          string         `json:"timeZone,omitempty"`
}

// +kubebuilder:object:generate=true
type LogsSpec struct {
	General   *LogConfig      `json:"general,omitempty"`
	Error     *ErrorLogConfig `json:"error,omitempty"`
	SlowQuery *SlowLogConfig  `json:"slowQuery,omitempty"`
	Collector *CollectorSpec  `json:"collector,omitempty"`
}

// +kubebuilder:object:generate=true
type LogConfig struct {
	Enabled bool `json:"enabled,omitempty"`
	Collect bool `json:"collect,omitempty"`
}

// +kubebuilder:object:generate=true
type ErrorLogConfig struct {
	Collect   bool  `json:"collect,omitempty"`
	Verbosity int32 `json:"verbosity,omitempty"`
}

// +kubebuilder:object:generate=true
type SlowLogConfig struct {
	Enabled       bool    `json:"enabled,omitempty"`
	LongQueryTime float64 `json:"longQueryTime,omitempty"`
	Collect       bool    `json:"collect,omitempty"`
}

// +kubebuilder:object:generate=true
type CollectorSpec struct {
	Image         string          `json:"image,omitempty"`
	ContainerName string          `json:"containerName,omitempty"`
	Env           []corev1.EnvVar `json:"env,omitempty"`
	Fluentd       *FluentdSpec    `json:"fluentd,omitempty"`
}

// +kubebuilder:object:generate=true
type FluentdSpec struct {
	GeneralLog                    *FluentdLogConfig   `json:"generalLog,omitempty"`
	ErrorLog                      *FluentdLogConfig   `json:"errorLog,omitempty"`
	SlowQueryLog                  *FluentdLogConfig   `json:"slowQueryLog,omitempty"`
	RecordAugmentation            *RecordAugmentation `json:"recordAugmentation,omitempty"`
	AdditionalFilterConfiguration string              `json:"additionalFilterConfiguration,omitempty"`
	Sinks                         []FluentdSink       `json:"sinks,omitempty"`
}

// +kubebuilder:object:generate=true
type FluentdLogConfig struct {
	Tag     string                          `json:"tag,omitempty"`
	Options map[string]runtime.RawExtension `json:"options,omitempty"`
}

// +kubebuilder:object:generate=true
type RecordAugmentation struct {
	Enabled        bool                        `json:"enabled,omitempty"`
	Labels         []LabelAugmentation         `json:"labels,omitempty"`
	Annotations    []AnnotationAugmentation    `json:"annotations,omitempty"`
	StaticFields   []StaticFieldAugmentation   `json:"staticFields,omitempty"`
	PodFields      []PodFieldAugmentation      `json:"podFields,omitempty"`
	ResourceFields []ResourceFieldAugmentation `json:"resourceFields,omitempty"`
}

// +kubebuilder:object:generate=true
type LabelAugmentation struct {
	FieldName string `json:"fieldName"`
	LabelName string `json:"labelName"`
}

// +kubebuilder:object:generate=true
type AnnotationAugmentation struct {
	FieldName      string `json:"fieldName"`
	AnnotationName string `json:"annotationName"`
}

// +kubebuilder:object:generate=true
type StaticFieldAugmentation struct {
	FieldName  string `json:"fieldName"`
	FieldValue string `json:"fieldValue"`
}

// +kubebuilder:object:generate=true
type PodFieldAugmentation struct {
	FieldName string `json:"fieldName"`
	FieldPath string `json:"fieldPath"`
}

// +kubebuilder:object:generate=true
type ResourceFieldAugmentation struct {
	FieldName     string `json:"fieldName"`
	ContainerName string `json:"containerName"`
	Resource      string `json:"resource"`
}

// +kubebuilder:object:generate=true
type FluentdSink struct {
	Name      string `json:"name"`
	RawConfig string `json:"rawConfig"`
}

// +kubebuilder:object:generate=true
type ReadReplicaSpec struct {
	Name                       string                        `json:"name"`
	Version                    string                        `json:"version,omitempty"`
	BaseServerId               int32                         `json:"baseServerId,omitempty"`
	DatadirVolumeClaimTemplate *corev1.PersistentVolumeClaim `json:"datadirVolumeClaimTemplate,omitempty"`
	Mycnf                      string                        `json:"mycnf,omitempty"`
	Instances                  int32                         `json:"instances,omitempty"`
	PodSpec                    *corev1.PodSpec               `json:"podSpec,omitempty"`
	PodAnnotations             map[string]string             `json:"podAnnotations,omitempty"`
	PodLabels                  map[string]string             `json:"podLabels,omitempty"`
}

// InnoDBClusterStatus defines the observed state of InnoDBCluster
// +kubebuilder:object:generate=true
type InnoDBClusterStatus struct {
	// Status contains the observed state as a raw extension since it's preserved-unknown-fields
	runtime.RawExtension `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// InnoDBCluster is the Schema for the innodbclusters API
// +kubebuilder:object:generate=true
type InnoDBCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InnoDBClusterSpec   `json:"spec,omitempty"`
	Status InnoDBClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InnoDBClusterList contains a list of InnoDBCluster
// +kubebuilder:object:generate=true
type InnoDBClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InnoDBCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InnoDBCluster{}, &InnoDBClusterList{})
}
