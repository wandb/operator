package translator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RedisStandaloneImage  = "quay.io/opstree/redis:v7.0.15"
	RedisReplicationImage = "quay.io/opstree/redis:v7.0.15"
	RedisSentinelImage    = "quay.io/opstree/redis-sentinel:v7.0.12"
)

/////////////////////////////////////////////////
// Redis Status

// RedisStatus is a representation of Status that must support round-trip translation
// between any version of WBRedisStatus and itself
type RedisStatus struct {
	Ready      bool
	State      string
	Conditions []metav1.Condition
	Connection InfraConnection
}
