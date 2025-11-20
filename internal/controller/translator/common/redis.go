package common

import (
	"context"
	"errors"
	"fmt"

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

type RedisErrorCode string

const (
	RedisDeploymentConflictCode RedisErrorCode = "DeploymentConflict"
)

type RedisStatus struct {
	Ready      bool
	Connection RedisConnection
	Conditions []RedisCondition
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

func NewRedisCondition(code RedisInfraCode, message string) RedisCondition {
	return RedisCondition{
		code:    code,
		message: message,
	}
}

type RedisCondition struct {
	code    RedisInfraCode
	message string
	hidden  interface{}
}

func (r RedisCondition) Code() string {
	return string(r.code)
}

func (r RedisCondition) Message() string {
	return r.message
}

func (r RedisCondition) ToRedisSentinelConnCondition() (RedisSentinelConnCondition, bool) {
	if r.code != RedisSentinelConnectionCode {
		return RedisSentinelConnCondition{}, false
	}
	result := RedisSentinelConnCondition{}
	result.hidden = r.hidden
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

func (r RedisCondition) ToRedisStandaloneConnCondition() (RedisStandaloneConnCondition, bool) {
	if r.code != RedisStandaloneConnectionCode {
		return RedisStandaloneConnCondition{}, false
	}
	result := RedisStandaloneConnCondition{}
	result.hidden = r.hidden
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

func NewRedisSentinelConnCondition(connInfo RedisSentinelConnInfo) RedisCondition {
	return RedisCondition{
		code:    RedisSentinelConnectionCode,
		message: fmt.Sprintf("redis://%s:%s?master=%s", connInfo.SentinelHost, connInfo.SentinelPort, connInfo.MasterName),
		hidden:  connInfo,
	}
}

type RedisSentinelConnCondition struct {
	RedisCondition
	connInfo RedisSentinelConnInfo
}

type RedisStandaloneConnInfo struct {
	Host string
	Port string
}

type RedisStandaloneConnCondition struct {
	RedisCondition
	connInfo RedisStandaloneConnInfo
}

func NewRedisStandaloneConnCondition(connInfo RedisStandaloneConnInfo) RedisCondition {
	return RedisCondition{
		code:    RedisStandaloneConnectionCode,
		message: fmt.Sprintf("redis://%s:%s", connInfo.Host, connInfo.Port),
		hidden:  connInfo,
	}
}

/////////////////////////////////////////////////
// WBRedisStatus translation

func ExtractRedisStatus(ctx context.Context, conditions []RedisCondition) RedisStatus {
	var ok bool
	var sentinelConnCond RedisSentinelConnCondition
	var standaloneConnCond RedisStandaloneConnCondition
	var result = RedisStatus{}

	for _, cond := range conditions {

		if sentinelConnCond, ok = cond.ToRedisSentinelConnCondition(); ok {
			result.Connection.SentinelHost = sentinelConnCond.connInfo.SentinelHost
			result.Connection.SentinelPort = sentinelConnCond.connInfo.SentinelPort
			result.Connection.SentinelMaster = sentinelConnCond.connInfo.MasterName
		} else if standaloneConnCond, ok = cond.ToRedisStandaloneConnCondition(); ok {
			result.Connection.RedisHost = standaloneConnCond.connInfo.Host
			result.Connection.RedisPort = standaloneConnCond.connInfo.Port
		} else {
			result.Conditions = append(result.Conditions, cond)
		}
	}

	result.Ready = result.Connection.RedisHost != "" || result.Connection.SentinelHost != ""

	return result
}
