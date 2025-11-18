package model

import (
	"context"
	"errors"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
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

func GetMySQLReplicaCountForSize(size apiv2.WBSize) (int32, error) {
	switch size {
	case apiv2.WBSizeDev:
		return 1, nil
	case apiv2.WBSizeSmall:
		return 3, nil
	default:
		return 0, fmt.Errorf("unsupported size for MySQL: %s (only 'dev' and 'small' are supported)", size)
	}
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

func GetMySQLConfigForSize(size apiv2.WBSize) (MySQLSizeConfig, error) {
	switch size {
	case apiv2.WBSizeDev:
		return MySQLSizeConfig{
			PXCImage:             DevPXCImage,
			ProxySQLEnabled:      false,
			ProxySQLReplicas:     0,
			ProxySQLImage:        "",
			TLSEnabled:           false,
			LogCollectorEnabled:  true,
			LogCollectorImage:    LogCollectorImage,
			AllowUnsafePXCSize:   true,
			AllowUnsafeProxySize: true,
		}, nil
	case apiv2.WBSizeSmall:
		return MySQLSizeConfig{
			PXCImage:             SmallPXCImage,
			ProxySQLEnabled:      true,
			ProxySQLReplicas:     3,
			ProxySQLImage:        ProxySQLImage,
			TLSEnabled:           true,
			LogCollectorEnabled:  false,
			LogCollectorImage:    "",
			AllowUnsafePXCSize:   false,
			AllowUnsafeProxySize: false,
		}, nil
	default:
		return MySQLSizeConfig{}, fmt.Errorf("unsupported size for MySQL: %s (only 'dev' and 'small' are supported)", size)
	}
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
	MySQLErrFailedToGetConfig  MySQLErrorCode = "FailedToGetConfig"
	MySQLErrFailedToInitialize MySQLErrorCode = "FailedToInitialize"
	MySQLErrFailedToCreate     MySQLErrorCode = "FailedToCreate"
	MySQLErrFailedToUpdate     MySQLErrorCode = "FailedToUpdate"
	MySQLErrFailedToDelete     MySQLErrorCode = "FailedToDelete"
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

func (m MySQLInfraError) mysqlCode() MySQLErrorCode {
	return MySQLErrorCode(m.code)
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

type MySQLInfraCode string

const (
	MySQLCreated    MySQLInfraCode = "MySQLCreated"
	MySQLUpdated    MySQLInfraCode = "MySQLUpdated"
	MySQLDeleted    MySQLInfraCode = "MySQLDeleted"
	MySQLConnection MySQLInfraCode = "MySQLConnection"
)

func NewMySQLStatus(code MySQLInfraCode, message string) InfraStatus {
	return InfraStatus{
		infraName: MySQL,
		code:      string(code),
		message:   message,
	}
}

type MySQLStatusDetail struct {
	InfraStatus
}

func (m MySQLStatusDetail) mysqlCode() MySQLInfraCode {
	return MySQLInfraCode(m.code)
}

func (m MySQLStatusDetail) ToMySQLConnDetail() (MySQLConnDetail, bool) {
	if m.mysqlCode() != MySQLConnection {
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

func NewMySQLConnDetail(connInfo MySQLConnInfo) InfraStatus {
	return InfraStatus{
		infraName: MySQL,
		code:      string(MySQLConnection),
		message:   "MySQL connection info",
		hidden:    connInfo,
	}
}

func (r *Results) ExtractMySQLStatus(ctx context.Context) apiv2.WBMySQLStatus {
	log := ctrl.LoggerFrom(ctx)

	var ok bool
	var connDetail MySQLConnDetail
	var errors = r.getMySQLErrors()
	var statuses = r.getMySQLStatusDetails()
	var wbStatus = apiv2.WBMySQLStatus{}

	for _, err := range errors {
		wbStatus.Details = append(wbStatus.Details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.code,
			Message: err.reason,
		})
	}

	for _, status := range statuses {
		if connDetail, ok = status.ToMySQLConnDetail(); ok {
			wbStatus.Connection.MySQLHost = connDetail.connInfo.Host
			wbStatus.Connection.MySQLPort = connDetail.connInfo.Port
			wbStatus.Connection.MySQLUser = connDetail.connInfo.User
			continue
		}

		wbStatus.Details = append(wbStatus.Details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateReady,
			Code:    status.code,
			Message: status.message,
		})
	}

	if len(errors) > 0 {
		wbStatus.State = apiv2.WBStateError
	} else {
		wbStatus.State = apiv2.WBStateReady
	}

	if len(errors) > 0 {
		log.Error(
			fmt.Errorf("MySQL has %d errors", len(errors)),
			"MySQL is in error state",
			"errors", errors,
		)
	}

	return wbStatus
}

func (i InfraStatus) ToMySQLStatusDetail() (MySQLStatusDetail, bool) {
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
	return utils.FilterMapFunc(r.StatusList, func(s InfraStatus) (MySQLStatusDetail, bool) { return s.ToMySQLStatusDetail() })
}
