package model

import (
	"context"
	"errors"
	"fmt"
	"slices"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
)

/////////////////////////////////////////////////
// Redis Default Values

const (
	ReplicaSentinelCount = 3

	DefaultSentinelGroup = "gorilla"

	DevStorageRequest = "100Mi"

	SmallStorageRequest        = "2Gi"
	SmallReplicaCpuRequest     = "250m"
	SmallReplicaCpuLimit       = "500m"
	SmallReplicaMemoryRequest  = "256Mi"
	SmallReplicaMemoryLimit    = "512Mi"
	SmallSentinelCpuRequest    = "125m"
	SmallSentinelCpuLimit      = "256m"
	SmallSentinelMemoryRequest = "128Mi"
	SmallSentinelMemoryLimit   = "256Mi"
)

/////////////////////////////////////////////////
// Redis Config

type RedisConfig struct {
	Enabled     bool
	Namespace   string
	StorageSize resource.Quantity
	Requests    corev1.ResourceList
	Limits      corev1.ResourceList
	Sentinel    SentinelConfig
}

type SentinelConfig struct {
	Enabled         bool
	MasterGroupName string
	ReplicaCount    int
	Requests        corev1.ResourceList
	Limits          corev1.ResourceList
}

func (r RedisConfig) IsHighAvailability() bool {
	return r.Sentinel.Enabled
}

func (i *InfraConfigBuilder) GetRedisConfig() (RedisConfig, error) {
	var details RedisConfig

	if i.mergedRedis != nil {
		details.Enabled = i.mergedRedis.Enabled
		details.Namespace = i.mergedRedis.Namespace
		details.StorageSize = resource.MustParse(i.mergedRedis.StorageSize)
		if i.mergedRedis.Config != nil {
			details.Requests = i.mergedRedis.Config.Resources.Requests
			details.Limits = i.mergedRedis.Config.Resources.Limits
		}
		if i.mergedRedis.Sentinel != nil {
			details.Sentinel.Enabled = i.mergedRedis.Sentinel.Enabled
			details.Sentinel.ReplicaCount = ReplicaSentinelCount
			if i.mergedRedis.Sentinel.Config != nil {
				details.Sentinel.MasterGroupName = i.mergedRedis.Sentinel.Config.MasterName
				details.Sentinel.Requests = i.mergedRedis.Sentinel.Config.Resources.Requests
				details.Sentinel.Limits = i.mergedRedis.Sentinel.Config.Resources.Limits
			}
		}
	}
	return details, nil
}

func BuildRedisDefaults(size Size, ownerNamespace string) (RedisConfig, error) {
	var err error
	var storageRequest, cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
	config := RedisConfig{
		Enabled:   true,
		Namespace: ownerNamespace,
		Requests:  corev1.ResourceList{},
		Limits:    corev1.ResourceList{},
	}

	switch size {
	case SizeDev:
		if storageRequest, err = resource.ParseQuantity(DevStorageRequest); err != nil {
			return config, err
		}
		config.StorageSize = storageRequest
	case SizeSmall:
		if storageRequest, err = resource.ParseQuantity(SmallStorageRequest); err != nil {
			return config, err
		}
		if cpuRequest, err = resource.ParseQuantity(SmallReplicaCpuRequest); err != nil {
			return config, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallReplicaCpuLimit); err != nil {
			return config, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallReplicaMemoryRequest); err != nil {
			return config, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallReplicaMemoryLimit); err != nil {
			return config, err
		}

		config.StorageSize = storageRequest
		config.Requests[corev1.ResourceCPU] = cpuRequest
		config.Limits[corev1.ResourceCPU] = cpuLimit
		config.Requests[corev1.ResourceMemory] = memoryRequest
		config.Limits[corev1.ResourceMemory] = memoryLimit

		var sentinelCpuRequest, sentinelCpuLimit, sentinelMemoryRequest, sentinelMemoryLimit resource.Quantity
		if sentinelCpuRequest, err = resource.ParseQuantity(SmallSentinelCpuRequest); err != nil {
			return config, err
		}
		if sentinelCpuLimit, err = resource.ParseQuantity(SmallSentinelCpuLimit); err != nil {
			return config, err
		}
		if sentinelMemoryRequest, err = resource.ParseQuantity(SmallSentinelMemoryRequest); err != nil {
			return config, err
		}
		if sentinelMemoryLimit, err = resource.ParseQuantity(SmallSentinelMemoryLimit); err != nil {
			return config, err
		}

		config.Sentinel = SentinelConfig{
			Enabled:         true,
			MasterGroupName: DefaultSentinelGroup,
			ReplicaCount:    ReplicaSentinelCount,
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    sentinelCpuRequest,
				corev1.ResourceMemory: sentinelMemoryRequest,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    sentinelCpuLimit,
				corev1.ResourceMemory: sentinelMemoryLimit,
			},
		}
	default:
		return config, fmt.Errorf("invalid profile: %v", size)
	}

	return config, nil
}

