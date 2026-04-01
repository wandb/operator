package translator

import (
	corev1 "k8s.io/api/core/v1"
)

const MinioModuleName = "minio"

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
// Minio Connection

type MinioConnection struct {
	Endpoint  corev1.SecretKeySelector
	AccessKey corev1.SecretKeySelector
	SecretKey corev1.SecretKeySelector
	Bucket    corev1.SecretKeySelector
	Region    corev1.SecretKeySelector
	URL       corev1.SecretKeySelector
}

/////////////////////////////////////////////////
// Minio Status

type MinioStatus struct {
	InfraStatus
	Connection MinioConnection
}
