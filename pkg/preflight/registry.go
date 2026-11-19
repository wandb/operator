package preflight

import "slices"

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

	ClusterCapacityCheck        = "clusterCapacityCheck"
	ExternalMysqlInClusterCheck = "externalMysqlInClusterCheck"
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

	// Run before apply by callers that walk the whole registry (wsm). Reconcile doesn't gate on them: the webhook
	// already validates literal proxies, and reconcile loads the manifest itself before anything else.
	ProxyCheck: typedCheck(ProxyCheck, "spec.global.proxy",
		TypeStatic, []ResourceReq{ReqReadOnly}, RunProxyCheck),
	CustomCACertsCheck: typedCheck(CustomCACertsCheck, "spec.global.customCACerts",
		TypeStatic, []ResourceReq{ReqReadOnly}, RunCustomCACertsCheck),
	// Matches "spec" rather than "spec.wandb" so a CR that omits spec.wandb still fails instead of matching nothing.
	ServerManifestCheck: typedCheck(ServerManifestCheck, "spec",
		TypeConnection, []ResourceReq{ReqReadOnly}, RunServerManifestCheck),

	// In Cluster Checks
	ClusterCapacityCheck:        withDependsOn(typedCheck(ClusterCapacityCheck, "spec", TypeCluster, []ResourceReq{ReqReadOnly}, RunClusterCapacityCheck), ServerManifestCheck),
	ExternalMysqlInClusterCheck: withCovers(typedCheck(ExternalMysqlInClusterCheck, "spec.mysql.*.externalMysql", TypeCluster, []ResourceReq{ReqResourceCreate}, RunExternalMysqlInClusterCheck), ExternalMysqlCheck),
}

// withDependsOn makes c report SKIPPED unless every named check passes first on the same run.
func withDependsOn(c Check, names ...string) Check {
	c.DependsOn = append(slices.Clone(c.DependsOn), names...)
	return c
}

// withCovers lets c's result stand in for unreachable failures of each named check on the same FieldPath.
func withCovers(c Check, names ...string) Check {
	c.Covers = append(slices.Clone(c.Covers), names...)
	return c
}
