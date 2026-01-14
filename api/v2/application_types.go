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
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/wandb/operator/pkg/vendored/argo-rollouts/argoproj.io.rollouts/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ApplicationSpec defines the desired state of Application.
type ApplicationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Kind string `json:"kind,omitempty"`

	// Replicas is the number of desired instances of the application.
	// This field is ignored if HpaTemplate is provided.
	Replicas *int32 `json:"replicas,omitempty"`

	MetaTemplate         metav1.ObjectMeta                          `json:"metaTemplate,omitempty"`
	PodTemplate          corev1.PodTemplateSpec                     `json:"podTemplate,omitempty"`
	ServiceTemplate      *corev1.ServiceSpec                        `json:"serviceTemplate,omitempty"`
	IngressTemplate      *networkingv1.IngressSpec                  `json:"ingressTemplate,omitempty"`
	HpaTemplate          *autoscalingv1.HorizontalPodAutoscalerSpec `json:"hpaTemplate,omitempty"`
	PdbTemplate          *policyv1.PodDisruptionBudgetSpec          `json:"pdbTemplate,omitempty"`
	ScaledObjectTemplate *kedav1alpha1.ScaledObjectSpec             `json:"scaledObjectTemplate,omitempty"`
	Jobs                 []batchv1.Job                              `json:"jobs,omitempty"`
	CronJobs             []batchv1.CronJob                          `json:"cronJobs,omitempty"`
}

// ApplicationStatus defines the observed state of Application.
type ApplicationStatus struct {
	Ready             bool                                         `json:"ready"`
	CronJobStatuses   map[string]batchv1.CronJobStatus             `json:"cronJobStatuses,omitempty"`
	DeploymentStatus  *v1.DeploymentStatus                         `json:"deploymentStatus,omitempty"`
	IngressStatus     *networkingv1.IngressStatus                  `json:"ingressStatus,omitempty"`
	JobStatuses       map[string]batchv1.JobStatus                 `json:"jobStatuses,omitempty"`
	RolloutStatus     *v1alpha1.RolloutStatus                      `json:"rolloutStatus,omitempty"`
	StatefulSetStatus *v1.StatefulSetStatus                        `json:"statefulSetStatus,omitempty"`
	ServiceStatus     *corev1.ServiceStatus                        `json:"serviceStatus,omitempty"`
	HPAStatus         *autoscalingv1.HorizontalPodAutoscalerStatus `json:"hpaStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Application is the Schema for the applications API.
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList contains a list of Application.
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
