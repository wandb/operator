/*
Copyright 2023.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN! NOTE: json tags are
// required.  Any new fields you add must have json tags for the fields to be
// serialized.

// WeightsAndBiasesSpec defines the desired state of WeightsAndBiases
type WeightsAndBiasesSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster Important: Run
	// "make" to regenerate code after modifying this file

	// The specification of Weights & Biases Chart that is used to deploy the
	// instance.

	Cdk8sVersion string `json:"version,omitempty"`
	ReleasePath  string `json:"releasePath,omitempty"`
}

// WeightsAndBiasesStatus defines the observed state of WeightsAndBiases
type WeightsAndBiasesStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Phase   string `json:"phase,omitempty"`
	Version string `json:"version,omitempty"`
	// Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=wandb
//+operator-sdk:csv:customresourcedefinitions:displayName="Weights & Biases"

// WeightsAndBiases is the Schema for the weightsandbiases API
type WeightsAndBiases struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WeightsAndBiasesSpec   `json:"spec,omitempty"`
	Status WeightsAndBiasesStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WeightsAndBiasesList contains a list of WeightsAndBiases
type WeightsAndBiasesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WeightsAndBiases `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WeightsAndBiases{}, &WeightsAndBiasesList{})
}
