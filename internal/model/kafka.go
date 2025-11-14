package model

import (
	"context"
	"fmt"
	"slices"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/model/defaults"
	mergev2 "github.com/wandb/operator/internal/model/merge/v2"
	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

/////////////////////////////////////////////////
// Kafka Config

type KafkaConfig struct {
	Enabled           bool
	Namespace         string
	StorageSize       string
	Replicas          int32
	Resources         ResourceRequirements
	ReplicationConfig KafkaReplicationConfig
}

type KafkaReplicationConfig struct {
	DefaultReplicationFactor int32
	MinInSyncReplicas        int32
	OffsetsTopicRF           int32
	TransactionStateRF       int32
	TransactionStateISR      int32
}

type ResourceRequirements struct {
	Requests v1.ResourceList
	Limits   v1.ResourceList
}

func (k KafkaConfig) IsHighAvailability() bool {
	return k.Replicas > 1
}

func (i *InfraConfigBuilder) GetKafkaConfig() (KafkaConfig, error) {
	var details KafkaConfig

	if i.mergedKafka != nil {
		details.Enabled = i.mergedKafka.Enabled
		details.Namespace = i.mergedKafka.Namespace
		details.StorageSize = i.mergedKafka.StorageSize

		if i.mergedKafka.Config != nil {
			details.Resources.Requests = i.mergedKafka.Config.Resources.Requests
			details.Resources.Limits = i.mergedKafka.Config.Resources.Limits
		}

		// Get replica count and replication config based on size
		var err error
		if details.Replicas, err = GetReplicaCountForSize(i.size); err != nil {
			return details, err
		}
		if details.ReplicationConfig, err = GetReplicationConfigForSize(i.size); err != nil {
			return details, err
		}
	}
	return details, nil
}

func (i *InfraConfigBuilder) AddKafkaSpec(actual *apiv2.WBKafkaSpec, size apiv2.WBSize) *InfraConfigBuilder {
	i.size = size
	var err error
	var defaultSpec, merged apiv2.WBKafkaSpec
	if defaultSpec, err = defaults.Kafka(size); err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	if merged, err = mergev2.Kafka(*actual, defaultSpec); err != nil {
		i.errors = append(i.errors, err)
		return i
	} else {
		i.mergedKafka = &merged
	}
	return i
}

func GetReplicaCountForSize(size apiv2.WBSize) (int32, error) {
	switch size {
	case apiv2.WBSizeDev:
		return 1, nil
	case apiv2.WBSizeSmall:
		return 3, nil
	default:
		return 0, fmt.Errorf("unsupported size for Kafka: %s (only 'dev' and 'small' are supported)", size)
	}
}

func GetReplicationConfigForSize(size apiv2.WBSize) (KafkaReplicationConfig, error) {
	switch size {
	case apiv2.WBSizeDev:
		return KafkaReplicationConfig{
			DefaultReplicationFactor: 1,
			MinInSyncReplicas:        1,
			OffsetsTopicRF:           1,
			TransactionStateRF:       1,
			TransactionStateISR:      1,
		}, nil
	case apiv2.WBSizeSmall:
		return KafkaReplicationConfig{
			DefaultReplicationFactor: 3,
			MinInSyncReplicas:        2,
			OffsetsTopicRF:           3,
			TransactionStateRF:       3,
			TransactionStateISR:      2,
		}, nil
	default:
		return KafkaReplicationConfig{}, fmt.Errorf("unsupported size for Kafka: %s (only 'dev' and 'small' are supported)", size)
	}
}

/////////////////////////////////////////////////
// Kafka Error

type KafkaErrorCode string

const (
	KafkaErrFailedToGetConfig  KafkaErrorCode = "FailedToGetConfig"
	KafkaErrFailedToInitialize KafkaErrorCode = "FailedToInitialize"
	KafkaErrFailedToCreate     KafkaErrorCode = "FailedToCreate"
	KafkaErrFailedToUpdate     KafkaErrorCode = "FailedToUpdate"
	KafkaErrFailedToDelete     KafkaErrorCode = "FailedToDelete"
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

func (k KafkaInfraError) kafkaCode() KafkaErrorCode {
	return KafkaErrorCode(k.code)
}

/////////////////////////////////////////////////
// Kafka Status

type KafkaInfraCode string

const (
	KafkaCreated         KafkaInfraCode = "KafkaCreated"
	KafkaUpdated         KafkaInfraCode = "KafkaUpdated"
	KafkaDeleted         KafkaInfraCode = "KafkaDeleted"
	KafkaNodePoolCreated KafkaInfraCode = "NodePoolCreated"
	KafkaNodePoolUpdated KafkaInfraCode = "NodePoolUpdated"
	KafkaNodePoolDeleted KafkaInfraCode = "NodePoolDeleted"
	KafkaConnection      KafkaInfraCode = "KafkaConnection"
)

func NewKafkaStatus(code KafkaInfraCode, message string) InfraStatus {
	return InfraStatus{
		infraName: Kafka,
		code:      string(code),
		message:   message,
	}
}

type KafkaStatusDetail struct {
	InfraStatus
}

func (k KafkaStatusDetail) kafkaCode() KafkaInfraCode {
	return KafkaInfraCode(k.code)
}

func (k KafkaStatusDetail) ToKafkaConnDetail() (KafkaConnDetail, bool) {
	if k.kafkaCode() != KafkaConnection {
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

type KafkaConnInfo struct {
	Host string
	Port string
}

type KafkaConnDetail struct {
	KafkaStatusDetail
	connInfo KafkaConnInfo
}

func NewKafkaConnDetail(connInfo KafkaConnInfo) InfraStatus {
	return InfraStatus{
		infraName: Kafka,
		code:      string(KafkaConnection),
		message:   fmt.Sprintf("kafka://%s:%s", connInfo.Host, connInfo.Port),
		hidden:    connInfo,
	}
}

/////////////////////////////////////////////////
// WBKafkaStatus translation

var kafkaNotReadyStates = []apiv2.WBStateType{
	apiv2.WBStateError, apiv2.WBStateReady, apiv2.WBStateDeleting, apiv2.WBStateDegraded, apiv2.WBStateOffline,
}

func (r *Results) ExtractKafkaStatus(ctx context.Context) apiv2.WBKafkaStatus {
	log := ctrl.LoggerFrom(ctx)

	var ok bool
	var connDetail KafkaConnDetail
	var errors = r.getKafkaErrors()
	var statuses = r.getKafkaStatusDetails()
	var wbStatus = apiv2.WBKafkaStatus{}

	for _, err := range errors {
		wbStatus.Details = append(wbStatus.Details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.code,
			Message: err.reason,
		})
	}

	for _, status := range statuses {
		if connDetail, ok = status.ToKafkaConnDetail(); ok {
			wbStatus.Connection.KafkaHost = connDetail.connInfo.Host
			wbStatus.Connection.KafkaPort = connDetail.connInfo.Port
		} else {
			switch status.kafkaCode() {
			case KafkaCreated, KafkaNodePoolCreated, KafkaUpdated, KafkaNodePoolUpdated:
				wbStatus.Details = append(wbStatus.Details, apiv2.WBStatusDetail{
					State:   apiv2.WBStateUpdating,
					Code:    status.code,
					Message: status.message,
				})
			case KafkaDeleted, KafkaNodePoolDeleted:
				wbStatus.Details = append(wbStatus.Details, apiv2.WBStatusDetail{
					State:   apiv2.WBStateDeleting,
					Code:    status.code,
					Message: status.message,
				})
			default:
				log.Info("Unhandled Kafka Infra Code", "code", status.code)
			}
		}
	}

	wbStatus.Ready = true
	for _, detail := range wbStatus.Details {
		if slices.Contains(kafkaNotReadyStates, detail.State) {
			wbStatus.Ready = false
		}
		if detail.State.WorseThan(wbStatus.State) {
			wbStatus.State = detail.State
		}
	}

	return wbStatus
}
