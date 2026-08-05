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
	"github.com/wandb/operator/pkg/vendored/argo-rollouts/argoproj.io.rollouts/v1alpha1"
	kedav1alpha1 "github.com/wandb/operator/pkg/vendored/keda/keda.sh/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
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

	MetaTemplate metav1.ObjectMeta      `json:"metaTemplate,omitempty"`
	PodTemplate  corev1.PodTemplateSpec `json:"podTemplate,omitempty"`

	// VolumeClaimTemplates are the persistent volume claim templates used when
	// Kind is "StatefulSet". They are immutable on a live StatefulSet, so they
	// are only applied at creation time.
	// +optional
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`

	// ServiceName is the name of the (typically headless) Service that governs a
	// StatefulSet, giving each pod a stable DNS identity
	// (<pod>.<serviceName>.<namespace>.svc). Required for clustered StatefulSet
	// workloads such as an HA etcd. Ignored for non-StatefulSet kinds.
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	ServiceTemplate      *corev1.ServiceSpec                        `json:"serviceTemplate,omitempty"`
	IngressTemplate      *networkingv1.IngressSpec                  `json:"ingressTemplate,omitempty"`
	HpaTemplate          *autoscalingv2.HorizontalPodAutoscalerSpec `json:"hpaTemplate,omitempty"`
	PdbTemplate          *policyv1.PodDisruptionBudgetSpec          `json:"pdbTemplate,omitempty"`
	ScaledObjectTemplate *kedav1alpha1.ScaledObjectSpec             `json:"scaledObjectTemplate,omitempty"`
	Jobs                 []batchv1.Job                              `json:"jobs,omitempty"`
	CronJobs             []batchv1.CronJob                          `json:"cronJobs,omitempty"`

	// Triage declares the bounded diagnostic actions that may be requested for
	// this application through TriageRun resources.
	// +optional
	Triage *ApplicationTriageSpec `json:"triage,omitempty"`

	// HTTPRouteTemplate is the desired HTTPRoute spec. Nil means no HTTPRoute.
	// +optional
	HTTPRouteTemplate *HTTPRouteTemplateSpec `json:"httpRouteTemplate,omitempty"`
}

// ApplicationTriageSpec contains the shared diagnostic runner and the actions
// exposed by an Application. TriageRun selects actions by name. The default
// action runs the declared runner unchanged; named actions receive an explicit
// --action selector.
type ApplicationTriageSpec struct {
	// ContainerName selects a container from the Application pod template. It
	// may be omitted when the Application has exactly one container.
	// +optional
	ContainerName string `json:"containerName,omitempty"`

	// Command replaces the selected container's entrypoint when non-empty.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	// +optional
	Command []string `json:"command,omitempty"`

	// Args replaces the selected container's arguments when non-empty. For a
	// named action, the controller appends "--action", the selected action name,
	// and then that action's Args. The default action only appends its Args.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	// +optional
	Args []string `json:"args,omitempty"`

	// Env adds or overrides environment variables inherited from the selected
	// application container.
	// +kubebuilder:validation:MaxItems=128
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources deliberately does not inherit the parent container's resource
	// requirements. When omitted, the controller applies small bounded
	// defaults suitable for diagnostics.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// TimeoutSeconds is the Job execution deadline. Zero selects the controller
	// default.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	// +optional
	TimeoutSeconds int64 `json:"timeoutSeconds,omitempty"`

	// Actions lists the stable action names and metadata exposed to callers.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +listType=map
	// +listMapKey=name
	Actions []TriageActionSpec `json:"actions"`
}

// TriageActionSpec describes one action exposed by the shared diagnostic
// runner. Execution identity and resource settings remain on the parent
// ApplicationTriageSpec so every action uses the same bounded runtime.
type TriageActionSpec struct {
	// Name is the stable identifier selected by TriageRun. "default" invokes the
	// shared runner without an action selector; every other name is passed as
	// "--action <name>".
	Name TriageActionName `json:"name"`

	// Description is human-readable help shown by clients such as Watchtower.
	// +kubebuilder:validation:MaxLength=512
	// +optional
	Description string `json:"description,omitempty"`

	// Args are appended after the optional action selector when starting the
	// diagnostic runner. They are suitable for action-specific flags, not
	// executable paths.
	// +kubebuilder:validation:MaxItems=32
	// +optional
	Args []string `json:"args,omitempty"`
}

// HTTPRouteTemplateSpec contains the fields needed to build a Gateway API HTTPRoute.
type HTTPRouteTemplateSpec struct {
	ParentRefs []gatewayv1.ParentReference `json:"parentRefs"`
	Hostnames  []gatewayv1.Hostname        `json:"hostnames,omitempty"`

	// Paths are the URL path prefixes to match. Defaults to ["/"] if empty.
	// +optional
	Paths []string `json:"paths,omitempty"`

	// PathType controls the match type: "Exact" for exact matching, anything else for prefix.
	// +optional
	PathType string `json:"pathType,omitempty"`

	// ServicePort is the port on the backend service to route traffic to.
	// Nil means no port is specified in the backend ref.
	// +optional
	ServicePort *gatewayv1.PortNumber `json:"servicePort,omitempty"`
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
	HPAStatus         *autoscalingv2.HorizontalPodAutoscalerStatus `json:"hpaStatus,omitempty"`

	// +optional
	HTTPRouteStatus *HTTPRouteStatusSummary `json:"httpRouteStatus,omitempty"`
}

type HTTPRouteStatusSummary struct {
	Accepted bool `json:"accepted,omitempty"`
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
