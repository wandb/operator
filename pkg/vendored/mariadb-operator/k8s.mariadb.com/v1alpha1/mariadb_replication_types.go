package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WaitPoint defines whether the transaction should wait for ACK before committing to the storage engine.
// More info: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point.
type WaitPoint string

const (
	// WaitPointAfterSync indicates that the primary waits for the replica ACK before committing the transaction to the storage engine.
	// It trades off performance for consistency.
	WaitPointAfterSync WaitPoint = "AfterSync"
	// WaitPointAfterCommit indicates that the primary commits the transaction to the storage engine and waits for the replica ACK afterwards.
	// It trades off consistency for performance.
	WaitPointAfterCommit WaitPoint = "AfterCommit"
)

// Validate returns an error if the WaitPoint is not valid.

// MariaDBFormat formats the WaitPoint so it can be used in MariaDB config files.

// Gtid indicates which Global Transaction ID (GTID) position mode should be used when connecting a replica to the master.
// See: https://mariadb.com/kb/en/gtid/#using-current_pos-vs-slave_pos.
type Gtid string

const (
	// GtidCurrentPos indicates the union of gtid_binlog_pos and gtid_slave_pos will be used when replicating from master.
	GtidCurrentPos Gtid = "CurrentPos"
	// GtidSlavePos indicates that gtid_slave_pos will be used when replicating from master.
	GtidSlavePos Gtid = "SlavePos"
)

// Validate returns an error if the Gtid is not valid.

// MariaDBFormat formats the Gtid so it can be used in MariaDB config files.

// PrimaryReplication is the replication configuration and operation parameters for the primary.
type PrimaryReplication struct {
	// PodIndex is the StatefulSet index of the primary node. The user may change this field to perform a manual switchover.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodIndex *int `json:"podIndex,omitempty"`
	// AutoFailover indicates whether the operator should automatically update PodIndex to perform an automatic primary failover.
	// It is enabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	AutoFailover *bool `json:"autoFailover,omitempty"`
	// AutoFailoverDelay indicates the duration before performing an automatic primary failover.
	// By default, no extra delay is added.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	AutoFailoverDelay *metav1.Duration `json:"autoFailoverDelay,omitempty"`
}

// SetDefaults fills the current PrimaryReplication object with DefaultReplicationSpec.
// This enables having minimal PrimaryReplication objects and provides sensible defaults.

// ReplicaBootstrapFrom defines the sources for bootstrapping new relicas.
type ReplicaBootstrapFrom struct {
	// PhysicalBackupTemplateRef is a reference to a PhysicalBackup object that will be used as template to create a new PhysicalBackup object
	// used synchronize the data from an up to date replica to the new replica to be bootstrapped.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PhysicalBackupTemplateRef LocalObjectReference `json:"physicalBackupTemplateRef"`
	// RestoreJob defines additional properties for the Job used to perform the restoration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	RestoreJob *Job `json:"restoreJob,omitempty"`
}

// ReplicaRecovery defines how the replicas should be recovered after they enter an error state.
type ReplicaRecovery struct {
	// Enabled is a flag to enable replica recovery.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Enabled bool `json:"enabled"`
	// ErrorDurationThreshold defines the time duration after which, if a replica continues to report errors,
	// the operator will initiate the recovery process for that replica.
	// This threshold applies only to error codes not identified as recoverable by the operator.
	// Errors identified as recoverable will trigger the recovery process immediately.
	// It defaults to 5 minutes.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ErrorDurationThreshold *metav1.Duration `json:"errorDurationThreshold,omitempty"`
}

