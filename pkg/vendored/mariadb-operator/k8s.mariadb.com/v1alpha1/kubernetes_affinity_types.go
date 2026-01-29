// nolint:lll
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#labelselectorrequirement-v1-meta
type LabelSelectorRequirement struct {
	Key      string                       `json:"key"`
	Operator metav1.LabelSelectorOperator `json:"operator"`
	// +optional
	// +listType=atomic
	Values []string `json:"values,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#labelselector-v1-meta
type LabelSelector struct {
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	// +optional
	// +listType=atomic
	MatchExpressions []LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#podaffinityterm-v1-core.
type PodAffinityTerm struct {
	// +optional
	LabelSelector *LabelSelector `json:"labelSelector,omitempty"`
	TopologyKey   string         `json:"topologyKey"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#weightedpodaffinityterm-v1-core.
type WeightedPodAffinityTerm struct {
	Weight          int32           `json:"weight"`
	PodAffinityTerm PodAffinityTerm `json:"podAffinityTerm"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#podantiaffinity-v1-core.
type PodAntiAffinity struct {
	// +optional
	// +listType=atomic
	RequiredDuringSchedulingIgnoredDuringExecution []PodAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	// +optional
	// +listType=atomic
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#nodeselectorrequirement-v1-core
type NodeSelectorRequirement struct {
	Key      string                      `json:"key"`
	Operator corev1.NodeSelectorOperator `json:"operator"`
	// +optional
	// +listType=atomic
	Values []string `json:"values,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#nodeselectorterm-v1-core
type NodeSelectorTerm struct {
	// +optional
	// +listType=atomic
	MatchExpressions []NodeSelectorRequirement `json:"matchExpressions,omitempty"`
	// +optional
	// +listType=atomic
	MatchFields []NodeSelectorRequirement `json:"matchFields,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#nodeselector-v1-core
type NodeSelector struct {
	// +listType=atomic
	NodeSelectorTerms []NodeSelectorTerm `json:"nodeSelectorTerms"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#preferredschedulingterm-v1-core
type PreferredSchedulingTerm struct {
	Weight     int32            `json:"weight"`
	Preference NodeSelectorTerm `json:"preference"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#nodeaffinity-v1-core
type NodeAffinity struct {
	// +optional
	RequiredDuringSchedulingIgnoredDuringExecution *NodeSelector `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	// +optional
	// +listType=atomic
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#affinity-v1-core.
type Affinity struct {
	// +optional
	PodAntiAffinity *PodAntiAffinity `json:"podAntiAffinity,omitempty"`
	// +optional
	NodeAffinity *NodeAffinity `json:"nodeAffinity,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#topologyspreadconstraint-v1-core.
type TopologySpreadConstraint struct {
	MaxSkew           int32                                `json:"maxSkew"`
	TopologyKey       string                               `json:"topologyKey"`
	WhenUnsatisfiable corev1.UnsatisfiableConstraintAction `json:"whenUnsatisfiable"`
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	// +optional
	MinDomains *int32 `json:"minDomains,omitempty"`
	// +optional
	NodeAffinityPolicy *corev1.NodeInclusionPolicy `json:"nodeAffinityPolicy,omitempty"`
	// +optional
	NodeTaintsPolicy *corev1.NodeInclusionPolicy `json:"nodeTaintsPolicy,omitempty"`
	// +optional
	MatchLabelKeys []string `json:"matchLabelKeys,omitempty"`
}
