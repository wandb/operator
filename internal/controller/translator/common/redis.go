package common

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
// Redis Constants

const (
	RedisStandaloneImage  = "quay.io/opstree/redis:v7.0.15"
	RedisReplicationImage = "quay.io/opstree/redis:v7.0.15"
	RedisSentinelImage    = "quay.io/opstree/redis-sentinel:v7.0.12"
	RedisNamePrefix       = "wandb-redis"
)

/////////////////////////////////////////////////
// Redis Config

type RedisConfig struct {
	Enabled     bool
	Namespace   string
	Name        string
	StorageSize resource.Quantity
	Requests    corev1.ResourceList
	Limits      corev1.ResourceList
	Sentinel    SentinelConfig
}

type SentinelConfig struct {
	Enabled         bool
	SentinelName    string
	ReplicationName string
	MasterGroupName string
	ReplicaCount    int
	Requests        corev1.ResourceList
	Limits          corev1.ResourceList
}

/////////////////////////////////////////////////
// Redis Error

type RedisErrorCode string

const (
	RedisDeploymentConflictCode RedisErrorCode = "DeploymentConflict"
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

type RedisStatus struct {
	Ready      bool
	Connection RedisConnection
	Details    []RedisStatusDetail
	Errors     []RedisInfraError
}

type RedisConnection struct {
	RedisHost      string
	RedisPort      string
	SentinelHost   string
	SentinelPort   string
	SentinelMaster string
}

type RedisInfraCode string

const (
	RedisSentinelCreatedCode    RedisInfraCode = "SentinelCreated"
	RedisReplicationCreatedCode RedisInfraCode = "ReplicationCreated"
	RedisStandaloneCreatedCode  RedisInfraCode = "StandaloneCreated"

	RedisSentinelDeletedCode    RedisInfraCode = "SentinelDeleted"
	RedisReplicationDeletedCode RedisInfraCode = "ReplicationDeleted"
	RedisStandaloneDeletedCode  RedisInfraCode = "StandaloneDeleted"

	RedisStandaloneConnectionCode RedisInfraCode = "StandaloneConnection"
	RedisSentinelConnectionCode   RedisInfraCode = "SentinelConnection"
)

func NewRedisStatusDetail(code RedisInfraCode, message string) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: Redis,
		code:      string(code),
		message:   message,
	}
}

type RedisStatusDetail struct {
	InfraStatusDetail
}

func (r RedisStatusDetail) ToRedisSentinelConnDetail() (RedisSentinelConnDetail, bool) {
	if RedisInfraCode(r.Code()) != RedisSentinelConnectionCode {
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
	if RedisInfraCode(r.Code()) != RedisStandaloneConnectionCode {
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

func NewRedisSentinelConnDetail(connInfo RedisSentinelConnInfo) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: Redis,
		code:      string(RedisSentinelConnectionCode),
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

func NewRedisStandaloneConnDetail(connInfo RedisStandaloneConnInfo) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: Redis,
		code:      string(RedisStandaloneConnectionCode),
		message:   fmt.Sprintf("redis://%s:%s", connInfo.Host, connInfo.Port),
		hidden:    connInfo,
	}
}

/////////////////////////////////////////////////
// WBRedisStatus translation

func ExtractRedisStatus(ctx context.Context, r *Results) RedisStatus {
	var ok bool
	var sentinelConnDetail RedisSentinelConnDetail
	var standaloneConnDetail RedisStandaloneConnDetail
	var result = RedisStatus{
		Errors: r.getRedisErrors(),
	}

	for _, detail := range r.getRedisStatusDetails() {

		if sentinelConnDetail, ok = detail.ToRedisSentinelConnDetail(); ok {
			result.Connection.SentinelHost = sentinelConnDetail.connInfo.SentinelHost
			result.Connection.SentinelPort = sentinelConnDetail.connInfo.SentinelPort
			result.Connection.SentinelMaster = sentinelConnDetail.connInfo.MasterName
		} else if standaloneConnDetail, ok = detail.ToRedisStandaloneConnDetail(); ok {
			result.Connection.RedisHost = standaloneConnDetail.connInfo.Host
			result.Connection.RedisPort = standaloneConnDetail.connInfo.Port
		} else {
			result.Details = append(result.Details, detail)
		}
	}

	if len(result.Errors) > 0 {
		result.Ready = false
	} else {
		result.Ready = result.Connection.RedisHost != "" || result.Connection.SentinelHost != ""
	}

	return result
}

func (r *Results) getRedisStatusDetails() []RedisStatusDetail {
	return utils.FilterMapFunc(r.StatusList, func(s InfraStatusDetail) (RedisStatusDetail, bool) { return s.ToRedisStatusDetail() })
}
