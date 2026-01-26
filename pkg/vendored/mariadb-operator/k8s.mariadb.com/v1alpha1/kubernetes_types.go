// nolint:lll
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#localobjectreference-v1-core.
type LocalObjectReference struct {
	// +optional
	// +default=""
	// +kubebuilder:default=""
	Name string `json:"name,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectreference-v1-core.
type ObjectReference struct {
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#secretkeyselector-v1-core.
// +structType=atomic
type SecretKeySelector struct {
	LocalObjectReference `json:",inline"`
	Key                  string `json:"key"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#configmapkeyselector-v1-core.
// +structType=atomic
type ConfigMapKeySelector struct {
	LocalObjectReference `json:",inline"`
	Key                  string `json:"key"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectfieldselector-v1-core.
// +structType=atomic
type ObjectFieldSelector struct {
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
	FieldPath  string `json:"fieldPath"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#envvarsource-v1-core.
type EnvVarSource struct {
	// +optional
	FieldRef *ObjectFieldSelector `json:"fieldRef,omitempty"`
	// +optional
	ConfigMapKeyRef *ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`
	// +optional
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#envvarsource-v1-core.
type EnvVar struct {
	// Name of the environment variable. Must be a C_IDENTIFIER.
	Name string `json:"name"`
	// +optional
	Value string `json:"value,omitempty"`
	// +optional
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#envfromsource-v1-core.
type EnvFromSource struct {
	// +optional
	Prefix string `json:"prefix,omitempty"`
	// +optional
	ConfigMapRef *LocalObjectReference `json:"configMapRef,omitempty"`
	// +optional
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#execaction-v1-core.
type ExecAction struct {
	// +optional
	// +listType=atomic
	Command []string `json:"command,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#httpgetaction-v1-core.
type HTTPGetAction struct {
	// +optional
	Path string             `json:"path,omitempty"`
	Port intstr.IntOrString `json:"port"`
	// +optional
	Host string `json:"host,omitempty"`
	// +optional
	Scheme corev1.URIScheme `json:"scheme,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#tcpsocketaction-v1-core.
type TCPSocketAction struct {
	Port intstr.IntOrString `json:"port"`
	// +optional
	Host string `json:"host,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#probe-v1-core.
type ProbeHandler struct {
	// +optional
	Exec *ExecAction `json:"exec,omitempty"`
	// +optional
	HTTPGet *HTTPGetAction `json:"httpGet,omitempty"`
	// +optional
	TCPSocket *TCPSocketAction `json:"tcpSocket,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#probe-v1-core.
type Probe struct {
	ProbeHandler `json:",inline"`
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#resourcerequirements-v1-core.
type ResourceRequirements struct {
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#securitycontext-v1-core.
type SecurityContext struct {
	// +optional
	Capabilities *corev1.Capabilities `json:"capabilities,omitempty"`
	// +optional
	Privileged *bool `json:"privileged,omitempty"`
	// +optional
	RunAsUser *int64 `json:"runAsUser,omitempty"`
	// +optional
	RunAsGroup *int64 `json:"runAsGroup,omitempty"`
	// +optional
	RunAsNonRoot *bool `json:"runAsNonRoot,omitempty"`
	// +optional
	ReadOnlyRootFilesystem *bool `json:"readOnlyRootFilesystem,omitempty"`
	// +optional
	AllowPrivilegeEscalation *bool `json:"allowPrivilegeEscalation,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#podsecuritycontext-v1-core
type PodSecurityContext struct {
	// +optional
	SELinuxOptions *corev1.SELinuxOptions `json:"seLinuxOptions,omitempty"`
	// +optional
	RunAsUser *int64 `json:"runAsUser,omitempty"`
	// +optional
	RunAsGroup *int64 `json:"runAsGroup,omitempty"`
	// +optional
	RunAsNonRoot *bool `json:"runAsNonRoot,omitempty"`
	// +optional
	// +listType=atomic
	SupplementalGroups []int64 `json:"supplementalGroups,omitempty"`
	// +optional
	FSGroup *int64 `json:"fsGroup,omitempty"`
	// +optional
	FSGroupChangePolicy *corev1.PodFSGroupChangePolicy `json:"fsGroupChangePolicy,omitempty"`
	// +optional
	SeccompProfile *corev1.SeccompProfile `json:"seccompProfile,omitempty"`
	// +optional
	AppArmorProfile *corev1.AppArmorProfile `json:"appArmorProfile,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#serviceport-v1-core
type ServicePort struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
}
