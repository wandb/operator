package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/wandb/operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

/////////////////////////////////////////////////
// Minio Constants

const (
	MinioImage           = "quay.io/minio/minio:latest"
	DevVolumesPerServer  = int32(1)
	ProdVolumesPerServer = int32(4)
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

/////////////////////////////////////////////////
// Minio Error

type MinioErrorCode string

const (
	MinioErrFailedToGetConfigCode  MinioErrorCode = "FailedToGetConfig"
	MinioErrFailedToInitializeCode MinioErrorCode = "FailedToInitialize"
	MinioErrFailedToCreateCode     MinioErrorCode = "FailedToCreate"
	MinioErrFailedToUpdateCode     MinioErrorCode = "FailedToUpdate"
	MinioErrFailedToDeleteCode     MinioErrorCode = "FailedToDelete"
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

type MinioStatus struct {
	Ready      bool
	Connection MinioConnection
	Details    []MinioStatusDetail
	Errors     []MinioInfraError
}

type MinioConnection struct {
	Host      string
	Port      string
	AccessKey string
}

type MinioInfraCode string

const (
	MinioCreatedCode    MinioInfraCode = "MinioCreated"
	MinioUpdatedCode    MinioInfraCode = "MinioUpdated"
	MinioDeletedCode    MinioInfraCode = "MinioDeleted"
	MinioConnectionCode MinioInfraCode = "MinioConnection"
)

func NewMinioStatusDetail(code MinioInfraCode, message string) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: Minio,
		code:      string(code),
		message:   message,
	}
}

type MinioStatusDetail struct {
	InfraStatusDetail
}

func (m MinioStatusDetail) ToMinioConnDetail() (MinioConnDetail, bool) {
	if MinioInfraCode(m.Code()) != MinioConnectionCode {
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

func NewMinioConnDetail(connInfo MinioConnInfo) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: Minio,
		code:      string(MinioConnectionCode),
		message:   "Minio connection info",
		hidden:    connInfo,
	}
}

func ExtractMinioStatus(ctx context.Context, r *Results) MinioStatus {
	var ok bool
	var connDetail MinioConnDetail
	var result = MinioStatus{
		Errors: r.getMinioErrors(),
	}

	for _, detail := range r.getMinioStatusDetails() {
		if connDetail, ok = detail.ToMinioConnDetail(); ok {
			result.Connection.Host = connDetail.connInfo.Host
			result.Connection.Port = connDetail.connInfo.Port
			result.Connection.AccessKey = connDetail.connInfo.AccessKey
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

func (i InfraStatusDetail) ToMinioStatusDetail() (MinioStatusDetail, bool) {
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
	return utils.FilterMapFunc(r.StatusList, func(s InfraStatusDetail) (MinioStatusDetail, bool) { return s.ToMinioStatusDetail() })
}
