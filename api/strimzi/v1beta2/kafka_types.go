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

package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KafkaSpec defines the desired state of Kafka
type KafkaSpec struct {
	Kafka          KafkaClusterSpec    `json:"kafka"`
	ZooKeeper      *ZooKeeperSpec      `json:"zookeeper,omitempty"`
	EntityOperator *EntityOperatorSpec `json:"entityOperator,omitempty"`
}

// KafkaClusterSpec defines the Kafka cluster configuration
type KafkaClusterSpec struct {
	Version         string                 `json:"version,omitempty"`
	MetadataVersion string                 `json:"metadataVersion,omitempty"`
	Replicas        int32                  `json:"replicas,omitempty"`
	Listeners       []GenericKafkaListener `json:"listeners,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	Config    map[string]string            `json:"config,omitempty"`
	Storage   *KafkaStorage                `json:"storage,omitempty"`
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	Rack      *Rack                        `json:"rack,omitempty"`
	Template  *KafkaClusterTemplate        `json:"template,omitempty"`
}

// GenericKafkaListener represents a Kafka listener configuration
type GenericKafkaListener struct {
	Name           string                             `json:"name"`
	Port           int32                              `json:"port"`
	Type           string                             `json:"type"`
	Tls            bool                               `json:"tls"`
	Authentication *KafkaListenerAuthentication       `json:"authentication,omitempty"`
	Configuration  *GenericKafkaListenerConfiguration `json:"configuration,omitempty"`
}

// KafkaListenerAuthentication defines authentication configuration
type KafkaListenerAuthentication struct {
	Type string `json:"type"`
}

// GenericKafkaListenerConfiguration defines additional listener configuration
type GenericKafkaListenerConfiguration struct {
	Bootstrap      *KafkaListenerConfigurationBootstrap `json:"bootstrap,omitempty"`
	Brokers        []KafkaListenerConfigurationBroker   `json:"brokers,omitempty"`
	IPFamilyPolicy string                               `json:"ipFamilyPolicy,omitempty"`
	IPFamilies     []string                             `json:"ipFamilies,omitempty"`
}

// KafkaListenerConfigurationBootstrap defines bootstrap configuration
type KafkaListenerConfigurationBootstrap struct {
	AlternativeNames []string          `json:"alternativeNames,omitempty"`
	Host             string            `json:"host,omitempty"`
	NodePort         int32             `json:"nodePort,omitempty"`
	LoadBalancerIP   string            `json:"loadBalancerIP,omitempty"`
	Annotations      map[string]string `json:"annotations,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
}

