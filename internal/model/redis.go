package model

import (
	"context"
	"errors"
	"fmt"
	"slices"

	apiv2 "github.com/wandb/operator/api/v2"
	mergev2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/internal/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
)

/////////////////////////////////////////////////
// Redis Config

type RedisConfig struct {
	Enabled     bool
	Namespace   string
	StorageSize resource.Quantity
	Requests    v1.ResourceList
	Limits      v1.ResourceList
	Sentinel    sentinelConfig
}

type sentinelConfig struct {
	Enabled         bool
	MasterGroupName string
	ReplicaCount    int
	Requests        v1.ResourceList
	Limits          v1.ResourceList
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
			details.Sentinel.ReplicaCount = mergev2.ReplicaSentinelCount
			if i.mergedRedis.Sentinel.Config != nil {
				details.Sentinel.MasterGroupName = i.mergedRedis.Sentinel.Config.MasterName
				details.Sentinel.Requests = i.mergedRedis.Sentinel.Config.Resources.Requests
				details.Sentinel.Limits = i.mergedRedis.Sentinel.Config.Resources.Limits
			}
		}
	}
	return details, nil
}

func (i *InfraConfigBuilder) AddRedisSpec(actual *apiv2.WBRedisSpec, size apiv2.WBSize) *InfraConfigBuilder {
	var err error
	var defaultSpec, merged apiv2.WBRedisSpec
	if defaultSpec, err = mergev2.BuildRedisDefaults(size, i.ownerNamespace); err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	if merged, err = mergev2.BuildRedisSpec(*actual, defaultSpec); err != nil {
		i.errors = append(i.errors, err)
		return i
	} else {
		i.mergedRedis = &merged
	}
	return i
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
