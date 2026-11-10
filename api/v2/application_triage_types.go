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

import corev1 "k8s.io/api/core/v1"

// ApplicationTriageSpec contains the shared diagnostic runner and the actions
// exposed by an Application. An ActionRun selects one action by name. The
// controller exposes its type and name through WANDB_ACTION_TYPE and
// WANDB_ACTION_NAME.
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

	// Args replaces the selected container's arguments when non-empty. The
	// selected action's Args are appended when starting the diagnostic runner.
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
	Actions []ApplicationTriageActionSpec `json:"actions"`
}

// ApplicationTriageActionSpec describes one action exposed by the shared
// triage runner. Execution identity and resource settings remain on the parent
// ApplicationTriageSpec so every action uses the same bounded runtime.
type ApplicationTriageActionSpec struct {
	// Name is the stable identifier selected by ActionRun and passed to the
	// shared runner through WANDB_ACTION_NAME.
	Name ActionName `json:"name"`

	// Description is human-readable help shown by clients such as Watchtower.
	// +kubebuilder:validation:MaxLength=512
	// +optional
	Description string `json:"description,omitempty"`

	// Args are appended when starting the diagnostic runner. They are suitable
	// for action-specific flags, not executable paths.
	// +kubebuilder:validation:MaxItems=32
	// +optional
	Args []string `json:"args,omitempty"`
}