/////////////////////////////////////////////////
// Redis Error

type RedisErrorCode string

const (
	RedisDeploymentConflict RedisErrorCode = "DeploymentConflict"
)

func NewRedisError(code RedisErrorCode, reason string) InfraError {
	return InfraError{
		infraName: Redis,
		code:      string(code),
		reason:    reason,
	}
}

type RedisInfraError struct {
	InfraError
}

func (r RedisInfraError) redisCode() RedisErrorCode {
	return RedisErrorCode(r.code)
}

func ToRedisInfraError(err error) (RedisInfraError, bool) {
	var infraError InfraError
	var ok bool
	infraError, ok = ToInfraError(err)
	if !ok {
		return RedisInfraError{}, false
	}
	result := RedisInfraError{}
	if infraError.infraName != Redis {
		return result, false
	}
	result.infraName = infraError.infraName
	result.code = infraError.code
	result.reason = infraError.reason
	return result, true
}

func (r *Results) getRedisErrors() []RedisInfraError {
	return utils.FilterMapFunc(r.ErrorList, func(err error) (RedisInfraError, bool) { return ToRedisInfraError(err) })
}

/////////////////////////////////////////////////
// Redis Status

/////////////////////////////////////////////////
// Redis Status

type RedisInfraCode string

const (
	RedisSentinelCreated    RedisInfraCode = "SentinelCreated"
	RedisReplicationCreated RedisInfraCode = "ReplicationCreated"
	RedisStandaloneCreated  RedisInfraCode = "StandaloneCreated"

	RedisSentinelDeleted    RedisInfraCode = "SentinelDeleted"
	RedisReplicationDeleted RedisInfraCode = "ReplicationDeleted"
	RedisStandaloneDeleted  RedisInfraCode = "StandaloneDeleted"

	RedisStandaloneConnection RedisInfraCode = "StandaloneConnection"
	RedisSentinelConnection   RedisInfraCode = "SentinelConnection"
)

func NewRedisStatus(code RedisInfraCode, message string) InfraStatus {
	return InfraStatus{
		infraName: Redis,
		code:      string(code),
		message:   message,
	}
}

type RedisStatusDetail struct {
	InfraStatus
}

func (r RedisStatusDetail) redisCode() RedisInfraCode {
	return RedisInfraCode(r.code)
}

func (r RedisStatusDetail) ToRedisSentinelConnDetail() (RedisSentinelConnDetail, bool) {
	if r.redisCode() != RedisSentinelConnection {
		return RedisSentinelConnDetail{}, false
	}
	result := RedisSentinelConnDetail{}
	result.hidden = r.hidden
	result.infraName = r.infraName
	result.code = r.code
	result.message = r.message

	connInfo, ok := r.hidden.(RedisSentinelConnInfo)
	if !ok {
		ctrl.Log.Error(
			errors.New("RedisSentinelConnection does not have connection info"),
			"this may result in incorrect or missing connection info",
		)
		return result, true
	}
	result.connInfo = connInfo
	return result, true
}

func (r RedisStatusDetail) ToRedisStandaloneConnDetail() (RedisStandaloneConnDetail, bool) {
	if r.redisCode() != RedisStandaloneConnection {
		return RedisStandaloneConnDetail{}, false
	}
	result := RedisStandaloneConnDetail{}
	result.hidden = r.hidden
	result.infraName = r.infraName
	result.code = r.code
	result.message = r.message

	connInfo, ok := r.hidden.(RedisStandaloneConnInfo)
	if !ok {
		ctrl.Log.Error(
			errors.New("RedisStandaloneConnection does not have connection info"),
			"this may result in incorrect or missing connection info",
		)
		return result, true
	}
	result.connInfo = connInfo
	return result, true
}

