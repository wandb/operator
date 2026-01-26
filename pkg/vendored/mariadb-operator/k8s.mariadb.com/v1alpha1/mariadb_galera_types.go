package v1alpha1

import (
	k8s_mariadb_com "github.com/wandb/operator/pkg/vendored/mariadb-operator/k8s.mariadb.com"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// SST is the Snapshot State Transfer used when new Pods join the cluster.
// More info: https://galeracluster.com/library/documentation/sst.html.
type SST string

const (
	// SSTRsync is an SST based on rsync.
	SSTRsync SST = "rsync"
	// SSTMariaBackup is an SST based on mariabackup. It is the recommended SST.
	SSTMariaBackup SST = "mariabackup"
	// SSTMysqldump is an SST based on mysqldump.
	SSTMysqldump SST = "mysqldump"
)

// Validate returns an error if the SST is not valid.

// MariaDBFormat formats the SST so it can be used in Galera config files.

// PrimaryGalera is the Galera configuration for the primary node.
type PrimaryGalera struct {
	// PodIndex is the StatefulSet index of the primary node. The user may change this field to perform a manual switchover.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodIndex *int `json:"podIndex,omitempty"`
	// AutoFailover indicates whether the operator should automatically update PodIndex to perform an automatic primary failover.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	AutoFailover *bool `json:"autoFailover,omitempty"`
}

// SetDefaults sets reasonable defaults.

// GaleraInitJob defines a Job used to be used to initialize the Galera cluster.
type GaleraInitJob struct {
	// Metadata defines additional metadata for the Galera init Job.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Metadata *Metadata `json:"metadata,omitempty"`
	// Resources describes the compute resource requirements.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *ResourceRequirements `json:"resources,omitempty"`
}

// GaleraRecoveryJob defines a Job used to be used to recover the Galera cluster.
type GaleraRecoveryJob struct {
	// Metadata defines additional metadata for the Galera recovery Jobs.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Metadata *Metadata `json:"metadata,omitempty"`
	// Resources describes the compute resource requirements.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *ResourceRequirements `json:"resources,omitempty"`
	// PodAffinity indicates whether the recovery Jobs should run in the same Node as the MariaDB Pods. It defaults to true.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	PodAffinity *bool `json:"podAffinity,omitempty"`
}

// GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.
// More info: https://galeracluster.com/library/documentation/crash-recovery.html.
type GaleraRecovery struct {
	// Enabled is a flag to enable GaleraRecovery.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`
	// MinClusterSize is the minimum number of replicas to consider the cluster healthy. It can be either a number of replicas (1) or a percentage (50%).
	// If Galera consistently reports less replicas than this value for the given 'ClusterHealthyTimeout' interval, a cluster recovery is initiated.
	// It defaults to '1' replica, and it is highly recommendeded to keep this value at '1' in most cases.
	// If set to more than one replica, the cluster recovery process may restart the healthy replicas as well.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MinClusterSize *intstr.IntOrString `json:"minClusterSize,omitempty"`
	// ClusterMonitorInterval represents the interval used to monitor the Galera cluster health.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ClusterMonitorInterval *metav1.Duration `json:"clusterMonitorInterval,omitempty"`
	// ClusterHealthyTimeout represents the duration at which a Galera cluster, that consistently failed health checks,
	// is considered unhealthy, and consequently the Galera recovery process will be initiated by the operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ClusterHealthyTimeout *metav1.Duration `json:"clusterHealthyTimeout,omitempty"`
	// ClusterBootstrapTimeout is the time limit for bootstrapping a cluster.
	// Once this timeout is reached, the Galera recovery state is reset and a new cluster bootstrap will be attempted.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ClusterBootstrapTimeout *metav1.Duration `json:"clusterBootstrapTimeout,omitempty"`
	// ClusterUpscaleTimeout represents the maximum duration for upscaling the cluster's StatefulSet during the recovery process.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ClusterUpscaleTimeout *metav1.Duration `json:"clusterUpscaleTimeout,omitempty"`
	// ClusterDownscaleTimeout represents the maximum duration for downscaling the cluster's StatefulSet during the recovery process.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ClusterDownscaleTimeout *metav1.Duration `json:"clusterDownscaleTimeout,omitempty"`
	// PodRecoveryTimeout is the time limit for recevorying the sequence of a Pod during the cluster recovery.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodRecoveryTimeout *metav1.Duration `json:"podRecoveryTimeout,omitempty"`
	// PodSyncTimeout is the time limit for a Pod to join the cluster after having performed a cluster bootstrap during the cluster recovery.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodSyncTimeout *metav1.Duration `json:"podSyncTimeout,omitempty"`
	// ForceClusterBootstrapInPod allows you to manually initiate the bootstrap process in a specific Pod.
	// IMPORTANT: Use this option only in exceptional circumstances. Not selecting the Pod with the highest sequence number may result in data loss.
	// IMPORTANT: Ensure you unset this field after completing the bootstrap to allow the operator to choose the appropriate Pod to bootstrap from in an event of cluster recovery.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ForceClusterBootstrapInPod *string `json:"forceClusterBootstrapInPod,omitempty"`
	// Job defines a Job that co-operates with mariadb-operator by performing the Galera cluster recovery .
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Job *GaleraRecoveryJob `json:"job,omitempty"`
}

// Validate determines whether a GaleraRecovery is valid.

// SetDefaults sets reasonable defaults.

// HasMinClusterSize returns whether the current cluster has the minimum number of replicas. If not, a cluster recovery will be performed.

// GaleraConfig defines storage options for the Galera configuration files.
type GaleraConfig struct {
	// ReuseStorageVolume indicates that storage volume used by MariaDB should be reused to store the Galera configuration files.
	// It defaults to false, which implies that a dedicated volume for the Galera configuration files is provisioned.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	ReuseStorageVolume *bool `json:"reuseStorageVolume,omitempty" webhook:"inmutableinit"`
	// VolumeClaimTemplate is a template for the PVC that will contain the Galera configuration files shared between the InitContainer, Agent and MariaDB.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	VolumeClaimTemplate *VolumeClaimTemplate `json:"volumeClaimTemplate,omitempty" webhook:"inmutableinit"`
}

// SetDefaults sets reasonable defaults.

// Galera allows you to enable multi-master HA via Galera in your MariaDB cluster.
type Galera struct {
	// GaleraSpec is the Galera desired state specification.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	GaleraSpec `json:",inline"`
	// Enabled is a flag to enable Galera.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
}

// SetDefaults sets reasonable defaults.

// GaleraSpec is the Galera desired state specification.
type GaleraSpec struct {
	// Primary is the Galera configuration for the primary node.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Primary PrimaryGalera `json:"primary,omitempty"`
	// SST is the Snapshot State Transfer used when new Pods join the cluster.
	// More info: https://galeracluster.com/library/documentation/sst.html.
	// +optional
	// +kubebuilder:validation:Enum=rsync;mariabackup;mysqldump
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SST SST `json:"sst,omitempty"`
	// AvailableWhenDonor indicates whether a donor node should be responding to queries. It defaults to false.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	AvailableWhenDonor *bool `json:"availableWhenDonor,omitempty"`
	// GaleraLibPath is a path inside the MariaDB image to the wsrep provider plugin. It is defaulted if not provided.
	// More info: https://galeracluster.com/library/documentation/mysql-wsrep-options.html#wsrep-provider.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	GaleraLibPath string `json:"galeraLibPath,omitempty"`
	// ReplicaThreads is the number of replica threads used to apply Galera write sets in parallel.
	// More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_slave_threads.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ReplicaThreads int `json:"replicaThreads,omitempty"`
	// ProviderOptions is map of Galera configuration parameters.
	// More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_provider_options.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ProviderOptions map[string]string `json:"providerOptions,omitempty"`
	// Agent is a sidecar agent that co-operates with mariadb-operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Agent Agent `json:"agent,omitempty"`
	// GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.
	// More info: https://galeracluster.com/library/documentation/crash-recovery.html.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Recovery *GaleraRecovery `json:"recovery,omitempty"`
	// InitContainer is an init container that runs in the MariaDB Pod and co-operates with mariadb-operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InitContainer InitContainer `json:"initContainer,omitempty"`
	// InitJob defines a Job that co-operates with mariadb-operator by performing initialization tasks.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InitJob *GaleraInitJob `json:"initJob,omitempty"`
	// GaleraConfig defines storage options for the Galera configuration files.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Config GaleraConfig `json:"config,omitempty"`
}

// GaleraBootstrapStatus indicates when and in which Pod the cluster bootstrap process has been performed.
type GaleraBootstrapStatus struct {
	Time *metav1.Time `json:"time,omitempty"`
	Pod  *string      `json:"pod,omitempty"`
}

// GaleraRecoveryStatus is the current state of the Galera recovery process.
type GaleraRecoveryStatus struct {
	// State is a per Pod representation of the Galera state file (grastate.dat).
	State map[string]*k8s_mariadb_com.GaleraState `json:"state,omitempty"`
	// State is a per Pod representation of the sequence recovery process.
	Recovered map[string]*k8s_mariadb_com.Bootstrap `json:"recovered,omitempty"`
	// Bootstrap indicates when and in which Pod the cluster bootstrap process has been performed.
	Bootstrap *GaleraBootstrapStatus `json:"bootstrap,omitempty"`
	// PodsRestarted that the Pods have been restarted after the cluster bootstrap.
	PodsRestarted *bool `json:"podsRestarted,omitempty"`
}

// HasGaleraReadyCondition indicates whether the MariaDB object has a GaleraReady status condition.
// This means that the Galera cluster is healthy.

// HasGaleraNotReadyCondition indicates whether the MariaDB object has a non GaleraReady status condition.
// This means that the Galera cluster is not healthy.

// HasGaleraConfiguredCondition indicates whether the MariaDB object has a GaleraConfigured status condition.
// This means that the cluster has been successfully configured the first time.

// IsGaleraInitialized indicates that the Galera init Job has successfully completed.

// IsGaleraInitializing indicates that the Galera init Job is running.
