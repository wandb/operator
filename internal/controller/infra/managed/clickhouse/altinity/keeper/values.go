package keeper

const (
	// KeeperModuleName is the W&B component label value for Keeper resources.
	KeeperModuleName = "clickhouse-keeper"

	// TODO: remove this hardcoded default once all supported manifest versions
	// supply clickhouseKeeper.<instance>.images.keeper. Pinned to the managed
	// ClickHouse server version.
	defaultKeeperImage = "altinity/clickhouse-keeper:25.8.16.10002.altinitystable"

	// KeeperClientPort is the ZooKeeper-compatible client port.
	KeeperClientPort = 2181

	// ClusterName is the name of the single Keeper cluster.
	ClusterName = "default"

	// KeeperCustomResourceType is the condition type reported for the CHK CR.
	KeeperCustomResourceType = "KeeperCustomResource"

	// KeeperReportedReadyType reports Keeper pod readiness; it gates ClickHouse readiness.
	KeeperReportedReadyType = "KeeperReportedReady"

	podTemplateName     = "keeper-pod-template"
	volumeTemplateName  = "keeper-data-volume"
	keeperContainerName = "clickhouse-keeper"
	keeperLogVolumeName = "keeper-log"
	keeperLogMountPath  = "/var/log/clickhouse-keeper"

	keeperRunAsUser  int64 = 101
	keeperRunAsGroup int64 = 101
	keeperFSGroup    int64 = 101
)
