package altinity

const (
	// Resource names
	CHIName        = "wandb-clickhouse"
	ServiceName    = "clickhouse-wandb-clickhouse"
	ConnectionName = "wandb-clickhouse-connection"

	// ClickHouse configuration
	ClickHouseNativePort = 9000
	ClickHouseHTTPPort   = 8123
	ClickHouseUser       = "test_user"
	ClickHousePassword   = "test_password"

	// Cluster configuration
	ClusterName        = "cluster"
	ShardsCount        = 1
	VolumeTemplateName = "default-volume"
)
