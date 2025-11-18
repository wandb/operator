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
// ClickHouse Default Values

const (
	DevClickHouseStorageSize   = "10Gi"
	SmallClickHouseStorageSize = "10Gi"

	SmallClickHouseCpuRequest    = "500m"
	SmallClickHouseCpuLimit      = "1000m"
	SmallClickHouseMemoryRequest = "1Gi"
	SmallClickHouseMemoryLimit   = "2Gi"

	ClickHouseVersion = "23.8"
)

/////////////////////////////////////////////////
// ClickHouse Config

type ClickHouseConfig struct {
	Enabled   bool
	Namespace string

	// Storage and resources
	StorageSize string
	Replicas    int32
	Version     string
	Resources   corev1.ResourceRequirements
}

func (c ClickHouseConfig) IsHighAvailability() bool {
	return c.Replicas > 1
}

func (i *InfraConfigBuilder) GetClickHouseConfig() (ClickHouseConfig, error) {
	var details ClickHouseConfig

	if i.mergedClickHouse != nil {
		details.Enabled = i.mergedClickHouse.Enabled
		details.Namespace = i.mergedClickHouse.Namespace
		details.StorageSize = i.mergedClickHouse.StorageSize
		details.Replicas = i.mergedClickHouse.Replicas
		details.Version = i.mergedClickHouse.Version

		if i.mergedClickHouse.Config != nil {
			details.Resources.Requests = i.mergedClickHouse.Config.Resources.Requests
			details.Resources.Limits = i.mergedClickHouse.Config.Resources.Limits
		}
	}
	return details, nil
}

func BuildClickHouseDefaults(size Size, ownerNamespace string) (ClickHouseConfig, error) {
	var err error
	var storageSize string
	config := ClickHouseConfig{
		Enabled:   true,
		Namespace: ownerNamespace,
		Version:   ClickHouseVersion,
	}

	switch size {
	case SizeDev:
		storageSize = DevClickHouseStorageSize
		config.StorageSize = storageSize
		config.Replicas = 1
	case SizeSmall:
		storageSize = SmallClickHouseStorageSize
		config.StorageSize = storageSize
		config.Replicas = 3

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallClickHouseCpuRequest); err != nil {
			return config, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallClickHouseCpuLimit); err != nil {
			return config, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallClickHouseMemoryRequest); err != nil {
			return config, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallClickHouseMemoryLimit); err != nil {
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
		return config, fmt.Errorf("unsupported size for ClickHouse: %s (only 'dev' and 'small' are supported)", size)
	}

	return config, nil
}

/////////////////////////////////////////////////
// ClickHouse Error

type ClickHouseErrorCode string

const (
	ClickHouseErrFailedToGetConfig  ClickHouseErrorCode = "FailedToGetConfig"
	ClickHouseErrFailedToInitialize ClickHouseErrorCode = "FailedToInitialize"
	ClickHouseErrFailedToCreate     ClickHouseErrorCode = "FailedToCreate"
	ClickHouseErrFailedToUpdate     ClickHouseErrorCode = "FailedToUpdate"
	ClickHouseErrFailedToDelete     ClickHouseErrorCode = "FailedToDelete"
)

func NewClickHouseError(code ClickHouseErrorCode, reason string) InfraError {
	return InfraError{
		infraName: Clickhouse,
		code:      string(code),
		reason:    reason,
	}
}

type ClickHouseInfraError struct {
	InfraError
}

func (c ClickHouseInfraError) clickhouseCode() ClickHouseErrorCode {
	return ClickHouseErrorCode(c.code)
}

func ToClickHouseInfraError(err error) (ClickHouseInfraError, bool) {
	var infraErr InfraError
	ok := errors.As(err, &infraErr)
	if !ok {
		return ClickHouseInfraError{}, false
	}
	if infraErr.infraName != Clickhouse {
		return ClickHouseInfraError{}, false
	}
	return ClickHouseInfraError{infraErr}, true
}

