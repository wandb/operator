package v2

import (
	v2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator"
	corev1 "k8s.io/api/core/v1"
)

func toWbInfraStatus(s translator.InfraStatus) v2.WBInfraStatus {
	return v2.WBInfraStatus{
		Ready:      s.Ready,
		State:      s.State,
		Conditions: s.Conditions,
	}
}

func toTranslatorInfraStatus(s v2.WBInfraStatus) translator.InfraStatus {
	return translator.InfraStatus{
		Ready:      s.Ready,
		State:      s.State,
		Conditions: s.Conditions,
	}
}

func copySelector(s corev1.SecretKeySelector) corev1.SecretKeySelector {
	return s
}

// MySQL

func ToTranslatorMysqlConnection(c v2.MysqlConnection) translator.MysqlConnection {
	return translator.MysqlConnection{
		Host:     copySelector(c.Host),
		Port:     copySelector(c.Port),
		Database: copySelector(c.Database),
		Username: copySelector(c.Username),
		Password: copySelector(c.Password),
		Tls:      copySelector(c.Tls),
		SslCa:    copySelector(c.SslCa),
		SslCert:  copySelector(c.SslCert),
		SslKey:   copySelector(c.SslKey),
		URL:      copySelector(c.URL),
	}
}

func ToWbMysqlConnection(c translator.MysqlConnection) v2.MysqlConnection {
	return v2.MysqlConnection{
		Host:     copySelector(c.Host),
		Port:     copySelector(c.Port),
		Database: copySelector(c.Database),
		Username: copySelector(c.Username),
		Password: copySelector(c.Password),
		Tls:      copySelector(c.Tls),
		SslCa:    copySelector(c.SslCa),
		SslCert:  copySelector(c.SslCert),
		SslKey:   copySelector(c.SslKey),
		URL:      copySelector(c.URL),
	}
}

func ToWbMysqlInfraStatus(s translator.MysqlStatus) v2.MysqlInfraStatus {
	return v2.MysqlInfraStatus{
		WBInfraStatus: toWbInfraStatus(s.InfraStatus),
		Connection:    ToWbMysqlConnection(s.Connection),
	}
}

func ToTranslatorMysqlInfraStatus(s v2.MysqlInfraStatus) translator.MysqlStatus {
	return translator.MysqlStatus{
		InfraStatus: toTranslatorInfraStatus(s.WBInfraStatus),
		Connection:  ToTranslatorMysqlConnection(s.Connection),
	}
}

// Redis

func ToTranslatorRedisConnection(c v2.RedisConnection) translator.RedisConnection {
	return translator.RedisConnection{
		Host:     copySelector(c.Host),
		Port:     copySelector(c.Port),
		Password: copySelector(c.Password),
		Tls:      copySelector(c.Tls),
		SslCa:    copySelector(c.SslCa),
		URL:      copySelector(c.URL),
	}
}

func ToWbRedisConnection(c translator.RedisConnection) v2.RedisConnection {
	return v2.RedisConnection{
		Host:     copySelector(c.Host),
		Port:     copySelector(c.Port),
		Password: copySelector(c.Password),
		Tls:      copySelector(c.Tls),
		SslCa:    copySelector(c.SslCa),
		URL:      copySelector(c.URL),
	}
}

func ToWbRedisInfraStatus(s translator.RedisStatus) v2.RedisInfraStatus {
	return v2.RedisInfraStatus{
		WBInfraStatus: toWbInfraStatus(s.InfraStatus),
		Connection:    ToWbRedisConnection(s.Connection),
	}
}

func ToTranslatorRedisInfraStatus(s v2.RedisInfraStatus) translator.RedisStatus {
	return translator.RedisStatus{
		InfraStatus: toTranslatorInfraStatus(s.WBInfraStatus),
		Connection:  ToTranslatorRedisConnection(s.Connection),
	}
}

// Kafka

func ToTranslatorKafkaConnection(c v2.KafkaConnection) translator.KafkaConnection {
	return translator.KafkaConnection{
		Host:           copySelector(c.Host),
		Port:           copySelector(c.Port),
		BrokerEndpoint: copySelector(c.BrokerEndpoint),
		URL:            copySelector(c.URL),
	}
}

