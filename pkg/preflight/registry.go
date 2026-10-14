package preflight

import apiv2 "github.com/wandb/operator/api/v2"

const (
	ExternalMysqlCheck       = "externalMysqlCheck"
	ExternalRedisCheck       = "externalRedisCheck"
	ExternalObjectStoreCheck = "externalObjectStoreCheck"
)

var Checks = map[string]Check{
	ExternalMysqlCheck: typedCheck[apiv2.MysqlConnection](ExternalMysqlCheck, "spec.mysql.*.externalMysql", RunExternalMysqlCheck),
	// Register new infra checks against the CR struct at their field, e.g.:
	//   ExternalRedisCheck:       typedCheck[apiv2.RedisConnection](ExternalRedisCheck, "spec.redis.*.externalRedis", RunExternalRedisCheck),
	//   ExternalObjectStoreCheck: typedCheck[apiv2.ObjectStoreConnection](ExternalObjectStoreCheck, "spec.objectStore.*.externalObjectStore", RunExternalObjectStoreCheck),
}
