package logx

import "go.uber.org/zap/zapcore"

/////////////////////////////////////
// Names, Defaults

const (
	Webhook    = "webhook"
	Kafka      = "kafka"
	Mysql      = "mysql"
	Redis      = "redis"
	Minio      = "minio"
	ClickHouse = "clickhouse"
)

var overrides = map[string]zapcore.Level{
	Webhook: zapcore.InfoLevel,
	Kafka:   zapcore.DebugLevel,
	//Mysql:      zapcore.DebugLevel,
	//Redis:      zapcore.DebugLevel,
	//Minio:      zapcore.DebugLevel,
	//ClickHouse: zapcore.DebugLevel,
}
