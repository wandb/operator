package translator

import (
	corev1 "k8s.io/api/core/v1"
)

const MysqlModuleName = "mysql"

/////////////////////////////////////////////////
// MySQL Config

type MySQLConfig struct {
	Enabled   bool
	Namespace string
	Name      string

	StorageSize string
	Replicas    int32
	Resources   corev1.ResourceRequirements
}

/////////////////////////////////////////////////
// MySQL Connection

type MysqlConnection struct {
	Host     corev1.SecretKeySelector
	Port     corev1.SecretKeySelector
	Database corev1.SecretKeySelector
	Username corev1.SecretKeySelector
	Password corev1.SecretKeySelector
	Tls      corev1.SecretKeySelector
	SslCa    corev1.SecretKeySelector
	SslCert  corev1.SecretKeySelector
	SslKey   corev1.SecretKeySelector
	URL      corev1.SecretKeySelector
}

/////////////////////////////////////////////////
// MySQL Status

type MysqlStatus struct {
	InfraStatus
	Connection MysqlConnection
}
