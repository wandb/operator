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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KafkaNodePoolSpec defines the desired state of KafkaNodePool
type KafkaNodePoolSpec struct {
	Replicas   int32                        `json:"replicas"`
	Roles      []string                     `json:"roles"`
	Storage    KafkaStorage                 `json:"storage"`
	Resources  *corev1.ResourceRequirements `json:"resources,omitempty"`
	JvmOptions *JvmOptions                  `json:"jvmOptions,omitempty"`
	Template   *KafkaNodePoolTemplate       `json:"template,omitempty"`
}

// JvmOptions defines JVM configuration options
type JvmOptions struct {
	Xms                  string            `json:"-Xms,omitempty"`
	Xmx                  string            `json:"-Xmx,omitempty"`
	XX                   map[string]string `json:"-XX,omitempty"`
	JavaSystemProperties []SystemProperty  `json:"javaSystemProperties,omitempty"`
	GcLoggingEnabled     bool              `json:"gcLoggingEnabled,omitempty"`
}

// SystemProperty defines a Java system property
type SystemProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// KafkaNodePoolTemplate defines template configuration for node pool
type KafkaNodePoolTemplate struct {
	Pod                   *PodTemplate       `json:"pod,omitempty"`
	PerPodService         *ResourceTemplate  `json:"perPodService,omitempty"`
	PersistentVolumeClaim *ResourceTemplate  `json:"persistentVolumeClaim,omitempty"`
	PodSet                *PodSetTemplate    `json:"podSet,omitempty"`
	InitContainer         *ContainerTemplate `json:"initContainer,omitempty"`
	KafkaContainer        *ContainerTemplate `json:"kafkaContainer,omitempty"`
}

// PodSetTemplate defines pod set template configuration
type PodSetTemplate struct {
	Metadata *MetadataTemplate `json:"metadata,omitempty"`
}

// KafkaNodePoolStatus defines the observed state of KafkaNodePool
type KafkaNodePoolStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	NodeIds            []int32            `json:"nodeIds,omitempty"`
	ClusterId          string             `json:"clusterId,omitempty"`
	Replicas           int32              `json:"replicas,omitempty"`
	LabelSelector      string             `json:"labelSelector,omitempty"`
	Roles              []string           `json:"roles,omitempty"`
}

// KafkaNodePool is the Schema for the kafkanodepools API
type KafkaNodePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaNodePoolSpec   `json:"spec,omitempty"`
	Status KafkaNodePoolStatus `json:"status,omitempty"`
}

// KafkaNodePoolList contains a list of KafkaNodePool
type KafkaNodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaNodePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaNodePool{}, &KafkaNodePoolList{})
}
