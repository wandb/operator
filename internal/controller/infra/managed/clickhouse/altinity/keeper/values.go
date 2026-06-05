package keeper

const (
	// KeeperModuleName is the W&B component label value for Keeper resources.
	KeeperModuleName = "clickhouse-keeper"

	// KeeperImage pins the Altinity ClickHouse Keeper image to the same version
	// as the managed ClickHouse server image (altinity.ClickHouseImage).
	KeeperImage = "altinity/clickhouse-keeper:25.8.16.10002.altinitystable"

	// KeeperClientPort is the ZooKeeper-compatible client port ClickHouse
	// connects to for replication coordination.
	KeeperClientPort = 2181

	// ClusterName is the name of the single Keeper cluster.
	ClusterName = "default"

	// KeeperCustomResourceType is the condition type reported for the CHK CR.
	KeeperCustomResourceType = "KeeperCustomResource"

	// KeeperReportedReadyType reports whether the Keeper ensemble's pods are
	// running. ClickHouse depends on Keeper for ReplicatedMergeTree coordination,
	// so this condition gates ClickHouse readiness.
	KeeperReportedReadyType = "KeeperReportedReady"

	podTemplateName     = "keeper-pod-template"
	volumeTemplateName  = "keeper-data-volume"
	keeperContainerName = "clickhouse-keeper"

	keeperRunAsUser  int64 = 101
	keeperRunAsGroup int64 = 101
	keeperFSGroup    int64 = 101
)
