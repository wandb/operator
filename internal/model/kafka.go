package model

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
)

/////////////////////////////////////////////////
// Kafka Default Values

const (
	DevKafkaStorageSize   = "1Gi"
	SmallKafkaStorageSize = "5Gi"

	SmallKafkaCpuRequest    = "500m"
	SmallKafkaCpuLimit      = "1000m"
	SmallKafkaMemoryRequest = "1Gi"
	SmallKafkaMemoryLimit   = "2Gi"
)

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

func (k KafkaConfig) IsHighAvailability() bool {
	return k.Replicas > 1
}

func BuildKafkaDefaults(size Size, ownerNamespace string) (KafkaConfig, error) {
	var err error
	var storageSize string
	config := KafkaConfig{
		Enabled:   true,
		Namespace: ownerNamespace,
	}

	switch size {
	case SizeDev:
		storageSize = DevKafkaStorageSize
		config.StorageSize = storageSize
		config.Replicas = 1
		config.ReplicationConfig = KafkaReplicationConfig{
			DefaultReplicationFactor: 1,
			MinInSyncReplicas:        1,
			OffsetsTopicRF:           1,
			TransactionStateRF:       1,
			TransactionStateISR:      1,
		}
	case SizeSmall:
		storageSize = SmallKafkaStorageSize
		config.StorageSize = storageSize
		config.Replicas = 3
		config.ReplicationConfig = KafkaReplicationConfig{
			DefaultReplicationFactor: 3,
			MinInSyncReplicas:        2,
			OffsetsTopicRF:           3,
			TransactionStateRF:       3,
			TransactionStateISR:      2,
		}

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallKafkaCpuRequest); err != nil {
			return config, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallKafkaCpuLimit); err != nil {
			return config, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallKafkaMemoryRequest); err != nil {
			return config, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallKafkaMemoryLimit); err != nil {
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
		return config, fmt.Errorf("unsupported size for Kafka: %s (only 'dev' and 'small' are supported)", size)
	}

	return config, nil
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
