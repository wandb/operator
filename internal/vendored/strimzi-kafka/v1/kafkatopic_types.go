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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KafkaTopicSpec defines the desired state of KafkaTopic
type KafkaTopicSpec struct {
	// TopicName is the name of the topic.
	// When absent this will default to the metadata.name of the KafkaTopic.
	TopicName string `json:"topicName,omitempty"`

	// Partitions is the number of partitions the topic should have.
	// This cannot be decreased after topic creation.
	// It can be increased after topic creation, but it is important to understand the consequences that has, especially for topics with semantic partitioning.
	// When absent this will default to the broker configuration for `num.partitions`.
	Partitions int32 `json:"partitions,omitempty"`

	// Replicas is the number of replicas the topic should have.
	// When absent this will default to the broker configuration for `default.replication.factor`.
	Replicas int32 `json:"replicas,omitempty"`

	// Config is the topic configuration.
	Config map[string]string `json:"config,omitempty"`
}

// ReplicasChange describes the replica count change state
type ReplicasChange struct {
	// TargetReplicas is the target number of replicas requested by the user.
	TargetReplicas int32 `json:"targetReplicas,omitempty"`

	// State is the state of the replica change operation.
	// Possible values: pending, ongoing
	State string `json:"state,omitempty"`

	// Message is a human-readable message about the state of the replica change operation.
	Message string `json:"message,omitempty"`

	// SessionId is the session identifier for the replica change operation.
	SessionId string `json:"sessionId,omitempty"`
}

// KafkaTopicStatus defines the observed state of KafkaTopic
type KafkaTopicStatus struct {
	// Conditions is the list of status conditions for this resource.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the CRD that was last reconciled by the operator.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// TopicName is the name of the topic.
	TopicName string `json:"topicName,omitempty"`

	// TopicId is the topic's id.
	// For a KafkaTopic with the ready condition, this will change only if the topic gets deleted and recreated with the same name.
	TopicId string `json:"topicId,omitempty"`

	// ReplicasChange describes the state of the change in the number of replicas.
	// When absent it means that the current replication factor of the topic is the same as the one requested in the spec.
	ReplicasChange *ReplicasChange `json:"replicasChange,omitempty"`
}

// KafkaTopic is the Schema for the kafkatopics API
type KafkaTopic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaTopicSpec   `json:"spec,omitempty"`
	Status KafkaTopicStatus `json:"status,omitempty"`
}

// KafkaTopicList contains a list of KafkaTopic
type KafkaTopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaTopic `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaTopic{}, &KafkaTopicList{})
}
