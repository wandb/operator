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

// ActionRunPhase describes the execution lifecycle of an ActionRun. A
// Succeeded run means that the action command completed successfully; it does
// not mean that every result reported a passing verdict.
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
type ActionRunPhase string

const (
	ActionRunPhasePending   ActionRunPhase = "Pending"
	ActionRunPhaseRunning   ActionRunPhase = "Running"
	ActionRunPhaseSucceeded ActionRunPhase = "Succeeded"
	ActionRunPhaseFailed    ActionRunPhase = "Failed"
)

// ActionSeverity is the verdict emitted by an individual action result.
// +kubebuilder:validation:Enum=pass;warn;fail;error
type ActionSeverity string

const (
	ActionSeverityPass  ActionSeverity = "pass"
	ActionSeverityWarn  ActionSeverity = "warn"
	ActionSeverityFail  ActionSeverity = "fail"
	ActionSeverityError ActionSeverity = "error"
)

// ActionType identifies the class of action selected from an Application.
// +kubebuilder:validation:Enum=triage;maintenance
type ActionType string

const (
	ActionTypeTriage      ActionType = "triage"
	ActionTypeMaintenance ActionType = "maintenance"
)

// ApplicationReference identifies an Application in the ActionRun's namespace.
type ApplicationReference struct {
	// Name is the name of the Application that declares the selected action.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Name string `json:"name"`
}

// ActionName identifies an action declared by an Application.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=63
type ActionName string

// ActionReference selects one action declared by the referenced
// Application. Descriptive and execution metadata remain owned by the
// Application and are resolved by the controller.
type ActionReference struct {
	// Name is the stable action name exposed by the Application.
	Name ActionName `json:"name"`
}

// ActionRunSpec defines one immutable request to run one Application action.
// Creating another run requires creating another ActionRun.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable"
type ActionRunSpec struct {
	// Type selects the Application action catalog. Triage is executable in this
	// release; maintenance is reserved for its future safety contract.
	Type ActionType `json:"type"`

	// ApplicationRef identifies the Application whose action will run. Cross-namespace
	// references are intentionally unsupported.
	ApplicationRef ApplicationReference `json:"applicationRef"`

	// Action selects exactly one action declared by the referenced Application.
	Action ActionReference `json:"action"`
}

// ActionResolvedExecution records the concrete execution selected from the
// Application at reconciliation time. It is an audit snapshot, not user input.
type ActionResolvedExecution struct {
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

// ActionRunSummary contains aggregate verdict counts for a completed run.
type ActionRunSummary struct {
	Total int32 `json:"total,omitempty"`
	Pass  int32 `json:"pass,omitempty"`
	Warn  int32 `json:"warn,omitempty"`
	Fail  int32 `json:"fail,omitempty"`
	Error int32 `json:"error,omitempty"`

	// OverallSeverity is the most severe check verdict in the run.
	OverallSeverity ActionSeverity `json:"overallSeverity,omitempty"`
}

// ActionResult contains one structured record emitted by the action command.
type ActionResult struct {
	Name string `json:"name"`

	// Umbrella is an optional logical grouping for related checks.
	Umbrella string `json:"umbrella,omitempty"`

	Severity ActionSeverity `json:"severity"`
	Message  string         `json:"message,omitempty"`

	// Evidence preserves application-defined structured evidence.
	// +kubebuilder:pruning:PreserveUnknownFields
	Evidence *apiextensionsv1.JSON `json:"evidence,omitempty"`

	Remediation string `json:"remediation,omitempty"`

	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	EndedAt   *metav1.Time `json:"endedAt,omitempty"`

	DurationMilliseconds int64 `json:"durationMs,omitempty"`
}

// ActionRunStatus defines the observed execution state and structured output.
type ActionRunStatus struct {
	Phase ActionRunPhase `json:"phase,omitempty"`

	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// JobRef identifies the Kubernetes Job executing the selected action.
	JobRef *corev1.LocalObjectReference `json:"jobRef,omitempty"`

	// ResolvedExecution is the execution snapshot selected from the referenced
	// Application.
	ResolvedExecution *ActionResolvedExecution `json:"resolvedExecution,omitempty"`

	StartedAt   *metav1.Time `json:"startedAt,omitempty"`
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	Summary *ActionRunSummary `json:"summary,omitempty"`
	Results []ActionResult    `json:"results,omitempty"`

	// Conditions represent the latest available observations of the run.
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:selectablefield:JSONPath=.spec.type
// +kubebuilder:selectablefield:JSONPath=.spec.applicationRef.name
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.spec.applicationRef.name`
// +kubebuilder:printcolumn:name="Action",type=string,JSONPath=`.spec.action.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Severity",type=string,JSONPath=`.status.summary.overallSeverity`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ActionRun is one immutable request to execute an Application action.
type ActionRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActionRunSpec   `json:"spec"`
	Status ActionRunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ActionRunList contains a list of ActionRun.
type ActionRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActionRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ActionRun{}, &ActionRunList{})
}
