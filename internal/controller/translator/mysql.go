package translator

import (
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
)

const MysqlModuleName = "mysql"

/////////////////////////////////////////////////
// MySQL Constants

const (
	// PXC Images - using the same images as the defaults package for consistency
	ProdPXCImage    = "percona/percona-xtradb-cluster:8.0"
	ProxySQLImage   = "percona/proxysql2:2.7.3"
	LogCollectorImg = "perconalab/percona-xtradb-cluster-operator:main-logcollector"
	PXCCRVersion    = "1.18.0"
)

/////////////////////////////////////////////////
// MySQL Config

type MySQLConfig struct {
	Enabled   bool
	Namespace string
	Name      string

	DeploymentType apiv2.MYSQLType

	// Storage and resources
	StorageSize string
	Replicas    int32
	Resources   corev1.ResourceRequirements

	// Percona XtraDB specific
	PXCImage            string
	ProxySQLEnabled     bool
	ProxySQLReplicas    int32
	ProxySQLImage       string
	TLSEnabled          bool
	LogCollectorEnabled bool
	LogCollectorImage   string

	// Unsafe flags (dev only)
	AllowUnsafePXCSize   bool
	AllowUnsafeProxySize bool
}

/////////////////////////////////////////////////
// MySQL Connection

type MysqlConnection struct {
	Host     corev1.SecretKeySelector
	Port     corev1.SecretKeySelector
	Database corev1.SecretKeySelector
	Username corev1.SecretKeySelector
	Password corev1.SecretKeySelector
	URL      corev1.SecretKeySelector
}

/////////////////////////////////////////////////
// MySQL Status

type MysqlStatus struct {
	InfraStatus
	Connection MysqlConnection
}
