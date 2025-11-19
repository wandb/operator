package common

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

/////////////////////////////////////////////////
// Kafka Constants

const (
	KafkaVersion         = "4.1.0"
	KafkaMetadataVersion = "4.1-IV0"
)

// GetKafkaReplicationConfig returns replication settings based on replica count.
// For single replica (dev mode), all factors are 1.
// For multi-replica (HA mode), uses standard HA settings.
func GetKafkaReplicationConfig(replicas int32) KafkaReplicationConfig {
	if replicas == 1 {
		return KafkaReplicationConfig{
			DefaultReplicationFactor: 1,
			MinInSyncReplicas:        1,
			OffsetsTopicRF:           1,
			TransactionStateRF:       1,
			TransactionStateISR:      1,
		}
	}
	// Multi-replica HA configuration
	minISR := int32(2)
	if replicas < 3 {
		minISR = 1
	}
	return KafkaReplicationConfig{
		DefaultReplicationFactor: min32(replicas, 3),
		MinInSyncReplicas:        minISR,
		OffsetsTopicRF:           min32(replicas, 3),
		TransactionStateRF:       min32(replicas, 3),
		TransactionStateISR:      minISR,
	}
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

/////////////////////////////////////////////////
// Kafka Config

type KafkaConfig struct {
	Enabled           bool
	Namespace         string
	StorageSize       string
	Replicas          int32
	Resources         corev1.ResourceRequirements
	ReplicationConfig KafkaReplicationConfig
}

type KafkaReplicationConfig struct {
	DefaultReplicationFactor int32
	MinInSyncReplicas        int32
	OffsetsTopicRF           int32
	TransactionStateRF       int32
	TransactionStateISR      int32
}

/////////////////////////////////////////////////
// Kafka Error

type KafkaErrorCode string

const (
	KafkaErrFailedToGetConfigCode  KafkaErrorCode = "FailedToGetConfig"
	KafkaErrFailedToInitializeCode KafkaErrorCode = "FailedToInitialize"
	KafkaErrFailedToCreateCode     KafkaErrorCode = "FailedToCreate"
	KafkaErrFailedToUpdateCode     KafkaErrorCode = "FailedToUpdate"
	KafkaErrFailedToDeleteCode     KafkaErrorCode = "FailedToDelete"
)

func NewKafkaError(code KafkaErrorCode, reason string) InfraError {
	return InfraError{
		infraName: Kafka,
		code:      string(code),
		reason:    reason,
	}
}

type KafkaInfraError struct {
	InfraError
}

func ToKafkaInfraError(err error) (KafkaInfraError, bool) {
	var infraError InfraError
	var ok bool
	infraError, ok = ToInfraError(err)
	if !ok {
		return KafkaInfraError{}, false
	}
	result := KafkaInfraError{}
	if infraError.infraName != Kafka {
		return result, false
	}
	result.infraName = infraError.infraName
	result.code = infraError.code
	result.reason = infraError.reason
	return result, true
}

func (r *Results) getKafkaErrors() []KafkaInfraError {
	return utils.FilterMapFunc(r.ErrorList, func(err error) (KafkaInfraError, bool) { return ToKafkaInfraError(err) })
}

/////////////////////////////////////////////////
// Kafka Status

type KafkaStatus struct {
	Ready      bool
	Connection KafkaConnection
	Details    []KafkaStatusDetail
	Errors     []KafkaInfraError
}

type KafkaConnection struct {
	Host string
	Port string
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

func NewKafkaStatusDetail(code KafkaInfraCode, message string) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: Kafka,
		code:      string(code),
		message:   message,
	}
}

type KafkaStatusDetail struct {
	InfraStatusDetail
}

type KafkaConnInfo struct {
	Host string
	Port string
}

type KafkaConnDetail struct {
	KafkaStatusDetail
	connInfo KafkaConnInfo
}

func NewKafkaConnDetail(connInfo KafkaConnInfo) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: Kafka,
		code:      string(KafkaConnectionCode),
		message:   fmt.Sprintf("kafka://%s:%s", connInfo.Host, connInfo.Port),
		hidden:    connInfo,
	}
}

func (k KafkaStatusDetail) ToKafkaConnDetail() (KafkaConnDetail, bool) {
	if KafkaInfraCode(k.Code()) != KafkaConnectionCode {
		return KafkaConnDetail{}, false
	}
	result := KafkaConnDetail{}
	result.hidden = k.hidden
	result.infraName = k.infraName
	result.code = k.code
	result.message = k.message

	connInfo, ok := k.hidden.(KafkaConnInfo)
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

/////////////////////////////////////////////////
// WBKafkaStatus translation

func ExtractKafkaStatus(ctx context.Context, r *Results) KafkaStatus {
	var ok bool
	var connDetail KafkaConnDetail
	var result = KafkaStatus{
		Errors: r.getKafkaErrors(),
	}

	for _, detail := range r.getKafkaStatusDetails() {
		if connDetail, ok = detail.ToKafkaConnDetail(); ok {
			result.Connection.Host = connDetail.connInfo.Host
			result.Connection.Port = connDetail.connInfo.Port
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

func (r *Results) getKafkaStatusDetails() []KafkaStatusDetail {
	return utils.FilterMapFunc(r.StatusList, func(s InfraStatusDetail) (KafkaStatusDetail, bool) { return s.ToKafkaStatusDetail() })
}
