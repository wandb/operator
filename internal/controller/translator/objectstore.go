package translator

import (
	corev1 "k8s.io/api/core/v1"
)

const ObjectStoreModuleName = "seaweedfs"

/////////////////////////////////////////////////
// SeaweedFS Constants

const (
	SeaweedImage = "chrislusf/seaweedfs:latest"
)

/////////////////////////////////////////////////
// ObjectStore Config

type ObjectStoreConfig struct {
	Enabled   bool
	Namespace string
	Name      string

	AccessKey string

	StorageSize string
	Replicas    int32
	Resources   corev1.ResourceRequirements
}

/////////////////////////////////////////////////
// ObjectStore Connection

type ObjectStoreConnection struct {
	Endpoint  corev1.SecretKeySelector
	Port      corev1.SecretKeySelector
	AccessKey corev1.SecretKeySelector
	SecretKey corev1.SecretKeySelector
	Bucket    corev1.SecretKeySelector
	Region    corev1.SecretKeySelector
	URL       corev1.SecretKeySelector
}

/////////////////////////////////////////////////
// ObjectStore Status

type ObjectStoreStatus struct {
	InfraStatus
	Connection ObjectStoreConnection
}
