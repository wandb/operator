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
// Minio Default Values

const (
	DevMinioStorageSize   = "10Gi"
	SmallMinioStorageSize = "10Gi"

	SmallMinioCpuRequest    = "500m"
	SmallMinioCpuLimit      = "1000m"
	SmallMinioMemoryRequest = "1Gi"
	SmallMinioMemoryLimit   = "2Gi"

	MinioImage = "quay.io/minio/minio:latest"
)

/////////////////////////////////////////////////
// Minio Config

type MinioConfig struct {
	Enabled   bool
	Namespace string

	// Storage and resources
	StorageSize      string
	Servers          int32
	VolumesPerServer int32
	Resources        corev1.ResourceRequirements

	// Minio specific
	Image string
}

func (m MinioConfig) IsHighAvailability() bool {
	return m.Servers > 1
}

func (i *InfraConfigBuilder) GetMinioConfig() (MinioConfig, error) {
	var details MinioConfig

	if i.mergedMinio != nil {
		details.Enabled = i.mergedMinio.Enabled
		details.Namespace = i.mergedMinio.Namespace
		details.StorageSize = i.mergedMinio.StorageSize

		if i.mergedMinio.Config != nil {
			details.Resources.Requests = i.mergedMinio.Config.Resources.Requests
			details.Resources.Limits = i.mergedMinio.Config.Resources.Limits
		}

		// Get server count and volumes based on size
		var err error
		minioSizeConfig, err := GetMinioConfigForSize(i.size)
		if err != nil {
			return details, err
		}

		details.Servers = minioSizeConfig.Servers
		details.VolumesPerServer = minioSizeConfig.VolumesPerServer
		details.Image = minioSizeConfig.Image
	}
	return details, nil
}

type MinioSizeConfig struct {
	Servers          int32
	VolumesPerServer int32
	Image            string
}

func GetMinioConfigForSize(size apiv2.WBSize) (MinioSizeConfig, error) {
	switch size {
	case apiv2.WBSizeDev:
		return MinioSizeConfig{
			Servers:          1,
			VolumesPerServer: 1,
			Image:            MinioImage,
		}, nil
	case apiv2.WBSizeSmall:
		return MinioSizeConfig{
			Servers:          3,
			VolumesPerServer: 4,
			Image:            MinioImage,
		}, nil
	default:
		return MinioSizeConfig{}, fmt.Errorf("unsupported size for Minio: %s (only 'dev' and 'small' are supported)", size)
	}
}

func BuildMinioDefaults(size Size, ownerNamespace string) (MinioConfig, error) {
	var err error
	var storageSize string
	config := MinioConfig{
		Enabled:   true,
		Namespace: ownerNamespace,
		Image:     MinioImage,
	}

	switch size {
	case SizeDev:
		storageSize = DevMinioStorageSize
		config.StorageSize = storageSize
		config.Servers = 1
		config.VolumesPerServer = 1
	case SizeSmall:
		storageSize = SmallMinioStorageSize
		config.StorageSize = storageSize
		config.Servers = 3
		config.VolumesPerServer = 4

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallMinioCpuRequest); err != nil {
			return config, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallMinioCpuLimit); err != nil {
			return config, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallMinioMemoryRequest); err != nil {
			return config, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallMinioMemoryLimit); err != nil {
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
		return config, fmt.Errorf("unsupported size for Minio: %s (only 'dev' and 'small' are supported)", size)
	}

	return config, nil
}

/////////////////////////////////////////////////
// Minio Error

type MinioErrorCode string

const (
	MinioErrFailedToGetConfig  MinioErrorCode = "FailedToGetConfig"
	MinioErrFailedToInitialize MinioErrorCode = "FailedToInitialize"
	MinioErrFailedToCreate     MinioErrorCode = "FailedToCreate"
	MinioErrFailedToUpdate     MinioErrorCode = "FailedToUpdate"
	MinioErrFailedToDelete     MinioErrorCode = "FailedToDelete"
)

func NewMinioError(code MinioErrorCode, reason string) InfraError {
	return InfraError{
		infraName: Minio,
		code:      string(code),
		reason:    reason,
	}
}