type RedisSentinelConnInfo struct {
	SentinelHost string
	SentinelPort string
	MasterName   string
}

func NewRedisSentinelConnDetail(connInfo RedisSentinelConnInfo) InfraStatus {
	return InfraStatus{
		infraName: Redis,
		code:      string(RedisSentinelConnection),
		message:   fmt.Sprintf("redis://%s:%s?master=%s", connInfo.SentinelHost, connInfo.SentinelPort, connInfo.MasterName),
		hidden:    connInfo,
	}
}

type RedisSentinelConnDetail struct {
	RedisStatusDetail
	connInfo RedisSentinelConnInfo
}

type RedisStandaloneConnInfo struct {
	Host string
	Port string
}

type RedisStandaloneConnDetail struct {
	RedisStatusDetail
	connInfo RedisStandaloneConnInfo
}

func NewRedisStandaloneConnDetail(connInfo RedisStandaloneConnInfo) InfraStatus {
	return InfraStatus{
		infraName: Redis,
		code:      string(RedisStandaloneConnection),
		message:   fmt.Sprintf("redis://%s:%s", connInfo.Host, connInfo.Port),
		hidden:    connInfo,
	}
}

/////////////////////////////////////////////////
// WBRedisStatus translation

var redisNotReadyStates = []apiv2.WBStateType{
	apiv2.WBStateError, apiv2.WBStateReady, apiv2.WBStateDeleting, apiv2.WBStateDegraded, apiv2.WBStateOffline,
}

func (r *Results) ExtractRedisStatus(ctx context.Context) apiv2.WBRedisStatus {
	log := ctrl.LoggerFrom(ctx)

	var ok bool
	var sentinelConnDetail RedisSentinelConnDetail
	var standaloneConnDetail RedisStandaloneConnDetail
	var errors = r.getRedisErrors()
	var statuses = r.getRedisStatusDetails()
	var wbStatus = apiv2.WBRedisStatus{}

	for _, err := range errors {
		wbStatus.Details = append(wbStatus.Details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.code,
			Message: err.reason,
		})
	}

	for _, status := range statuses {

		if sentinelConnDetail, ok = status.ToRedisSentinelConnDetail(); ok {
			wbStatus.Connection.RedisSentinelHost = sentinelConnDetail.connInfo.SentinelHost
			wbStatus.Connection.RedisSentinelPort = sentinelConnDetail.connInfo.SentinelPort
			wbStatus.Connection.RedisMasterName = sentinelConnDetail.connInfo.MasterName
		} else if standaloneConnDetail, ok = status.ToRedisStandaloneConnDetail(); ok {
			wbStatus.Connection.RedisHost = standaloneConnDetail.connInfo.Host
			wbStatus.Connection.RedisPort = standaloneConnDetail.connInfo.Port
		} else {
			switch status.redisCode() {
			case RedisSentinelCreated, RedisReplicationCreated, RedisStandaloneCreated:
				wbStatus.Details = append(wbStatus.Details, apiv2.WBStatusDetail{
					State:   apiv2.WBStateUpdating,
					Code:    status.code,
					Message: status.message,
				})
				break
			case RedisStandaloneDeleted, RedisSentinelDeleted, RedisReplicationDeleted:
				wbStatus.Details = append(wbStatus.Details, apiv2.WBStatusDetail{
					State:   apiv2.WBStateDeleting,
					Code:    status.code,
					Message: status.message,
				})
				break
			default:
				log.Info("Unhandled Redis Infra Code", "code", status.code)
			}
		}
	}

	wbStatus.Ready = true
	for _, detail := range wbStatus.Details {
		if slices.Contains(redisNotReadyStates, detail.State) {
			wbStatus.Ready = false
		}
		if detail.State.WorseThan(wbStatus.State) {
			wbStatus.State = detail.State
		}
	}

	return wbStatus
}

func (r *Results) getRedisStatusDetails() []RedisStatusDetail {
	return utils.FilterMapFunc(r.StatusList, func(s InfraStatus) (RedisStatusDetail, bool) { return s.ToRedisStatusDetail() })
}
