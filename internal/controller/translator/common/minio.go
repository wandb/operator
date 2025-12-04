package common

import (
	"context"
	"fmt"

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
	Name      string

	// Custom Config
	RootUser            string
	MinioBrowserSetting string

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

/////////////////////////////////////////////////
// Minio Status

type MinioStatus struct {
	Ready      bool
	Connection MinioConnection
	Conditions []MinioCondition
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

func NewMinioCondition(code MinioInfraCode, message string) MinioCondition {
	return MinioCondition{
		code:    code,
		message: message,
	}
}

type MinioCondition struct {
	code    MinioInfraCode
	message string
	hidden  interface{}
}

func (m MinioCondition) Code() string {
	return string(m.code)
}

func (m MinioCondition) Message() string {
	return m.message
}

func (m MinioCondition) ToMinioConnCondition() (MinioConnCondition, bool) {
	if m.code != MinioConnectionCode {
		return MinioConnCondition{}, false
	}
	result := MinioConnCondition{}
	result.hidden = m.hidden
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

type MinioConnCondition struct {
	MinioCondition
	connInfo MinioConnInfo
}

func NewMinioConnCondition(connInfo MinioConnInfo) MinioCondition {
	return MinioCondition{
		code:    MinioConnectionCode,
		message: "Minio connection info",
		hidden:  connInfo,
	}
}

func ExtractMinioStatus(ctx context.Context, conditions []MinioCondition) MinioStatus {
	var ok bool
	var connCond MinioConnCondition
	var result = MinioStatus{}

	for _, cond := range conditions {
		if connCond, ok = cond.ToMinioConnCondition(); ok {
			result.Connection.Host = connCond.connInfo.Host
			result.Connection.Port = connCond.connInfo.Port
			result.Connection.AccessKey = connCond.connInfo.AccessKey
			continue
		}

		result.Conditions = append(result.Conditions, cond)
	}

	result.Ready = result.Connection.Host != ""

	return result
}