type MinioInfraError struct {
	InfraError
}

func (m MinioInfraError) minioCode() MinioErrorCode {
	return MinioErrorCode(m.code)
}

func ToMinioInfraError(err error) (MinioInfraError, bool) {
	var infraErr InfraError
	ok := errors.As(err, &infraErr)
	if !ok {
		return MinioInfraError{}, false
	}
	if infraErr.infraName != Minio {
		return MinioInfraError{}, false
	}
	return MinioInfraError{infraErr}, true
}

func (r *Results) getMinioErrors() []MinioInfraError {
	return utils.FilterMapFunc(r.ErrorList, func(err error) (MinioInfraError, bool) { return ToMinioInfraError(err) })
}

/////////////////////////////////////////////////
// Minio Status

type MinioInfraCode string

const (
	MinioCreated    MinioInfraCode = "MinioCreated"
	MinioUpdated    MinioInfraCode = "MinioUpdated"
	MinioDeleted    MinioInfraCode = "MinioDeleted"
	MinioConnection MinioInfraCode = "MinioConnection"
)

func NewMinioStatus(code MinioInfraCode, message string) InfraStatus {
	return InfraStatus{
		infraName: Minio,
		code:      string(code),
		message:   message,
	}
}

type MinioStatusDetail struct {
	InfraStatus
}

func (m MinioStatusDetail) minioCode() MinioInfraCode {
	return MinioInfraCode(m.code)
}

func (m MinioStatusDetail) ToMinioConnDetail() (MinioConnDetail, bool) {
	if m.minioCode() != MinioConnection {
		return MinioConnDetail{}, false
	}
	result := MinioConnDetail{}
	result.hidden = m.hidden
	result.infraName = m.infraName
	result.code = m.code
	result.message = m.message

	connInfo, ok := m.hidden.(MinioConnInfo)
	if !ok {
		ctrl.Log.Error(
			fmt.Errorf("MinioConnection does not have connection info"),
			"this may result in incorrect or missing connection info",
		)
		return result, true
	}
	result.connInfo = connInfo
	return result, true
}

type MinioConnInfo struct {
	Host      string
	Port      string
	AccessKey string
}

type MinioConnDetail struct {
	MinioStatusDetail
	connInfo MinioConnInfo
}

func NewMinioConnDetail(connInfo MinioConnInfo) InfraStatus {
	return InfraStatus{
		infraName: Minio,
		code:      string(MinioConnection),
		message:   "Minio connection info",
		hidden:    connInfo,
	}
}

func (r *Results) ExtractMinioStatus(ctx context.Context) apiv2.WBMinioStatus {
	log := ctrl.LoggerFrom(ctx)

	var ok bool
	var connDetail MinioConnDetail
	var errors = r.getMinioErrors()
	var statuses = r.getMinioStatusDetails()
	var wbStatus = apiv2.WBMinioStatus{}

	for _, err := range errors {
		wbStatus.Details = append(wbStatus.Details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.code,
			Message: err.reason,
		})
	}

	for _, status := range statuses {
		if connDetail, ok = status.ToMinioConnDetail(); ok {
			wbStatus.Connection.MinioHost = connDetail.connInfo.Host
			wbStatus.Connection.MinioPort = connDetail.connInfo.Port
			wbStatus.Connection.MinioAccessKey = connDetail.connInfo.AccessKey
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
			fmt.Errorf("Minio has %d errors", len(errors)),
			"Minio is in error state",
			"errors", errors,
		)
	}

	return wbStatus
}

func (i InfraStatus) ToMinioStatusDetail() (MinioStatusDetail, bool) {
	result := MinioStatusDetail{}
	if i.infraName != Minio {
		return result, false
	}
	result.infraName = i.infraName
	result.code = i.code
	result.message = i.message
	result.hidden = i.hidden
	return result, true
}

func (r *Results) getMinioStatusDetails() []MinioStatusDetail {
	return utils.FilterMapFunc(r.StatusList, func(s InfraStatus) (MinioStatusDetail, bool) { return s.ToMinioStatusDetail() })
}