func (r *Results) getClickHouseErrors() []ClickHouseInfraError {
	return utils.FilterMapFunc(r.ErrorList, func(err error) (ClickHouseInfraError, bool) { return ToClickHouseInfraError(err) })
}

/////////////////////////////////////////////////
// ClickHouse Status

type ClickHouseInfraCode string

const (
	ClickHouseCreated    ClickHouseInfraCode = "ClickHouseCreated"
	ClickHouseUpdated    ClickHouseInfraCode = "ClickHouseUpdated"
	ClickHouseDeleted    ClickHouseInfraCode = "ClickHouseDeleted"
	ClickHouseConnection ClickHouseInfraCode = "ClickHouseConnection"
)

func NewClickHouseStatus(code ClickHouseInfraCode, message string) InfraStatus {
	return InfraStatus{
		infraName: Clickhouse,
		code:      string(code),
		message:   message,
	}
}

type ClickHouseStatusDetail struct {
	InfraStatus
}

func (c ClickHouseStatusDetail) clickhouseCode() ClickHouseInfraCode {
	return ClickHouseInfraCode(c.code)
}

func (c ClickHouseStatusDetail) ToClickHouseConnDetail() (ClickHouseConnDetail, bool) {
	if c.clickhouseCode() != ClickHouseConnection {
		return ClickHouseConnDetail{}, false
	}
	result := ClickHouseConnDetail{}
	result.hidden = c.hidden
	result.infraName = c.infraName
	result.code = c.code
	result.message = c.message

	connInfo, ok := c.hidden.(ClickHouseConnInfo)
	if !ok {
		ctrl.Log.Error(
			fmt.Errorf("ClickHouseConnection does not have connection info"),
			"this may result in incorrect or missing connection info",
		)
		return result, true
	}
	result.connInfo = connInfo
	return result, true
}

type ClickHouseConnInfo struct {
	Host string
	Port string
	User string
}

type ClickHouseConnDetail struct {
	ClickHouseStatusDetail
	connInfo ClickHouseConnInfo
}

func NewClickHouseConnDetail(connInfo ClickHouseConnInfo) InfraStatus {
	return InfraStatus{
		infraName: Clickhouse,
		code:      string(ClickHouseConnection),
		message:   "ClickHouse connection info",
		hidden:    connInfo,
	}
}

func (r *Results) ExtractClickHouseStatus(ctx context.Context) apiv2.WBClickHouseStatus {
	log := ctrl.LoggerFrom(ctx)

	var ok bool
	var connDetail ClickHouseConnDetail
	var errors = r.getClickHouseErrors()
	var statuses = r.getClickHouseStatusDetails()
	var wbStatus = apiv2.WBClickHouseStatus{}

	for _, err := range errors {
		wbStatus.Details = append(wbStatus.Details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.code,
			Message: err.reason,
		})
	}

	for _, status := range statuses {
		if connDetail, ok = status.ToClickHouseConnDetail(); ok {
			wbStatus.Connection.ClickHouseHost = connDetail.connInfo.Host
			wbStatus.Connection.ClickHousePort = connDetail.connInfo.Port
			wbStatus.Connection.ClickHouseUser = connDetail.connInfo.User
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
			fmt.Errorf("ClickHouse has %d errors", len(errors)),
			"ClickHouse is in error state",
			"errors", errors,
		)
	}

	return wbStatus
}

func (i InfraStatus) ToClickHouseStatusDetail() (ClickHouseStatusDetail, bool) {
	result := ClickHouseStatusDetail{}
	if i.infraName != Clickhouse {
		return result, false
	}
	result.infraName = i.infraName
	result.code = i.code
	result.message = i.message
	result.hidden = i.hidden
	return result, true
}

func (r *Results) getClickHouseStatusDetails() []ClickHouseStatusDetail {
	return utils.FilterMapFunc(r.StatusList, func(s InfraStatus) (ClickHouseStatusDetail, bool) { return s.ToClickHouseStatusDetail() })
}