func ToWbKafkaConnection(c translator.KafkaConnection) v2.KafkaConnection {
	return v2.KafkaConnection{
		Host:           copySelector(c.Host),
		Port:           copySelector(c.Port),
		BrokerEndpoint: copySelector(c.BrokerEndpoint),
		URL:            copySelector(c.URL),
	}
}

func ToWbKafkaInfraStatus(s translator.KafkaStatus) v2.KafkaInfraStatus {
	return v2.KafkaInfraStatus{
		WBInfraStatus: toWbInfraStatus(s.InfraStatus),
		Connection:    ToWbKafkaConnection(s.Connection),
	}
}

func ToTranslatorKafkaInfraStatus(s v2.KafkaInfraStatus) translator.KafkaStatus {
	return translator.KafkaStatus{
		InfraStatus: toTranslatorInfraStatus(s.WBInfraStatus),
		Connection:  ToTranslatorKafkaConnection(s.Connection),
	}
}

// Minio

func ToTranslatorMinioConnection(c v2.MinioConnection) translator.MinioConnection {
	return translator.MinioConnection{
		Endpoint:  copySelector(c.Endpoint),
		AccessKey: copySelector(c.AccessKey),
		SecretKey: copySelector(c.SecretKey),
		Bucket:    copySelector(c.Bucket),
		Region:    copySelector(c.Region),
		URL:       copySelector(c.URL),
	}
}

func ToWbMinioConnection(c translator.MinioConnection) v2.MinioConnection {
	return v2.MinioConnection{
		Endpoint:  copySelector(c.Endpoint),
		AccessKey: copySelector(c.AccessKey),
		SecretKey: copySelector(c.SecretKey),
		Bucket:    copySelector(c.Bucket),
		Region:    copySelector(c.Region),
		URL:       copySelector(c.URL),
	}
}

func ToWbMinioInfraStatus(s translator.MinioStatus) v2.MinioInfraStatus {
	return v2.MinioInfraStatus{
		WBInfraStatus: toWbInfraStatus(s.InfraStatus),
		Connection:    ToWbMinioConnection(s.Connection),
	}
}

func ToTranslatorMinioInfraStatus(s v2.MinioInfraStatus) translator.MinioStatus {
	return translator.MinioStatus{
		InfraStatus: toTranslatorInfraStatus(s.WBInfraStatus),
		Connection:  ToTranslatorMinioConnection(s.Connection),
	}
}

// ClickHouse

func ToTranslatorClickHouseConnection(c v2.ClickHouseConnection) translator.ClickHouseConnection {
	return translator.ClickHouseConnection{
		Host:     copySelector(c.Host),
		Port:     copySelector(c.Port),
		Database: copySelector(c.Database),
		Username: copySelector(c.Username),
		Password: copySelector(c.Password),
		URL:      copySelector(c.URL),
	}
}

func ToWbClickHouseConnection(c translator.ClickHouseConnection) v2.ClickHouseConnection {
	return v2.ClickHouseConnection{
		Host:     copySelector(c.Host),
		Port:     copySelector(c.Port),
		Database: copySelector(c.Database),
		Username: copySelector(c.Username),
		Password: copySelector(c.Password),
		URL:      copySelector(c.URL),
	}
}

func ToWbClickHouseInfraStatus(s translator.ClickHouseStatus) v2.ClickHouseInfraStatus {
	return v2.ClickHouseInfraStatus{
		WBInfraStatus: toWbInfraStatus(s.InfraStatus),
		Connection:    ToWbClickHouseConnection(s.Connection),
	}
}

func ToTranslatorClickHouseInfraStatus(s v2.ClickHouseInfraStatus) translator.ClickHouseStatus {
	return translator.ClickHouseStatus{
		InfraStatus: toTranslatorInfraStatus(s.WBInfraStatus),
		Connection:  ToTranslatorClickHouseConnection(s.Connection),
	}
}
