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

	// DefaultReplicas is the Keeper ensemble size used when unset. An odd number
	// is required so the ensemble can form a quorum.
	DefaultReplicas int32 = 3

	// DefaultStorageSize is the per-node PV size used when unset. Keeper state
	// (raft log + snapshots) is small.
	DefaultStorageSize = "10Gi"

	// KeeperCustomResourceType is the condition type reported for the CHK CR.
	KeeperCustomResourceType = "KeeperCustomResource"

	podTemplateName     = "keeper-pod-template"
	volumeTemplateName  = "keeper-data-volume"
	keeperContainerName = "clickhouse-keeper"

	keeperRunAsUser  int64 = 101
	keeperRunAsGroup int64 = 101
	keeperFSGroup    int64 = 101
)
