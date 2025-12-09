package translator

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/////////////////////////////////////////////////
// Minio Constants

const (
	MinioImage           = "quay.io/minio/minio:latest"
	DevVolumesPerServer  = int32(1)
	ProdVolumesPerServer = int32(4)
)

/////////////////////////////////////////////////
// Minio Config

type MinioConfig struct {
	Enabled   bool
	Namespace string
	Name      string

	// Custom Config
	RootUser            string
	MinioBrowserSetting string

	// Storage and resources
	StorageSize      string
	Servers          int32
	VolumesPerServer int32
	Resources        corev1.ResourceRequirements

	// Minio specific
	Image string
}

/////////////////////////////////////////////////
// Minio Status

// MinioStatus is a representation of Status that must support round-trip translation
// between any version of WBMinioStatus and itself
type MinioStatus struct {
	Ready      bool
	State      string
	Conditions []metav1.Condition
	Connection InfraConnection
}
