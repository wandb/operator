package logx

import "log/slog"

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

var overrides = map[string]slog.Level{
	//ReconcileInfraV2:  slog.LevelDebug,
	//ReconcileAppV2:    slog.LevelDebug,
	Kafka: slog.LevelDebug,
	//Mysql:             slog.LevelDebug,
	//Redis:             slog.LevelDebug,
	//Minio:             slog.LevelDebug,
	//ClickHouse:        slog.LevelDebug,
	//DefaultingWebhook: slog.LevelDebug,
	//ValidatingWebhook: slog.LevelDebug,
}
