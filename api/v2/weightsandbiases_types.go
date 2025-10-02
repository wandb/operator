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

type WBStreamingType string

const (
	WBKafkaStreaming WBStreamingType = "kafka"
)

type WBPhaseType string

const (
	WBPhaseReady       WBPhaseType = "Ready"
	WBPhaseError       WBPhaseType = "Error"
	WBPhaseReconciling WBPhaseType = "Reconciling"
	WBPhaseDeleting    WBPhaseType = "Deleting"
)

type WBInfraStatusType string

const (
	WBInfraStatusReady    WBInfraStatusType = "Ready"
	WBInfraStatusError    WBInfraStatusType = "Error"
	WBInfraStatusPending  WBInfraStatusType = "Pending"
	WBInfraStatusMissing  WBInfraStatusType = "Missing"
	WBInfraStatusDeleting WBInfraStatusType = "Deleting"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=wandb

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

type ConfigMapReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Key       string `json:"key,omitempty"`
}

// WeightsAndBiasesSpec defines the desired state of WeightsAndBiases.
type WeightsAndBiasesSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Profile is akin to high-level environment info
	Profile string `json:"profile,omitempty"`

	Infra WBInfraSpec `json:"infra,omitempty"`
}

type WBStreamingSpec struct {
	Enabled      bool                `json:"enabled"`
	Type         WBStreamingType     `json:"type"`
	ConfigMapRef *ConfigMapReference `json:"configMapRef,omitempty"`
}

type WBInfraSpec struct {
	Streaming WBStreamingSpec `json:"streaming,omitempty"`
}

type WBStreamingStatus struct {
	Ready                bool              `json:"ready"`
	ReconciliationStatus WBInfraStatusType `json:"reconciliationStatus,omitempty" default:"Missing"`
	LastReconciled       metav1.Time       `json:"lastReconciled,omitempty"`
}

type WBInfraStatus struct {
	StreamingStatus WBStreamingStatus `json:"streamingStatus,omitempty"`
}

type WBComponentStatus struct {
	InfraStatus WBInfraStatus `json:"infraStatus,omitempty"`
}

// WeightsAndBiasesStatus defines the observed state of WeightsAndBiases.
type WeightsAndBiasesStatus struct {
	Phase              WBPhaseType       `json:"phase,omitempty"`
	ComponentStatus    WBComponentStatus `json:"componentStatus,omitempty"`
	ObservedGeneration int64             `json:"observedGeneration"`
}
