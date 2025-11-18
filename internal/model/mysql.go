package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/wandb/operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
)

/////////////////////////////////////////////////
// MySQL Default Values

const (
	DevMySQLStorageSize   = "1Gi"
	SmallMySQLStorageSize = "10Gi"

	SmallMySQLCpuRequest    = "500m"
	SmallMySQLCpuLimit      = "1000m"
	SmallMySQLMemoryRequest = "1Gi"
	SmallMySQLMemoryLimit   = "2Gi"

	DevPXCImage   = "perconalab/percona-xtradb-cluster-operator:main-pxc8.0"
	SmallPXCImage = "percona/percona-xtradb-cluster:8.0"

	ProxySQLImage     = "percona/proxysql2:2.7.3"
	LogCollectorImage = "perconalab/percona-xtradb-cluster-operator:main-logcollector"
	CRVersion         = "1.18.0"
)

/////////////////////////////////////////////////
// MySQL Config

type MySQLConfig struct {
	Enabled   bool
	Namespace string

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

func (m MySQLConfig) IsHighAvailability() bool {
	return m.Replicas > 1
}

type MySQLSizeConfig struct {
	PXCImage             string
	ProxySQLEnabled      bool
	ProxySQLReplicas     int32
	ProxySQLImage        string
	TLSEnabled           bool
	LogCollectorEnabled  bool
	LogCollectorImage    string
	AllowUnsafePXCSize   bool
	AllowUnsafeProxySize bool
}

func BuildMySQLDefaults(size Size, ownerNamespace string) (MySQLConfig, error) {
	var err error
	var storageSize string
	config := MySQLConfig{
		Enabled:   true,
		Namespace: ownerNamespace,
	}

	switch size {
	case SizeDev:
		storageSize = DevMySQLStorageSize
		config.StorageSize = storageSize
		config.Replicas = 1
		config.PXCImage = DevPXCImage
		config.ProxySQLEnabled = false
		config.ProxySQLReplicas = 0
		config.ProxySQLImage = ""
		config.TLSEnabled = false
		config.LogCollectorEnabled = true
		config.LogCollectorImage = LogCollectorImage
		config.AllowUnsafePXCSize = true
		config.AllowUnsafeProxySize = true
	case SizeSmall:
		storageSize = SmallMySQLStorageSize
		config.StorageSize = storageSize
		config.Replicas = 3
		config.PXCImage = SmallPXCImage
		config.ProxySQLEnabled = true
		config.ProxySQLReplicas = 3
		config.ProxySQLImage = ProxySQLImage
		config.TLSEnabled = true
		config.LogCollectorEnabled = false
		config.LogCollectorImage = ""
		config.AllowUnsafePXCSize = false
		config.AllowUnsafeProxySize = false

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallMySQLCpuRequest); err != nil {
			return config, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallMySQLCpuLimit); err != nil {
			return config, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallMySQLMemoryRequest); err != nil {
			return config, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallMySQLMemoryLimit); err != nil {
			return config, err
		}

		config.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuRequest,
				corev1.ResourceMemory: memoryRequest,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memoryLimit,
			},
		}
	default:
		return config, fmt.Errorf("unsupported size for MySQL: %s (only 'dev' and 'small' are supported)", size)
	}

	return config, nil
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

func NewMySQLError(code MySQLErrorCode, reason string) InfraError {
	return InfraError{
		infraName: MySQL,
		code:      string(code),
		reason:    reason,
	}
}

type MySQLInfraError struct {
	InfraError
}

func ToMySQLInfraError(err error) (MySQLInfraError, bool) {
	var infraErr InfraError
	ok := errors.As(err, &infraErr)
	if !ok {
		return MySQLInfraError{}, false
	}
	if infraErr.infraName != MySQL {
		return MySQLInfraError{}, false
	}
	return MySQLInfraError{infraErr}, true
}

func (r *Results) getMySQLErrors() []MySQLInfraError {
	return utils.FilterMapFunc(r.ErrorList, func(err error) (MySQLInfraError, bool) { return ToMySQLInfraError(err) })
}

/////////////////////////////////////////////////
// MySQL Status

type MySQLStatus struct {
	Ready      bool
	Connection MySQLConnection
	Details    []MySQLStatusDetail
	Errors     []MySQLInfraError
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

func NewMySQLStatusDetail(code MySQLInfraCode, message string) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: MySQL,
		code:      string(code),
		message:   message,
	}
}

type MySQLStatusDetail struct {
	InfraStatusDetail
}

func (m MySQLStatusDetail) ToMySQLConnDetail() (MySQLConnDetail, bool) {
	if MySQLInfraCode(m.Code()) != MySQLConnectionCode {
		return MySQLConnDetail{}, false
	}
	result := MySQLConnDetail{}
	result.hidden = m.hidden
	result.infraName = m.infraName
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

type MySQLConnDetail struct {
	MySQLStatusDetail
	connInfo MySQLConnInfo
}

func NewMySQLConnDetail(connInfo MySQLConnInfo) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: MySQL,
		code:      string(MySQLConnectionCode),
		message:   "MySQL connection info",
		hidden:    connInfo,
	}
}

func ExtractMySQLStatus(ctx context.Context, r *Results) MySQLStatus {
	var ok bool
	var connDetail MySQLConnDetail
	var result = MySQLStatus{
		Errors: r.getMySQLErrors(),
	}

	for _, detail := range r.getMySQLStatusDetails() {
		if connDetail, ok = detail.ToMySQLConnDetail(); ok {
			result.Connection.Host = connDetail.connInfo.Host
			result.Connection.Port = connDetail.connInfo.Port
			result.Connection.User = connDetail.connInfo.User
			continue
		}

		result.Details = append(result.Details, detail)
	}

	if len(result.Errors) > 0 {
		result.Ready = false
	} else {
		result.Ready = result.Connection.Host != ""
	}

	return result
}

func (i InfraStatusDetail) ToMySQLStatusDetail() (MySQLStatusDetail, bool) {
	result := MySQLStatusDetail{}
	if i.infraName != MySQL {
		return result, false
	}
	result.infraName = i.infraName
	result.code = i.code
	result.message = i.message
	result.hidden = i.hidden
	return result, true
}

func (r *Results) getMySQLStatusDetails() []MySQLStatusDetail {
	return utils.FilterMapFunc(r.StatusList, func(s InfraStatusDetail) (MySQLStatusDetail, bool) { return s.ToMySQLStatusDetail() })
}
