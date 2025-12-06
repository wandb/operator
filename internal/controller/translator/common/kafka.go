package common

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

/////////////////////////////////////////////////
// Kafka Constants

const (
	KafkaVersion         = "4.1.0"
	KafkaMetadataVersion = "4.1-IV0"
)

type KafkaErrorCode string

const (
	KafkaErrFailedToGetConfigCode  KafkaErrorCode = "FailedToGetConfig"
	KafkaErrFailedToInitializeCode KafkaErrorCode = "FailedToInitialize"
	KafkaErrFailedToCreateCode     KafkaErrorCode = "FailedToCreate"
	KafkaErrFailedToUpdateCode     KafkaErrorCode = "FailedToUpdate"
	KafkaErrFailedToDeleteCode     KafkaErrorCode = "FailedToDelete"
)

/////////////////////////////////////////////////
// Kafka Status

type KafkaStatus struct {
	Ready      bool
	Connection KafkaConnection
	Conditions []KafkaCondition
}

type KafkaConnection struct {
	URL corev1.SecretKeySelector
}

type KafkaInfraCode string

const (
	KafkaCreatedCode         KafkaInfraCode = "KafkaCreated"
	KafkaUpdatedCode         KafkaInfraCode = "KafkaUpdated"
	KafkaDeletedCode         KafkaInfraCode = "KafkaDeleted"
	KafkaNodePoolCreatedCode KafkaInfraCode = "NodePoolCreated"
	KafkaNodePoolUpdatedCode KafkaInfraCode = "NodePoolUpdated"
	KafkaNodePoolDeletedCode KafkaInfraCode = "NodePoolDeleted"
	KafkaConnectionCode      KafkaInfraCode = "KafkaConnection"
)

func NewKafkaCondition(code KafkaInfraCode, message string) KafkaCondition {
	return KafkaCondition{
		code:    code,
		message: message,
	}
}

type KafkaCondition struct {
	code    KafkaInfraCode
	message string
	hidden  interface{}
}

func (k KafkaCondition) Code() string {
	return string(k.code)
}

func (k KafkaCondition) Message() string {
	return k.message
}

type KafkaConnCondition struct {
	KafkaCondition
	connInfo KafkaConnection
}

func NewKafkaConnCondition(connInfo KafkaConnection) KafkaCondition {
	return KafkaCondition{
		code:    KafkaConnectionCode,
		message: "Kafka connection info",
		hidden:  connInfo,
	}
}

func (k KafkaCondition) ToKafkaConnCondition() (KafkaConnCondition, bool) {
	if k.code != KafkaConnectionCode {
		return KafkaConnCondition{}, false
	}
	result := KafkaConnCondition{}
	result.hidden = k.hidden
	result.code = k.code
	result.message = k.message

	connInfo, ok := k.hidden.(KafkaConnection)
	if !ok {
		ctrl.Log.Error(
			fmt.Errorf("KafkaConnection does not have connection info"),
			"this may result in incorrect or missing connection info",
		)
		return result, true
	}
	result.connInfo = connInfo
	return result, true
}

func ExtractKafkaStatus(ctx context.Context, conditions []KafkaCondition) KafkaStatus {
	var ok bool
	var connCond KafkaConnCondition
	var result = KafkaStatus{}

	for _, cond := range conditions {
		if connCond, ok = cond.ToKafkaConnCondition(); ok {
			result.Connection = connCond.connInfo
			continue
		}

		result.Conditions = append(result.Conditions, cond)
	}

	result.Ready = result.Connection.URL.Name != ""

	return result
}