// ReplicaReplication is the replication configuration and operation parameters for the replicas.
type ReplicaReplication struct {
	// ReplPasswordSecretKeyRef provides a reference to the Secret to use as password for the replication user.
	// By default, a random password will be generated.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplPasswordSecretKeyRef *GeneratedSecretKeyRef `json:"replPasswordSecretKeyRef,omitempty"`
	// Gtid indicates which Global Transaction ID (GTID) position mode should be used when connecting a replica to the master.
	// By default, CurrentPos is used.
	// See: https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/replication-statements/change-master-to#master_use_gtid.
	// +optional
	// +kubebuilder:validation:Enum=CurrentPos;SlavePos
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Gtid *Gtid `json:"gtid,omitempty"`
	// ConnectionRetrySeconds is the number of seconds that the replica will wait between connection retries.
	// See: https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/replication-statements/change-master-to#master_connect_retry.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	ConnectionRetrySeconds *int `json:"connectionRetrySeconds,omitempty"`
	// MaxLagSeconds is the maximum number of seconds that replicas are allowed to lag behind the primary.
	// If a replica exceeds this threshold, it is marked as not ready and read queries will no longer be forwarded to it.
	// If not provided, it defaults to 0, which means that replicas are not allowed to lag behind the primary (recommended).
	// Lagged replicas will not be taken into account as candidates for the new primary during failover,
	// and they will block other operations, such as switchover and upgrade.
	// This field is not taken into account by MaxScale, you can define the maximum lag as router parameters.
	// See: https://mariadb.com/docs/maxscale/reference/maxscale-routers/maxscale-readwritesplit#max_replication_lag.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	MaxLagSeconds *int `json:"maxLagSeconds,omitempty"`
	// SyncTimeout defines the timeout for the synchronization phase during switchover and failover operations.
	// During switchover, all replicas must be synced with the current primary before promoting the new primary.
	// During failover, the new primary must be synced before being promoted as primary. This implies processing all the events in the relay log.
	// When the timeout is reached, the operator restarts the operation from the beginning.
	// It defaults to 10s.
	// See: https://mariadb.com/docs/server/reference/sql-functions/secondary-functions/miscellaneous-functions/master_gtid_wait
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SyncTimeout *metav1.Duration `json:"syncTimeout,omitempty"`
	// ReplicaBootstrapFrom defines the data sources used to bootstrap new replicas.
	// This will be used as part of the scaling out and recovery operations, when new replicas are created.
	// If not provided, scale out and recovery operations will return an error.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicaBootstrapFrom *ReplicaBootstrapFrom `json:"bootstrapFrom,omitempty"`
	// ReplicaRecovery defines how the replicas should be recovered after they enter an error state.
	// This process deletes data from faulty replicas and recreates them using the source defined in the bootstrapFrom field.
	// It is disabled by default, and it requires the bootstrapFrom field to be set.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicaRecovery *ReplicaRecovery `json:"recovery,omitempty"`
}

// SetDefaults fills the current ReplicaReplication object with DefaultReplicationSpec.
// This enables having minimal ReplicaReplication objects and provides sensible defaults.

// Validate returns an error if the ReplicaReplication is not valid.

// Replication defines replication configuration for a MariaDB cluster.
type Replication struct {
	// ReplicationSpec is the Replication desired state specification.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicationSpec `json:",inline"`
	// Enabled is a flag to enable replication.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
}

// ReplicationSpec is the replication desired state.
type ReplicationSpec struct {
	// Primary is the replication configuration for the primary node.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Primary PrimaryReplication `json:"primary,omitempty"`
	// ReplicaReplication is the replication configuration for the replica nodes.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Replica ReplicaReplication `json:"replica,omitempty"`
	// GtidStrictMode determines whether the GTID strict mode is enabled.
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid#gtid_strict_mode.
	// It is enabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	GtidStrictMode *bool `json:"gtidStrictMode,omitempty"`
	// SemiSyncEnabled determines whether semi-synchronous replication is enabled.
	// Semi-synchronous replication requires that at least one replica should have sent an ACK to the primary node
	// before committing the transaction back to the client.
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication
	// It is enabled by default
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SemiSyncEnabled *bool `json:"semiSyncEnabled,omitempty"`
	// SemiSyncAckTimeout for the replica to acknowledge transactions to the primary.
	// It requires semi-synchronous replication to be enabled.
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication#rpl_semi_sync_master_timeout
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SemiSyncAckTimeout *metav1.Duration `json:"semiSyncAckTimeout,omitempty"`
	// SemiSyncWaitPoint determines whether the transaction should wait for an ACK after having synced the binlog (AfterSync)
	// or after having committed to the storage engine (AfterCommit, the default).
	// It requires semi-synchronous replication to be enabled.
	// See: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point.
	// +optional
	// +kubebuilder:validation:Enum=AfterSync;AfterCommit
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SemiSyncWaitPoint *WaitPoint `json:"semiSyncWaitPoint,omitempty"`
	// SyncBinlog indicates after how many events the binary log is synchronized to the disk.
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-and-binary-log-system-variables#sync_binlog
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	SyncBinlog *int `json:"syncBinlog,omitempty"`
	// InitContainer is an init container that runs in the MariaDB Pod and co-operates with mariadb-operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InitContainer InitContainer `json:"initContainer,omitempty"`
	// Agent is a sidecar agent that runs in the MariaDB Pod and co-operates with mariadb-operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Agent Agent `json:"agent,omitempty"`
	// StandaloneProbes indicates whether to use the default non-HA startup and liveness probes.
	// It is disabled by default
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	StandaloneProbes *bool `json:"standaloneProbes,omitempty"`
}

