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
	ClickHouseErrFailedToGetConfigCode  ClickHouseErrorCode = "FailedToGetConfig"
	ClickHouseErrFailedToInitializeCode ClickHouseErrorCode = "FailedToInitialize"
	ClickHouseErrFailedToCreateCode     ClickHouseErrorCode = "FailedToCreate"
	ClickHouseErrFailedToUpdateCode     ClickHouseErrorCode = "FailedToUpdate"
	ClickHouseErrFailedToDeleteCode     ClickHouseErrorCode = "FailedToDelete"
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

type ClickHouseStatus struct {
	Ready      bool
	Connection ClickHouseConnection
	Details    []ClickHouseStatusDetail
	Errors     []ClickHouseInfraError
}

type ClickHouseConnection struct {
	Host string
	Port string
	User string
}

type ClickHouseInfraCode string

const (
	ClickHouseCreatedCode    ClickHouseInfraCode = "ClickHouseCreated"
	ClickHouseUpdatedCode    ClickHouseInfraCode = "ClickHouseUpdated"
	ClickHouseDeletedCode    ClickHouseInfraCode = "ClickHouseDeleted"
	ClickHouseConnectionCode ClickHouseInfraCode = "ClickHouseConnection"
)

func NewClickHouseStatusDetail(code ClickHouseInfraCode, message string) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: Clickhouse,
		code:      string(code),
		message:   message,
	}
}

type ClickHouseStatusDetail struct {
	InfraStatusDetail
}

func (c ClickHouseStatusDetail) ToClickHouseConnDetail() (ClickHouseConnDetail, bool) {
	if ClickHouseInfraCode(c.Code()) != ClickHouseConnectionCode {
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
			fmt.Errorf("ClickHouseConnectionCode does not have connection info"),
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

func NewClickHouseConnDetail(connInfo ClickHouseConnInfo) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: Clickhouse,
		code:      string(ClickHouseConnectionCode),
		message:   "ClickHouse connection info",
		hidden:    connInfo,
	}
}

func ExtractClickHouseStatus(ctx context.Context, r *Results) ClickHouseStatus {
	var ok bool
	var connDetail ClickHouseConnDetail
	var result = ClickHouseStatus{
		Errors: r.getClickHouseErrors(),
	}

	for _, detail := range r.getClickHouseStatusDetails() {
		if connDetail, ok = detail.ToClickHouseConnDetail(); ok {
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

func (i InfraStatusDetail) ToClickHouseStatusDetail() (ClickHouseStatusDetail, bool) {
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
	return utils.FilterMapFunc(r.StatusList, func(s InfraStatusDetail) (ClickHouseStatusDetail, bool) { return s.ToClickHouseStatusDetail() })
}
