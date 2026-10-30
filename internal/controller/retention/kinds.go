package retention

import "k8s.io/apimachinery/pkg/runtime/schema"

// WatchedKinds includes generated roots and supporting objects. Management is
// routed by installation UID instead of a garbage-collection owner reference.
func WatchedKinds() []schema.GroupVersionKind {
	return []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "Secret"},
		{Group: "", Version: "v1", Kind: "ConfigMap"},
		{Group: "", Version: "v1", Kind: "ServiceAccount"},
		{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"},
		{Group: "apps.wandb.com", Version: "v2", Kind: "Application"},
		{Group: "moco.cybozu.com", Version: "v1beta2", Kind: "MySQLCluster"},
		{Group: "redis.redis.opstreelabs.in", Version: "v1beta2", Kind: "Redis"},
		{Group: "redis.redis.opstreelabs.in", Version: "v1beta2", Kind: "RedisSentinel"},
		{Group: "redis.redis.opstreelabs.in", Version: "v1beta2", Kind: "RedisReplication"},
		{Group: "seaweed.seaweedfs.com", Version: "v1", Kind: "Seaweed"},
		{Group: "clickhouse.altinity.com", Version: "v1", Kind: "ClickHouseInstallation"},
		{Group: "clickhouse-keeper.altinity.com", Version: "v1", Kind: "ClickHouseKeeperInstallation"},
	}
}
