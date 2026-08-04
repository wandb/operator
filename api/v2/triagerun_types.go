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
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TriageRunPhase describes the execution lifecycle of a TriageRun. A
// Succeeded run means that the diagnostic command completed successfully; it
// does not mean that every diagnostic check passed.
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
type TriageRunPhase string

const (
	TriageRunPhasePending   TriageRunPhase = "Pending"
	TriageRunPhaseRunning   TriageRunPhase = "Running"
	TriageRunPhaseSucceeded TriageRunPhase = "Succeeded"
	TriageRunPhaseFailed    TriageRunPhase = "Failed"
)

// TriageSeverity is the verdict emitted by an individual diagnostic check.
// +kubebuilder:validation:Enum=pass;warn;fail;error
type TriageSeverity string

const (
	TriageSeverityPass  TriageSeverity = "pass"
	TriageSeverityWarn  TriageSeverity = "warn"
	TriageSeverityFail  TriageSeverity = "fail"
	TriageSeverityError TriageSeverity = "error"
)

// TriageApplicationReference identifies an Application in the TriageRun's
// namespace.
type TriageApplicationReference struct {
	// Name is the name of the Application to diagnose.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// TriageActionName identifies an action declared by an Application.
// +kubebuilder:validation:MinLength=1
type TriageActionName string

// TriageRunSpec defines one immutable request to run one or more diagnostic
// actions for an Application.
// Creating another run requires creating another TriageRun.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable"
type TriageRunSpec struct {
	// ApplicationRef identifies the Application to diagnose. Cross-namespace
	// references are intentionally unsupported.
	ApplicationRef TriageApplicationReference `json:"applicationRef"`

	// Actions selects one or more triage actions declared by the referenced
	// Application. Each action is executed independently.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:UniqueItems=true
	Actions []TriageActionName `json:"actions"`
}

// TriageResolvedExecution records the concrete execution selected from the
// Application at reconciliation time. It is an audit snapshot, not user input.
type TriageResolvedExecution struct {
	// ApplicationGeneration is the Application generation used to resolve this
	// execution.
	ApplicationGeneration int64 `json:"applicationGeneration,omitempty"`

	// ContainerName is the application container whose image and runtime
	// configuration were selected.
	ContainerName string `json:"containerName,omitempty"`

	// Image is the concrete container image used by the diagnostic Job.
	Image string `json:"image,omitempty"`

	// Command is the resolved container entrypoint.
	Command []string `json:"command,omitempty"`

	// Args are the resolved arguments passed to Command.
	Args []string `json:"args,omitempty"`

	// TimeoutSeconds is the resolved execution deadline.
	TimeoutSeconds int64 `json:"timeoutSeconds,omitempty"`
}

// TriageRunSummary contains aggregate verdict counts for a completed run.
type TriageRunSummary struct {
	Total int32 `json:"total,omitempty"`
	Pass  int32 `json:"pass,omitempty"`
	Warn  int32 `json:"warn,omitempty"`
	Fail  int32 `json:"fail,omitempty"`
	Error int32 `json:"error,omitempty"`

	// OverallSeverity is the most severe check verdict in the run.
	OverallSeverity TriageSeverity `json:"overallSeverity,omitempty"`
}

// TriageCheckResult contains one structured record emitted by the diagnostic
// command.
type TriageCheckResult struct {
	Name string `json:"name"`

	// Umbrella is an optional logical grouping for related checks.
	Umbrella string `json:"umbrella,omitempty"`

	Severity TriageSeverity `json:"severity"`
	Message  string         `json:"message,omitempty"`

	// Evidence preserves application-defined structured evidence.
	// +kubebuilder:pruning:PreserveUnknownFields
	Evidence *apiextensionsv1.JSON `json:"evidence,omitempty"`

	Remediation string `json:"remediation,omitempty"`

	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	EndedAt   *metav1.Time `json:"endedAt,omitempty"`

	DurationMilliseconds int64 `json:"durationMs,omitempty"`
}

// TriageActionStatus records the execution and structured diagnostic output
// for one selected action.
type TriageActionStatus struct {
	// Action is the selected Application action represented by this status.
	Action TriageActionName `json:"action"`

	Phase TriageRunPhase `json:"phase,omitempty"`

	// JobRef identifies the Kubernetes Job executing this action.
	JobRef *corev1.LocalObjectReference `json:"jobRef,omitempty"`

	// ResolvedExecution is the execution snapshot selected from the referenced
	// Application.
	ResolvedExecution *TriageResolvedExecution `json:"resolvedExecution,omitempty"`

	StartedAt   *metav1.Time `json:"startedAt,omitempty"`
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	Summary *TriageRunSummary   `json:"summary,omitempty"`
	Results []TriageCheckResult `json:"results,omitempty"`
}

// TriageRunStatus defines the observed execution state and diagnostic output.
type TriageRunStatus struct {
	Phase TriageRunPhase `json:"phase,omitempty"`

	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	StartedAt   *metav1.Time `json:"startedAt,omitempty"`
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	Summary *TriageRunSummary `json:"summary,omitempty"`

	// ActionStatuses contains one entry for every selected action.
	// +listType=map
	// +listMapKey=action
	ActionStatuses []TriageActionStatus `json:"actionStatuses,omitempty"`

	// Conditions represent the latest available observations of the run.
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.spec.applicationRef.name`
// +kubebuilder:printcolumn:name="Actions",type=string,JSONPath=`.spec.actions`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Severity",type=string,JSONPath=`.status.summary.overallSeverity`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TriageRun is one immutable request to diagnose an Application.
type TriageRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TriageRunSpec   `json:"spec"`
	Status TriageRunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TriageRunList contains a list of TriageRun.
type TriageRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TriageRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TriageRun{}, &TriageRunList{})
}
