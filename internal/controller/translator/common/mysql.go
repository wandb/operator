package common

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

/////////////////////////////////////////////////
// MySQL Constants

const (
	// PXC Images - using the same images as the defaults package for consistency
	DevPXCImage     = "perconalab/percona-xtradb-cluster-operator:main-pxc8.0"
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
// MySQL Error

type MySQLErrorCode string

const (
	MySQLErrFailedToGetConfigCode  MySQLErrorCode = "FailedToGetConfig"
	MySQLErrFailedToInitializeCode MySQLErrorCode = "FailedToInitialize"
	MySQLErrFailedToCreateCode     MySQLErrorCode = "FailedToCreate"
	MySQLErrFailedToUpdateCode     MySQLErrorCode = "FailedToUpdate"
	MySQLErrFailedToDeleteCode     MySQLErrorCode = "FailedToDelete"
)

/////////////////////////////////////////////////
// MySQL Status

type MySQLStatus struct {
	Ready      bool
	Connection MySQLConnection
	Conditions []MySQLCondition
}

type MySQLConnection struct {
	Host string
	Port string
	User string
}

type MySQLInfraCode string

const (
	MySQLCreatedCode    MySQLInfraCode = "MySQLCreated"
	MySQLUpdatedCode    MySQLInfraCode = "MySQLUpdated"
	MySQLDeletedCode    MySQLInfraCode = "MySQLDeleted"
	MySQLConnectionCode MySQLInfraCode = "MySQLConnection"
)

func NewMySQLCondition(code MySQLInfraCode, message string) MySQLCondition {
	return MySQLCondition{
		code:    code,
		message: message,
	}
}

type MySQLCondition struct {
	code    MySQLInfraCode
	message string
	hidden  interface{}
}

func (m MySQLCondition) Code() string {
	return string(m.code)
}

func (m MySQLCondition) Message() string {
	return m.message
}

func (m MySQLCondition) ToMySQLConnCondition() (MySQLConnCondition, bool) {
	if m.code != MySQLConnectionCode {
		return MySQLConnCondition{}, false
	}
	result := MySQLConnCondition{}
	result.hidden = m.hidden
	result.code = m.code
	result.message = m.message

	connInfo, ok := m.hidden.(MySQLConnInfo)
	if !ok {
		ctrl.Log.Error(
			fmt.Errorf("MySQLConnection does not have connection info"),
			"this may result in incorrect or missing connection info",
		)
		return result, true
	}
	result.connInfo = connInfo
	return result, true
}

type MySQLConnInfo struct {
	Host string
	Port string
	User string
}

type MySQLConnCondition struct {
	MySQLCondition
	connInfo MySQLConnInfo
}

func NewMySQLConnCondition(connInfo MySQLConnInfo) MySQLCondition {
	return MySQLCondition{
		code:    MySQLConnectionCode,
		message: "MySQL connection info",
		hidden:  connInfo,
	}
}

func ExtractMySQLStatus(ctx context.Context, conditions []MySQLCondition) MySQLStatus {
	var ok bool
	var connCond MySQLConnCondition
	var result = MySQLStatus{}

	for _, cond := range conditions {
		if connCond, ok = cond.ToMySQLConnCondition(); ok {
			result.Connection.Host = connCond.connInfo.Host
			result.Connection.Port = connCond.connInfo.Port
			result.Connection.User = connCond.connInfo.User
			continue
		}

		result.Conditions = append(result.Conditions, cond)
	}

	result.Ready = result.Connection.Host != ""

	return result
}
