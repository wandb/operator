package translator

import (
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/////////////////////////////////////////////////
// MySQL Constants

const (
	// PXC Images - using the same images as the defaults package for consistency
	DevPXCImage     = "percona/percona-xtradb-cluster:8.0"
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
// MySQL Status

// MysqlStatus is a representation of Status that must support round-trip translation
// between any version of WBMysqlStatus and itself
type MysqlStatus struct {
	Ready      bool
	State      string
	Conditions []metav1.Condition
	Connection InfraConnection
}