// KafkaListenerConfigurationBroker defines per-broker configuration
type KafkaListenerConfigurationBroker struct {
	Broker         int32             `json:"broker"`
	AdvertisedHost string            `json:"advertisedHost,omitempty"`
	AdvertisedPort int32             `json:"advertisedPort,omitempty"`
	Host           string            `json:"host,omitempty"`
	NodePort       int32             `json:"nodePort,omitempty"`
	LoadBalancerIP string            `json:"loadBalancerIP,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

// KafkaStorage defines the storage configuration for Kafka
type KafkaStorage struct {
	Type        string                `json:"type"`
	Size        string                `json:"size,omitempty"`
	Class       string                `json:"class,omitempty"`
	DeleteClaim bool                  `json:"deleteClaim,omitempty"`
	Selector    map[string]string     `json:"selector,omitempty"`
	Volumes     []corev1.Volume       `json:"volumes,omitempty"`
	Kraft       *KRaftMetadataStorage `json:"kraftMetadata,omitempty"`
}

// KRaftMetadataStorage defines KRaft metadata storage
type KRaftMetadataStorage struct {
	Type  string `json:"type"`
	Size  string `json:"size,omitempty"`
	Class string `json:"class,omitempty"`
}

// Rack defines rack awareness configuration
type Rack struct {
	TopologyKey string `json:"topologyKey"`
}

// KafkaClusterTemplate defines template configuration
type KafkaClusterTemplate struct {
	StatefulSet              *StatefulSetTemplate `json:"statefulset,omitempty"`
	Pod                      *PodTemplate         `json:"pod,omitempty"`
	BootstrapService         *ResourceTemplate    `json:"bootstrapService,omitempty"`
	BrokersService           *ResourceTemplate    `json:"brokersService,omitempty"`
	ExternalBootstrapService *ResourceTemplate    `json:"externalBootstrapService,omitempty"`
	PerPodService            *ResourceTemplate    `json:"perPodService,omitempty"`
}

// StatefulSetTemplate defines StatefulSet template
type StatefulSetTemplate struct {
	Metadata            *MetadataTemplate `json:"metadata,omitempty"`
	PodManagementPolicy string            `json:"podManagementPolicy,omitempty"`
}

// PodTemplate defines Pod template
type PodTemplate struct {
	Metadata                      *MetadataTemplate                 `json:"metadata,omitempty"`
	ImagePullSecrets              []corev1.LocalObjectReference     `json:"imagePullSecrets,omitempty"`
	SecurityContext               *corev1.PodSecurityContext        `json:"securityContext,omitempty"`
	TerminationGracePeriodSeconds *int64                            `json:"terminationGracePeriodSeconds,omitempty"`
	Affinity                      *corev1.Affinity                  `json:"affinity,omitempty"`
	Tolerations                   []corev1.Toleration               `json:"tolerations,omitempty"`
	PriorityClassName             string                            `json:"priorityClassName,omitempty"`
	SchedulerName                 string                            `json:"schedulerName,omitempty"`
	HostAliases                   []corev1.HostAlias                `json:"hostAliases,omitempty"`
	EnableServiceLinks            *bool                             `json:"enableServiceLinks,omitempty"`
	TopologySpreadConstraints     []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// ResourceTemplate defines resource template
type ResourceTemplate struct {
	Metadata *MetadataTemplate `json:"metadata,omitempty"`
}

// MetadataTemplate defines metadata template
type MetadataTemplate struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ZooKeeperSpec defines the ZooKeeper configuration
type ZooKeeperSpec struct {
	Replicas int32        `json:"replicas,omitempty"`
	Storage  KafkaStorage `json:"storage"`
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	Config    map[string]string            `json:"config,omitempty"`
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	Template  *ZooKeeperClusterTemplate    `json:"template,omitempty"`
}

// ZooKeeperClusterTemplate defines ZooKeeper template configuration
type ZooKeeperClusterTemplate struct {
	StatefulSet   *StatefulSetTemplate `json:"statefulset,omitempty"`
	Pod           *PodTemplate         `json:"pod,omitempty"`
	ClientService *ResourceTemplate    `json:"clientService,omitempty"`
	NodesService  *ResourceTemplate    `json:"nodesService,omitempty"`
}

// EntityOperatorSpec defines the Entity Operator configuration
type EntityOperatorSpec struct {
	TopicOperator *EntityTopicOperatorSpec `json:"topicOperator,omitempty"`
	UserOperator  *EntityUserOperatorSpec  `json:"userOperator,omitempty"`
	Template      *EntityOperatorTemplate  `json:"template,omitempty"`
}

// EntityTopicOperatorSpec defines Topic Operator configuration
type EntityTopicOperatorSpec struct {
	WatchedNamespace string                       `json:"watchedNamespace,omitempty"`
	Image            string                       `json:"image,omitempty"`
	Resources        *corev1.ResourceRequirements `json:"resources,omitempty"`
	Logging          *EntityOperatorLogging       `json:"logging,omitempty"`
}

// EntityUserOperatorSpec defines User Operator configuration
type EntityUserOperatorSpec struct {
	WatchedNamespace string                       `json:"watchedNamespace,omitempty"`
	Image            string                       `json:"image,omitempty"`
	Resources        *corev1.ResourceRequirements `json:"resources,omitempty"`
	Logging          *EntityOperatorLogging       `json:"logging,omitempty"`
}

// EntityOperatorLogging defines logging configuration
type EntityOperatorLogging struct {
	Type string `json:"type,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	LoggerName map[string]string `json:"loggers,omitempty"`
}

// EntityOperatorTemplate defines Entity Operator template
type EntityOperatorTemplate struct {
	Deployment             *ResourceTemplate  `json:"deployment,omitempty"`
	Pod                    *PodTemplate       `json:"pod,omitempty"`
	TopicOperatorContainer *ContainerTemplate `json:"topicOperatorContainer,omitempty"`
	UserOperatorContainer  *ContainerTemplate `json:"userOperatorContainer,omitempty"`
	TlsSidecarContainer    *ContainerTemplate `json:"tlsSidecarContainer,omitempty"`
	ServiceAccount         *ResourceTemplate  `json:"serviceAccount,omitempty"`
}

// ContainerTemplate defines container template
type ContainerTemplate struct {
	Env             []corev1.EnvVar         `json:"env,omitempty"`
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}

// KafkaStatus defines the observed state of Kafka
type KafkaStatus struct {
	Conditions                    []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration            int64              `json:"observedGeneration,omitempty"`
	Listeners                     []ListenerStatus   `json:"listeners,omitempty"`
	ClusterId                     string             `json:"clusterId,omitempty"`
	KafkaMetadataState            string             `json:"kafkaMetadataState,omitempty"`
	KafkaVersion                  string             `json:"kafkaVersion,omitempty"`
	KafkaMetadataVersion          string             `json:"kafkaMetadataVersion,omitempty"`
	OperatorLastSuccessfulVersion string             `json:"operatorLastSuccessfulVersion,omitempty"`
}

// ListenerStatus defines the status of a listener
type ListenerStatus struct {
	Name             string            `json:"name,omitempty"`
	Addresses        []ListenerAddress `json:"addresses,omitempty"`
	BootstrapServers string            `json:"bootstrapServers,omitempty"`
	Certificates     []string          `json:"certificates,omitempty"`
}

// ListenerAddress defines an address for a listener
type ListenerAddress struct {
	Host string `json:"host,omitempty"`
	Port int32  `json:"port,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kafkas,scope=Namespaced,categories=strimzi

// Kafka is the Schema for the kafkas API
type Kafka struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaSpec   `json:"spec,omitempty"`
	Status KafkaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KafkaList contains a list of Kafka
type KafkaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kafka `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kafka{}, &KafkaList{})
}
