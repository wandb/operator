package preflight

const (
	ExternalMysqlCheck       = "externalMysqlCheck"
	ExternalRedisCheck       = "externalRedisCheck"
	ExternalClickHouseCheck  = "externalClickHouseCheck"
	ExternalObjectStoreCheck = "externalObjectStoreCheck"

	ManagedMysqlAPIsCheck       = "managedMysqlApisCheck"
	ManagedRedisAPIsCheck       = "managedRedisApisCheck"
	ManagedObjectStoreAPIsCheck = "managedObjectStoreApisCheck"
	ManagedClickHouseAPIsCheck  = "managedClickHouseApisCheck"

	ProxyCheck          = "proxyCheck"
	CustomCACertsCheck  = "customCACertsCheck"
	ServerManifestCheck = "serverManifestCheck"
)

var Checks = map[string]Check{
	ExternalMysqlCheck: typedCheck(ExternalMysqlCheck, "spec.mysql.*.externalMysql",
		TypeConnection, []ResourceReq{ReqReadOnly}, RunExternalMysqlCheck),
	ExternalRedisCheck: typedCheck(ExternalRedisCheck, "spec.redis.*.externalRedis",
		TypeConnection, []ResourceReq{ReqReadOnly}, RunExternalRedisCheck),
	ExternalClickHouseCheck: typedCheck(ExternalClickHouseCheck, "spec.clickhouse.*.externalClickhouse",
		TypeConnection, []ResourceReq{ReqReadOnly}, RunExternalClickHouseCheck),
	ExternalObjectStoreCheck: typedCheck(ExternalObjectStoreCheck, "spec.objectStore.*.externalObjectStore",
		TypeConnection, []ResourceReq{ReqReadOnly}, RunExternalObjectStoreCheck),

	ManagedMysqlAPIsCheck: managedAPIsCheck(ManagedMysqlAPIsCheck, "spec.mysql.*.managedMysql", managedMysqlAPIs),
	ManagedRedisAPIsCheck: managedAPIsCheck(ManagedRedisAPIsCheck, "spec.redis.*.managedRedis", managedRedisAPIs),
	ManagedObjectStoreAPIsCheck: managedAPIsCheck(ManagedObjectStoreAPIsCheck, "spec.objectStore.*.managedObjectStore",
		managedObjectStoreAPIs),
	ManagedClickHouseAPIsCheck: managedAPIsCheck(ManagedClickHouseAPIsCheck, "spec.clickhouse.*.managedClickhouse",
		managedClickHouseAPIs),

	ProxyCheck: typedCheck(ProxyCheck, "spec.global.proxy",
		TypeStatic, []ResourceReq{ReqReadOnly}, RunProxyCheck),
	CustomCACertsCheck: typedCheck(CustomCACertsCheck, "spec.global.customCACerts",
		TypeStatic, []ResourceReq{ReqReadOnly}, RunCustomCACertsCheck),
	// Matches "spec" rather than "spec.wandb" so a CR that omits spec.wandb still fails instead of matching nothing.
	ServerManifestCheck: typedCheck(ServerManifestCheck, "spec",
		TypeConnection, []ResourceReq{ReqReadOnly}, RunServerManifestCheck),
}
