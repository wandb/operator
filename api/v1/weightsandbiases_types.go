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
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
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

	// +kubebuilder:validation:Optional
	Version string `json:"version,omitempty"`
	// +kubebuilder:validation:Optional
	License string `json:"license,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Config ConfigValues `json:"config,omitempty"`
}

// Unstructured values for rendering CDK8s Config.
// +k8s:deepcopy-gen=false
type ConfigValues struct {
	// Object is a JSON compatible map with string, float, int, bool, []interface{}, or
	// map[string]interface{} children.
	Object map[string]interface{} `json:"-"`
}

// MarshalJSON ensures that the unstructured object produces proper
// JSON when passed to Go's standard JSON library.
func (u *ConfigValues) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.Object)
}

// UnmarshalJSON ensures that the unstructured object properly decodes
// JSON when passed to Go's standard JSON library.
func (u *ConfigValues) UnmarshalJSON(data []byte) error {
	m := make(map[string]interface{})
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	u.Object = m

	return nil
}

// Declaring this here prevents it from being generated.
func (u *ConfigValues) DeepCopyInto(out *ConfigValues) {
	out.Object = runtime.DeepCopyJSON(u.Object)
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
