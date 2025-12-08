package translator

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// RedisStatus is a representation of Status that must support round-trip translation
// between any version of WBRedisStatus and itself
type RedisStatus struct {
	Ready      bool
	State      string
	Conditions []metav1.Condition
	Connection InfraConnection
}

type RedisConnection struct {
	URL corev1.SecretKeySelector
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

	connInfo, ok := r.hidden.(RedisConnection)
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

	connInfo, ok := r.hidden.(RedisConnection)
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

func NewRedisSentinelConnCondition(connInfo RedisConnection) RedisCondition {
	return RedisCondition{
		code:    RedisSentinelConnectionCode,
		message: "Redis Sentinel connection info",
		hidden:  connInfo,
	}
}

type RedisSentinelConnCondition struct {
	RedisCondition
	connInfo RedisConnection
}

type RedisStandaloneConnCondition struct {
	RedisCondition
	connInfo RedisConnection
}

func NewRedisStandaloneConnCondition(connInfo RedisConnection) RedisCondition {
	return RedisCondition{
		code:    RedisStandaloneConnectionCode,
		message: "Redis Standalone connection info",
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
			result.Connection = sentinelConnCond.connInfo
		} else if standaloneConnCond, ok = cond.ToRedisStandaloneConnCondition(); ok {
			result.Connection = standaloneConnCond.connInfo
		} else {
			result.Conditions = append(result.Conditions, cond)
		}
	}

	result.Ready = result.Connection.URL.Name != ""

	return result
}
