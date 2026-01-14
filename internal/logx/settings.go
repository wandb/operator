package logx

import "go.uber.org/zap/zapcore"

/////////////////////////////////////
// Names, Defaults

const (
	ReconcileInfraV2  = "reconcile-infraV2"
	ReconcileAppV2    = "reconcile-appV2"
	Kafka             = "kafka-reconcile"
	Mysql             = "mysql-reconcile"
	Redis             = "redis-reconcile"
	Minio             = "minio-reconcile"
	ClickHouse        = "clickhouse-reconcile"
	DefaultingWebhook = "defaulting-webhook"
	ValidatingWebhook = "validating-webhook"
)

var overrides = map[string]zapcore.Level{
	//ReconcileInfraV2:  zapcore.DebugLevel,
	//ReconcileAppV2:    zapcore.DebugLevel,
	Kafka: zapcore.DebugLevel,
	//Mysql:             zapcore.DebugLevel,
	//Redis:             zapcore.DebugLevel,
	//Minio:             zapcore.DebugLevel,
	//ClickHouse:        zapcore.DebugLevel,
	//DefaultingWebhook: zapcore.DebugLevel,
	//ValidatingWebhook: zapcore.DebugLevel,
}
