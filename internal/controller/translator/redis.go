package translator

import (
	corev1 "k8s.io/api/core/v1"
)

const RedisModuleName = "redis"

const (
	RedisStandaloneImage  = "quay.io/opstree/redis:v7.0.15"
	RedisReplicationImage = "quay.io/opstree/redis:v7.0.15"
	RedisSentinelImage    = "quay.io/opstree/redis-sentinel:v7.0.12"
)

/////////////////////////////////////////////////
// Redis Connection

type RedisConnection struct {
	Host     corev1.SecretKeySelector
	Port     corev1.SecretKeySelector
	Password corev1.SecretKeySelector
	Tls      corev1.SecretKeySelector
	SslCa    corev1.SecretKeySelector
	URL      corev1.SecretKeySelector
}

/////////////////////////////////////////////////
// Redis Status

type RedisStatus struct {
	InfraStatus
	Connection RedisConnection
}
