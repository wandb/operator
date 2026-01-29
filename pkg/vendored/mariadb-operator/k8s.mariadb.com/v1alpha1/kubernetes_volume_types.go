package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#emptydirvolumesource-v1-core.
type EmptyDirVolumeSource struct {
	// +optional
	Medium corev1.StorageMedium `json:"medium,omitempty"`
	// +optional
	SizeLimit *resource.Quantity `json:"sizeLimit,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#nfsvolumesource-v1-core.
type NFSVolumeSource struct {
	Server string `json:"server"`
	Path   string `json:"path"`
	// +optional
	ReadOnly bool `json:"readOnly,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#csivolumesource-v1-core.
type CSIVolumeSource struct {
	Driver string `json:"driver"`
	// +optional
	ReadOnly *bool `json:"readOnly,omitempty"`
	// +optional
	FSType *string `json:"fsType,omitempty"`
	// +optional
	VolumeAttributes map[string]string `json:"volumeAttributes,omitempty"`
	// +optional
	NodePublishSecretRef *LocalObjectReference `json:"nodePublishSecretRef,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#hostpathvolumesource-v1-core
type HostPathVolumeSource struct {
	Path string `json:"path"`
	// +optional
	Type *string `json:"type,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#persistentvolumeclaimvolumesource-v1-core.
type PersistentVolumeClaimVolumeSource struct {
	ClaimName string `json:"claimName"`
	// +optional
	ReadOnly bool `json:"readOnly,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#secretvolumesource-v1-core.
type SecretVolumeSource struct {
	// +optional
	SecretName string `json:"secretName,omitempty"`
	// +optional
	DefaultMode *int32 `json:"defaultMode,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#configmapvolumesource-v1-core.
type ConfigMapVolumeSource struct {
	LocalObjectReference `json:",inline"`
	// +optional
	DefaultMode *int32 `json:"defaultMode,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#volume-v1-core.
type StorageVolumeSource struct {
	// +optional
	EmptyDir *EmptyDirVolumeSource `json:"emptyDir,omitempty"`
	// +optional
	NFS *NFSVolumeSource `json:"nfs,omitempty"`
	// +optional
	CSI *CSIVolumeSource `json:"csi,omitempty"`
	// +optional
	HostPath *HostPathVolumeSource `json:"hostPath,omitempty"`
	// +optional
	PersistentVolumeClaim *PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#volume-v1-core.
type VolumeSource struct {
	StorageVolumeSource `json:",inline"`
	// +optional
	Secret *SecretVolumeSource `json:"secret,omitempty"`
	// +optional
	ConfigMap *ConfigMapVolumeSource `json:"configMap,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#volume-v1-core.
type Volume struct {
	Name         string `json:"name"`
	VolumeSource `json:",inline"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#volumemount-v1-core.
type VolumeMount struct {
	// This must match the Name of a Volume.
	Name string `json:"name"`
	// +optional
	ReadOnly  bool   `json:"readOnly,omitempty"`
	MountPath string `json:"mountPath"`
	// +optional
	SubPath string `json:"subPath,omitempty"`
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#persistentvolumeclaimspec-v1-core.
type PersistentVolumeClaimSpec struct {
	// +optional
	// +listType=atomic
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// +optional
	Resources corev1.VolumeResourceRequirements `json:"resources,omitempty"`
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}
