package model

import (
	"context"
	"errors"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

/////////////////////////////////////////////////
// Default values

const (
	// Percona XtraDB Cluster images
	DevPXCImage   = "perconalab/percona-xtradb-cluster-operator:main-pxc8.0"
	SmallPXCImage = "percona/percona-xtradb-cluster:8.0"

	// Component images
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

func (i *InfraConfigBuilder) GetMySQLConfig() (MySQLConfig, error) {
	var details MySQLConfig

	if i.mergedMySQL != nil {
		details.Enabled = i.mergedMySQL.Enabled
		details.Namespace = i.mergedMySQL.Namespace
		details.StorageSize = i.mergedMySQL.StorageSize

		if i.mergedMySQL.Config != nil {
			details.Resources.Requests = i.mergedMySQL.Config.Resources.Requests
			details.Resources.Limits = i.mergedMySQL.Config.Resources.Limits
		}

		// Get replica count and MySQL-specific config based on size
		var err error
		if details.Replicas, err = GetMySQLReplicaCountForSize(i.size); err != nil {
			return details, err
		}

		mysqlSizeConfig, err := GetMySQLConfigForSize(i.size)
		if err != nil {
			return details, err
		}

		details.PXCImage = mysqlSizeConfig.PXCImage
		details.ProxySQLEnabled = mysqlSizeConfig.ProxySQLEnabled
		details.ProxySQLReplicas = mysqlSizeConfig.ProxySQLReplicas
		details.ProxySQLImage = mysqlSizeConfig.ProxySQLImage
		details.TLSEnabled = mysqlSizeConfig.TLSEnabled
		details.LogCollectorEnabled = mysqlSizeConfig.LogCollectorEnabled
		details.LogCollectorImage = mysqlSizeConfig.LogCollectorImage
		details.AllowUnsafePXCSize = mysqlSizeConfig.AllowUnsafePXCSize
		details.AllowUnsafeProxySize = mysqlSizeConfig.AllowUnsafeProxySize
	}
	return details, nil
}

func (i *InfraConfigBuilder) AddMySQLSpec(actual *apiv2.WBMySQLSpec, size apiv2.WBSize) *InfraConfigBuilder {
	i.size = size
	var err error
	var defaultSpec, merged apiv2.WBMySQLSpec
	if defaultSpec, err = translatorv2.BuildMySQLDefaults(size, i.ownerNamespace); err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	if merged, err = translatorv2.BuildMySQLSpec(*actual, defaultSpec); err != nil {
		i.errors = append(i.errors, err)
		return i
	} else {
		i.mergedMySQL = &merged
	}
	return i
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
