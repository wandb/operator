/*
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

package v1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Constants
const (
	GRPCPortDelta = 10000

	MasterHTTPPort    = 9333
	VolumeHTTPPort    = 8444
	FilerHTTPPort     = 8888
	FilerS3Port       = 8333
	FilerIcebergPort  = 8181
	AdminHTTPPort     = 23646
	WorkerMetricsPort = 9101

	MasterGRPCPort = MasterHTTPPort + GRPCPortDelta
	VolumeGRPCPort = VolumeHTTPPort + GRPCPortDelta
	FilerGRPCPort  = FilerHTTPPort + GRPCPortDelta
	AdminGRPCPort  = AdminHTTPPort + GRPCPortDelta
)

type IngressSpec struct {
	Enabled     bool              `json:"enabled,omitempty"`
	ClassName   *string           `json:"className,omitempty"`
	Host        string            `json:"host,omitempty"`
	Path        string            `json:"path,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	TLS         []IngressTLS      `json:"tls,omitempty"`
}

type IngressTLS struct {
	Hosts      []string `json:"hosts,omitempty"`
	SecretName string   `json:"secretName,omitempty"`
}

type TLSSpec struct {
	Enabled   bool          `json:"enabled,omitempty"`
	IssuerRef *TLSIssuerRef `json:"issuerRef,omitempty"`
}

type TLSIssuerRef struct {
	Name  string `json:"name"`
	Kind  string `json:"kind,omitempty"`
	Group string `json:"group,omitempty"`
}


type SeaweedSpec struct {
	TLS            *TLSSpec `json:"tls,omitempty"`
	MetricsAddress string   `json:"metricsAddress,omitempty"`
	Image          string   `json:"image,omitempty"`
	Version        string   `json:"version,omitempty"`

	Master         *MasterSpec                    `json:"master,omitempty"`
	Volume         *VolumeSpec                    `json:"volume,omitempty"`
	VolumeTopology map[string]*VolumeTopologySpec `json:"volumeTopology,omitempty"`
	Filer          *FilerSpec                     `json:"filer,omitempty"`
	Admin          *AdminSpec                     `json:"admin,omitempty"`
	Worker         *WorkerSpec                    `json:"worker,omitempty"`
	S3             *S3GatewaySpec                 `json:"s3,omitempty"`

	SchedulerName             string                                `json:"schedulerName,omitempty"`
	PVReclaimPolicy           *corev1.PersistentVolumeReclaimPolicy `json:"pvReclaimPolicy,omitempty"`
	ImagePullPolicy           corev1.PullPolicy                     `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets          []corev1.LocalObjectReference         `json:"imagePullSecrets,omitempty"`
	EnablePVReclaim           *bool                                 `json:"enablePVReclaim,omitempty"`
	HostNetwork               *bool                                 `json:"hostNetwork,omitempty"`
	Affinity                  *corev1.Affinity                      `json:"affinity,omitempty"`
	NodeSelector              map[string]string                     `json:"nodeSelector,omitempty"`
	Annotations               map[string]string                     `json:"annotations,omitempty"`
	Tolerations               []corev1.Toleration                   `json:"tolerations,omitempty"`
	StatefulSetUpdateStrategy appsv1.StatefulSetUpdateStrategyType  `json:"statefulSetUpdateStrategy,omitempty"`
	VolumeServerDiskCount     *int32                                `json:"volumeServerDiskCount,omitempty"`
	HostSuffix                *string                               `json:"hostSuffix,omitempty"`
}

// SeaweedStatus defines the observed state of Seaweed
type SeaweedStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	Master             ComponentStatus    `json:"master,omitempty"`
	Volume             ComponentStatus    `json:"volume,omitempty"`
	Filer              ComponentStatus    `json:"filer,omitempty"`
	Admin              ComponentStatus    `json:"admin,omitempty"`
	Worker             ComponentStatus    `json:"worker,omitempty"`
	S3                 ComponentStatus    `json:"s3,omitempty"`
}

// ComponentStatus represents the status of a seaweedfs component
type ComponentStatus struct {
	Replicas      int32 `json:"replicas,omitempty"`
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// MasterSpec is the spec for masters
type MasterSpec struct {
	ComponentSpec               `json:",inline"`
	corev1.ResourceRequirements `json:",inline"`

	Replicas int32        `json:"replicas"`
	Service  *ServiceSpec `json:"service,omitempty"`
	Config   *string      `json:"config,omitempty"`

	MetricsPort        *int32       `json:"metricsPort,omitempty"`
	VolumePreallocate  *bool        `json:"volumePreallocate,omitempty"`
	VolumeSizeLimitMB  *int32       `json:"volumeSizeLimitMB,omitempty"`
	GarbageThreshold   *string      `json:"garbageThreshold,omitempty"`
	PulseSeconds       *int32       `json:"pulseSeconds,omitempty"`
	DefaultReplication *string      `json:"defaultReplication,omitempty"`
	ConcurrentStart    *bool        `json:"concurrentStart,omitempty"`
	Ingress            *IngressSpec `json:"ingress,omitempty"`
}

type VolumeServerConfig struct {
	ComponentSpec               `json:",inline"`
	corev1.ResourceRequirements `json:",inline"`

	Service          *ServiceSpec `json:"service,omitempty"`
	StorageClassName *string      `json:"storageClassName,omitempty"`

	MetricsPort         *int32 `json:"metricsPort,omitempty"`
	CompactionMBps      *int32 `json:"compactionMBps,omitempty"`
	FileSizeLimitMB     *int32 `json:"fileSizeLimitMB,omitempty"`
	FixJpgOrientation   *bool  `json:"fixJpgOrientation,omitempty"`
	IdleTimeout         *int32 `json:"idleTimeout,omitempty"`
	MaxVolumeCounts     *int32 `json:"maxVolumeCounts,omitempty"`
	MinFreeSpacePercent *int32 `json:"minFreeSpacePercent,omitempty"`
}

// VolumeSpec is the spec for volume servers
type VolumeSpec struct {
	VolumeServerConfig `json:",inline"`

	Replicas   int32        `json:"replicas"`
	Rack       *string      `json:"rack,omitempty"`
	DataCenter *string      `json:"dataCenter,omitempty"`
	Ingress    *IngressSpec `json:"ingress,omitempty"`
}

// VolumeTopologySpec defines a volume server group with specific topology placement
type VolumeTopologySpec struct {
	VolumeServerConfig `json:",inline"`

	Replicas   int32  `json:"replicas"`
	Rack       string `json:"rack"`
	DataCenter string `json:"dataCenter"`
}

// S3Config defines the S3 configuration (deprecated: prefer S3GatewaySpec)
type S3Config struct {
	Enabled      bool                      `json:"enabled,omitempty"`
	ConfigSecret *corev1.SecretKeySelector `json:"configSecret,omitempty"`
}

// S3GatewaySpec defines a standalone S3 gateway Deployment
type S3GatewaySpec struct {
	ComponentSpec               `json:",inline"`
	corev1.ResourceRequirements `json:",inline"`

	Replicas     int32                     `json:"replicas"`
	Service      *ServiceSpec              `json:"service,omitempty"`
	ConfigSecret *corev1.SecretKeySelector `json:"configSecret,omitempty"`
	MetricsPort  *int32                    `json:"metricsPort,omitempty"`
	Port         *int32                    `json:"port,omitempty"`
	DomainName   *string                   `json:"domainName,omitempty"`
	IAM          bool                      `json:"iam,omitempty"`
	Ingress      *IngressSpec              `json:"ingress,omitempty"`
}

// IcebergConfig defines the Iceberg catalog REST API configuration
type IcebergConfig struct {
	Enabled bool   `json:"enabled,omitempty"`
	Port    *int32 `json:"port,omitempty"`
}

// IcebergEffectivePort returns the port to use for the Iceberg catalog REST API.
func (c *IcebergConfig) IcebergEffectivePort() int32 {
	if c.Port != nil {
		return *c.Port
	}
	return FilerIcebergPort
}

// FilerSpec is the spec for filers
type FilerSpec struct {
	ComponentSpec               `json:",inline"`
	corev1.ResourceRequirements `json:",inline"`

	Replicas    int32            `json:"replicas"`
	Service     *ServiceSpec     `json:"service,omitempty"`
	Config      *string          `json:"config,omitempty"`
	MetricsPort *int32           `json:"metricsPort,omitempty"`
	Persistence *PersistenceSpec `json:"persistence,omitempty"`

	MaxMB     *int32         `json:"maxMB,omitempty"`
	S3        *S3Config      `json:"s3,omitempty"`
	IAM       bool           `json:"iam,omitempty"`
	Iceberg   *IcebergConfig `json:"iceberg,omitempty"`
	Ingress   *IngressSpec   `json:"ingress,omitempty"`
	S3Ingress *IngressSpec   `json:"s3Ingress,omitempty"`
}

// AdminSpec is the spec for the admin server
type AdminSpec struct {
	ComponentSpec               `json:",inline"`
	corev1.ResourceRequirements `json:",inline"`

	Service           *ServiceSpec                 `json:"service,omitempty"`
	MetricsPort       *int32                       `json:"metricsPort,omitempty"`
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
	Ingress           *IngressSpec                 `json:"ingress,omitempty"`
}

// WorkerSpec is the spec for worker processes
type WorkerSpec struct {
	ComponentSpec               `json:",inline"`
	corev1.ResourceRequirements `json:",inline"`

	Replicas    int32            `json:"replicas"`
	MetricsPort *int32           `json:"metricsPort,omitempty"`
	Persistence *PersistenceSpec `json:"persistence,omitempty"`
	JobType     *string          `json:"jobType,omitempty"`
	MaxDetect   *int32           `json:"maxDetect,omitempty"`
	MaxExecute  *int32           `json:"maxExecute,omitempty"`
}

// ComponentSpec is the base spec of each component
type ComponentSpec struct {
	Version                       *string                              `json:"version,omitempty"`
	ImagePullPolicy               *corev1.PullPolicy                   `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets              []corev1.LocalObjectReference        `json:"imagePullSecrets,omitempty"`
	HostNetwork                   *bool                                `json:"hostNetwork,omitempty"`
	Affinity                      *corev1.Affinity                     `json:"affinity,omitempty"`
	PriorityClassName             *string                              `json:"priorityClassName,omitempty"`
	SchedulerName                 *string                              `json:"schedulerName,omitempty"`
	NodeSelector                  map[string]string                    `json:"nodeSelector,omitempty"`
	Annotations                   map[string]string                    `json:"annotations,omitempty"`
	Tolerations                   []corev1.Toleration                  `json:"tolerations,omitempty"`
	Env                           []corev1.EnvVar                      `json:"env,omitempty"`
	TerminationGracePeriodSeconds *int64                               `json:"terminationGracePeriodSeconds,omitempty"`
	StatefulSetUpdateStrategy     appsv1.StatefulSetUpdateStrategyType `json:"statefulSetUpdateStrategy,omitempty"`
	Volumes                       []corev1.Volume                      `json:"volumes,omitempty"`
	VolumeMounts                  []corev1.VolumeMount                 `json:"volumeMounts,omitempty"`
	ExtraArgs                     []string                             `json:"extraArgs,omitempty"`
}

// ServiceSpec is a subset of the original k8s spec
type ServiceSpec struct {
	Type           corev1.ServiceType `json:"type,omitempty"`
	Annotations    map[string]string  `json:"annotations,omitempty"`
	LoadBalancerIP *string            `json:"loadBalancerIP,omitempty"`
	ClusterIP      *string            `json:"clusterIP,omitempty"`
}

type PersistenceSpec struct {
	Enabled          bool                                `json:"enabled,omitempty"`
	ExistingClaim    *string                             `json:"existingClaim,omitempty"`
	MountPath        *string                             `json:"mountPath,omitempty"`
	SubPath          *string                             `json:"subPath,omitempty"`
	AccessModes      []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	Selector         *metav1.LabelSelector               `json:"selector,omitempty"`
	Resources        corev1.VolumeResourceRequirements   `json:"resources,omitempty"`
	VolumeName       string                              `json:"volumeName,omitempty"`
	StorageClassName *string                             `json:"storageClassName,omitempty"`
	VolumeMode       *corev1.PersistentVolumeMode        `json:"volumeMode,omitempty"`
	DataSource       *corev1.TypedLocalObjectReference   `json:"dataSource,omitempty"`
}

// Seaweed is the Schema for the seaweeds API
type Seaweed struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SeaweedSpec   `json:"spec,omitempty"`
	Status SeaweedStatus `json:"status,omitempty"`
}

// SeaweedList contains a list of Seaweed
type SeaweedList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Seaweed `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Seaweed{}, &SeaweedList{})
}