// IsGtidStrictModeEnabled determines whether GTID strict mode is enabled.

// IsSemiSyncEnabled determines whether semi-synchronous replication is enabled.

// Validate determines whether replication config is valid.

// SetDefaults sets reasonable defaults for replication.

// HasConfiguredReplication indicates whether the MariaDB object has a ConditionTypeReplicationConfigured status condition.
// This means that replication has been successfully configured for the first time.

// HasConfiguredReplica indicates whether the cluster has a configured replica.

// IsConfiguredReplica determines whether a specific replica has been configured.

// IsReplicaRecoveryEnabled indicates if the replica recovery is enabled

// IsRecoveringReplicas indicates that a replica is being recovered.

// ReplicaRecoveryError indicates that the MariaDB instance has a replica recovery error.

// SetReplicaToRecover sets the replica to be recovered

// IsReplicaBeingRecovered indicates whether a replica is being recovered

// GetAutomaticFailoverDelay returns the duration of the automatic failover delay.

// IsSwitchingPrimary indicates whether a primary swichover operation is in progress.

// IsReplicationSwitchoverRequired indicates that a primary switchover operation is required.

// ReplicationRole represents the observed replication roles.
type ReplicationRole string

const (
	ReplicationRolePrimary ReplicationRole = "Primary"
	ReplicationRoleReplica ReplicationRole = "Replica"
	ReplicationRoleUnknown ReplicationRole = "Unknown"
)

// ReplicaStatusVars is the observed replica status variables.
type ReplicaStatusVars struct {
	// LastIOErrno is the error code returned by the IO thread.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastIOErrno *int `json:"lastIOErrno,omitempty"`
	// LastIOErrno is the error message returned by the IO thread.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastIOError *string `json:"lastIOError,omitempty"`
	// LastSQLErrno is the error code returned by the SQL thread.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastSQLErrno *int `json:"lastSQLErrno,omitempty"`
	// LastSQLError is the error message returned by the SQL thread.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastSQLError *string `json:"lastSQLError,omitempty"`
	// SlaveIORunning indicates whether the slave IO thread is running.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	SlaveIORunning *bool `json:"slaveIORunning,omitempty"`
	// SlaveSQLRunning indicates whether the slave SQL thread is running.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	SlaveSQLRunning *bool `json:"slaveSQLRunning,omitempty"`
	// SecondsBehindMaster measures the replication lag with the primary.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	SecondsBehindMaster *int `json:"secondsBehindMaster,omitempty"`
	// GtidIOPos is the last GTID position received by the IO thread and written to the relay log.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	GtidIOPos *string `json:"gtidIOPos,omitempty"`
	// GtidCurrentPos is the last GTID position executed by the SQL thread.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	GtidCurrentPos *string `json:"gtidCurrentPos,omitempty"`
}

// EqualErrors determines equality of error codes.

// ReplicaStatus is the observed replica status.
type ReplicaStatus struct {
	// ReplicaStatusVars is the observed replica status variables.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicaStatusVars `json:",inline"`
	// LastErrorTransitionTime is the last time the replica transitioned to an error state.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastErrorTransitionTime metav1.Time `json:"lastErrorTransitionTime,omitempty"`
}

// ReplicationStatus is the replication current state.
type ReplicationStatus struct {
	// Roles is the observed replication roles for each Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Roles map[string]ReplicationRole `json:"roles,omitempty"`
	// Replicas is the observed replication status for each replica.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Replicas map[string]ReplicaStatus `json:"replicas,omitempty"`
	// ReplicaToRecover is the replica that is being recovered by the operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ReplicaToRecover *string `json:"replicaToRecover,omitempty"`
}

// UseStandaloneProbes indicates whether to use the default non-HA startup and liveness probes.
