package translator

import (
	corev1 "k8s.io/api/core/v1"
)

const ClickhouseModuleName = "clickhouse"

/////////////////////////////////////////////////
// ClickHouse Connection

type ClickHouseConnection struct {
	Host     corev1.SecretKeySelector
	Port     corev1.SecretKeySelector
	Database corev1.SecretKeySelector
	Username corev1.SecretKeySelector
	Password corev1.SecretKeySelector
	URL      corev1.SecretKeySelector
}

/////////////////////////////////////////////////
// ClickHouse Status

type ClickHouseStatus struct {
	InfraStatus
	Connection ClickHouseConnection
}
