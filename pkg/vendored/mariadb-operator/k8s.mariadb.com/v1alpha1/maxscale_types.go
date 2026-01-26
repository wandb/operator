package v1alpha1

import (
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MaxScaleServer defines a MariaDB server to forward traffic to.
type MaxScaleServer struct {
	// Name is the identifier of the MariaDB server.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name"`
	// Address is the network address of the MariaDB server.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Address string `json:"address"`
	// Port is the network port of the MariaDB server. If not provided, it defaults to 3306.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	Port int32 `json:"port,omitempty"`
	// Protocol is the MaxScale protocol to use when communicating with this MariaDB server. If not provided, it defaults to MariaDBBackend.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Protocol string `json:"protocol,omitempty"`
	// Maintenance indicates whether the server is in maintenance mode.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Maintenance bool `json:"maintenance,omitempty"`
	// Params defines extra parameters to pass to the server.
	// Any parameter supported by MaxScale may be specified here. See reference:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#server_1.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Params map[string]string `json:"params,omitempty"`
}

// SetDefaults sets default values.

// MonitorModule defines the type of monitor module
type MonitorModule string

const (
	// MonitorModuleMariadb is a monitor to be used with MariaDB servers.
	MonitorModuleMariadb MonitorModule = "mariadbmon"
	// MonitorModuleGalera is a monitor to be used with Galera servers.
	MonitorModuleGalera MonitorModule = "galeramon"
)

// Validate determines whether a MonitorModule is valid.

// CooperativeMonitoring enables coordination between multiple MaxScale instances running monitors.
// See: https://mariadb.com/docs/server/architecture/components/maxscale/monitors/mariadbmon/use-cooperative-locking-ha-maxscale-mariadb-monitor/
type CooperativeMonitoring string

const (
	// CooperativeMonitoringMajorityOfAll requires a lock from the majority of the MariaDB servers, even the ones that are down.
	CooperativeMonitoringMajorityOfAll CooperativeMonitoring = "majority_of_all"
	// CooperativeMonitoringMajorityOfRunning requires a lock from the majority of the MariaDB servers.
	CooperativeMonitoringMajorityOfRunning CooperativeMonitoring = "majority_of_running"
)

// MaxScaleMonitor monitors MariaDB server instances
type MaxScaleMonitor struct {
	// SuspendTemplate defines how a resource can be suspended. Feature flag --feature-maxscale-suspend is required in the controller to enable this.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SuspendTemplate `json:",inline"`
	// Name is the identifier of the monitor. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name"`
	// Module is the module to use to monitor MariaDB servers. It is mandatory when no MariaDB reference is provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Module MonitorModule `json:"module" webhook:"inmutableinit"`
	// Interval used to monitor MariaDB servers. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Interval metav1.Duration `json:"interval,omitempty"`
	// CooperativeMonitoring enables coordination between multiple MaxScale instances running monitors. It is defaulted when HA is enabled.
	// +optional
	// +kubebuilder:validation:Enum=majority_of_all;majority_of_running
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	CooperativeMonitoring *CooperativeMonitoring `json:"cooperativeMonitoring,omitempty"`
	// Params defines extra parameters to pass to the monitor.
	// Any parameter supported by MaxScale may be specified here. See reference:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-common-monitor-parameters/.
	// Monitor specific parameter are also supported:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-galera-monitor/#galera-monitor-optional-parameters.
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-monitor/#configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Params map[string]string `json:"params,omitempty"`
}

// SetCondition sets a status condition to MaxScale

// MaxScaleListener defines how the MaxScale server will listen for connections.
type MaxScaleListener struct {
	// SuspendTemplate defines how a resource can be suspended. Feature flag --feature-maxscale-suspend is required in the controller to enable this.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SuspendTemplate `json:",inline"`
	// Name is the identifier of the listener. It is defaulted if not provided
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name"`
	// Port is the network port where the MaxScale server will listen.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	Port int32 `json:"port,omitempty" webhook:"inmutable"`
	// Protocol is the MaxScale protocol to use when communicating with the client. If not provided, it defaults to MariaDBProtocol.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Protocol string `json:"protocol,omitempty"`
	// Params defines extra parameters to pass to the listener.
	// Any parameter supported by MaxScale may be specified here. See reference:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#listener_1.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Params map[string]string `json:"params,omitempty"`
}

// SetDefaults sets default values.

// ServiceRouter defines the type of service router.
type ServiceRouter string

const (
	// ServiceRouterReadWriteSplit splits the load based on the queries. Write queries are performed on master and read queries on the replicas.
	ServiceRouterReadWriteSplit ServiceRouter = "readwritesplit"
	// ServiceRouterReadConnRoute splits the load based on the connections. Each connection is assigned to a server.
	ServiceRouterReadConnRoute ServiceRouter = "readconnroute"
)

// Services define how the traffic is forwarded to the MariaDB servers.
type MaxScaleService struct {
	// SuspendTemplate defines how a resource can be suspended. Feature flag --feature-maxscale-suspend is required in the controller to enable this.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SuspendTemplate `json:",inline"`
	// Name is the identifier of the MaxScale service.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name"`
	// Router is the type of router to use.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=readwritesplit;readconnroute
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Router ServiceRouter `json:"router" webhook:"inmutable"`
	// MaxScaleListener defines how the MaxScale server will listen for connections.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Listener MaxScaleListener `json:"listener"`
	// Params defines extra parameters to pass to the service.
	// Any parameter supported by MaxScale may be specified here. See reference:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#service_1.
	// Router specific parameter are also supported:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-readwritesplit/#configuration.
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-readconnroute/#configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Params map[string]string `json:"params,omitempty"`
}

// SetDefaults sets default values.

// MaxScaleAdmin configures the admin REST API and GUI.
type MaxScaleAdmin struct {
	// Port where the admin REST API and GUI will be exposed.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	Port int32 `json:"port"`
	// GuiEnabled indicates whether the admin GUI should be enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	GuiEnabled *bool `json:"guiEnabled,omitempty"`
}

// SetDefaults sets default values.

// MaxScaleConfigSync defines how the config changes are replicated across replicas.
type MaxScaleConfigSync struct {
	// Database is the MariaDB logical database where the 'maxscale_config' table will be created in order to persist and synchronize config changes. If not provided, it defaults to 'mysql'.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Database string `json:"database,omitempty"`
	// Interval defines the config synchronization interval. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Interval metav1.Duration `json:"interval,omitempty"`
	// Interval defines the config synchronization timeout. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Timeout metav1.Duration `json:"timeout,omitempty"`
}

// MaxScaleConfig defines the MaxScale configuration.
type MaxScaleConfig struct {
	// Params is a key value pair of parameters to be used in the MaxScale static configuration file.
	// Any parameter supported by MaxScale may be specified here. See reference:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#global-settings.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Params map[string]string `json:"params,omitempty"`
	// VolumeClaimTemplate provides a template to define the PVCs for storing MaxScale runtime configuration files. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	VolumeClaimTemplate VolumeClaimTemplate `json:"volumeClaimTemplate"`
	// Sync defines how to replicate configuration across MaxScale replicas. It is defaulted when HA is enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Sync *MaxScaleConfigSync `json:"sync,omitempty"`
}

// MaxScaleAuth defines the credentials required for MaxScale to connect to MariaDB.
type MaxScaleAuth struct {
	// Generate  defies whether the operator should generate users and grants for MaxScale to work.
	// It only supports MariaDBs specified via spec.mariaDbRef.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Generate *bool `json:"generate,omitempty" webhook:"inmutableinit"`
	// AdminUsername is an admin username to call the admin REST API. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	AdminUsername string `json:"adminUsername,omitempty" webhook:"inmutableinit"`
	// AdminPasswordSecretKeyRef is Secret key reference to the admin password to call the admin REST API. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	AdminPasswordSecretKeyRef GeneratedSecretKeyRef `json:"adminPasswordSecretKeyRef,omitempty" webhook:"inmutableinit"`
	// DeleteDefaultAdmin determines whether the default admin user should be deleted after the initial configuration. If not provided, it defaults to true.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	DeleteDefaultAdmin *bool `json:"deleteDefaultAdmin,omitempty" webhook:"inmutableinit"`
	// MetricsUsername is an metrics username to call the REST API. It is defaulted if metrics are enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	MetricsUsername string `json:"metricsUsername,omitempty" webhook:"inmutableinit"`
	// MetricsPasswordSecretKeyRef is Secret key reference to the metrics password to call the admib REST API. It is defaulted if metrics are enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	MetricsPasswordSecretKeyRef GeneratedSecretKeyRef `json:"metricsPasswordSecretKeyRef,omitempty"`
	// ClientUsername is the user to connect to MaxScale. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ClientUsername string `json:"clientUsername,omitempty" webhook:"inmutableinit"`
	// ClientPasswordSecretKeyRef is Secret key reference to the password to connect to MaxScale. It is defaulted if not provided.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ClientPasswordSecretKeyRef GeneratedSecretKeyRef `json:"clientPasswordSecretKeyRef,omitempty"`
	// ClientMaxConnections defines the maximum number of connections that the client can establish.
	// If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.
	// It defaults to 30 times the number of MaxScale replicas.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ClientMaxConnections int32 `json:"clientMaxConnections,omitempty" webhook:"inmutableinit"`
	// ServerUsername is the user used by MaxScale to connect to MariaDB server. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServerUsername string `json:"serverUsername,omitempty" webhook:"inmutableinit"`
	// ServerPasswordSecretKeyRef is Secret key reference to the password used by MaxScale to connect to MariaDB server. It is defaulted if not provided.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServerPasswordSecretKeyRef GeneratedSecretKeyRef `json:"serverPasswordSecretKeyRef,omitempty"`
	// ServerMaxConnections defines the maximum number of connections that the server can establish.
	// If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.
	// It defaults to 30 times the number of MaxScale replicas.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServerMaxConnections int32 `json:"serverMaxConnections,omitempty" webhook:"inmutableinit"`
	// MonitorUsername is the user used by MaxScale monitor to connect to MariaDB server. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	MonitorUsername string `json:"monitorUsername,omitempty" webhook:"inmutableinit"`
	// MonitorPasswordSecretKeyRef is Secret key reference to the password used by MaxScale monitor to connect to MariaDB server. It is defaulted if not provided.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	MonitorPasswordSecretKeyRef GeneratedSecretKeyRef `json:"monitorPasswordSecretKeyRef,omitempty"`
	// MonitorMaxConnections defines the maximum number of connections that the monitor can establish.
	// If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.
	// It defaults to 30 times the number of MaxScale replicas.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	MonitorMaxConnections int32 `json:"monitorMaxConnections,omitempty" webhook:"inmutableinit"`
	// MonitoSyncUsernamerUsername is the user used by MaxScale config sync to connect to MariaDB server. It is defaulted when HA is enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SyncUsername *string `json:"syncUsername,omitempty" webhook:"inmutableinit"`
	// SyncPasswordSecretKeyRef is Secret key reference to the password used by MaxScale config to connect to MariaDB server. It is defaulted when HA is enabled.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SyncPasswordSecretKeyRef *GeneratedSecretKeyRef `json:"syncPasswordSecretKeyRef,omitempty"`
	// SyncMaxConnections defines the maximum number of connections that the sync can establish.
	// If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.
	// It defaults to 30 times the number of MaxScale replicas.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	SyncMaxConnections *int32 `json:"syncMaxConnections,omitempty" webhook:"inmutableinit"`
}

// SetDefaults sets default values.

// TLS defines the PKI to be used with MaxScale.
type MaxScaleTLS struct {
	// Enabled indicates whether TLS is enabled, determining if certificates should be issued and mounted to the MaxScale instance.
	// It is enabled by default when the referred MariaDB instance (via mariaDbRef) has TLS enabled and enforced.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`
	// AdminCASecretRef is a reference to a Secret containing the admin certificate authority keypair. It is used to establish trust and issue certificates for the MaxScale's administrative REST API and GUI.
	// One of:
	// - Secret containing both the 'ca.crt' and 'ca.key' keys. This allows you to bring your own CA to Kubernetes to issue certificates.
	// - Secret containing only the 'ca.crt' in order to establish trust. In this case, either adminCertSecretRef or adminCertIssuerRef fields must be provided.
	// If not provided, a self-signed CA will be provisioned to issue the server certificate.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	AdminCASecretRef *LocalObjectReference `json:"adminCASecretRef,omitempty"`
	// AdminCertSecretRef is a reference to a TLS Secret used by the MaxScale's administrative REST API and GUI.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	AdminCertSecretRef *LocalObjectReference `json:"adminCertSecretRef,omitempty"`
	// AdminCertIssuerRef is a reference to a cert-manager issuer object used to issue the MaxScale's administrative REST API and GUI certificate. cert-manager must be installed previously in the cluster.
	// It is mutually exclusive with adminCertSecretRef.
	// By default, the Secret field 'ca.crt' provisioned by cert-manager will be added to the trust chain. A custom trust bundle may be specified via adminCASecretRef.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	AdminCertIssuerRef *cmmeta.ObjectReference `json:"adminCertIssuerRef,omitempty"`
	// ListenerCASecretRef is a reference to a Secret containing the listener certificate authority keypair. It is used to establish trust and issue certificates for the MaxScale's listeners.
	// One of:
	// - Secret containing both the 'ca.crt' and 'ca.key' keys. This allows you to bring your own CA to Kubernetes to issue certificates.
	// - Secret containing only the 'ca.crt' in order to establish trust. In this case, either listenerCertSecretRef or listenerCertIssuerRef fields must be provided.
	// If not provided, a self-signed CA will be provisioned to issue the listener certificate.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ListenerCASecretRef *LocalObjectReference `json:"listenerCASecretRef,omitempty"`
	// ListenerCertSecretRef is a reference to a TLS Secret used by the MaxScale's listeners.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ListenerCertSecretRef *LocalObjectReference `json:"listenerCertSecretRef,omitempty"`
	// ListenerCertIssuerRef is a reference to a cert-manager issuer object used to issue the MaxScale's listeners certificate. cert-manager must be installed previously in the cluster.
	// It is mutually exclusive with listenerCertSecretRef.
	// By default, the Secret field 'ca.crt' provisioned by cert-manager will be added to the trust chain. A custom trust bundle may be specified via listenerCASecretRef.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ListenerCertIssuerRef *cmmeta.ObjectReference `json:"listenerCertIssuerRef,omitempty"`
	// ServerCASecretRef is a reference to a Secret containing the MariaDB server CA certificates. It is used to establish trust with MariaDB servers.
	// The Secret should contain a 'ca.crt' key in order to establish trust.
	// If not provided, and the reference to a MariaDB resource is set (mariaDbRef), it will be defaulted to the referred MariaDB CA bundle.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServerCASecretRef *LocalObjectReference `json:"serverCASecretRef,omitempty"`
	// ServerCertSecretRef is a reference to a TLS Secret used by MaxScale to connect to the MariaDB servers.
	// If not provided, and the reference to a MariaDB resource is set (mariaDbRef), it will be defaulted to the referred MariaDB client certificate (clientCertSecretRef).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServerCertSecretRef *LocalObjectReference `json:"serverCertSecretRef,omitempty"`
	// VerifyPeerCertificate specifies whether the peer certificate's signature should be validated against the CA.
	// It is disabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	VerifyPeerCertificate *bool `json:"verifyPeerCertificate,omitempty"`
	// VerifyPeerHost specifies whether the peer certificate's SANs should match the peer host.
	// It is disabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	VerifyPeerHost *bool `json:"verifyPeerHost,omitempty"`
	// ReplicationSSLEnabled specifies whether the replication SSL is enabled. If enabled, the SSL options will be added to the server configuration.
	// It is enabled by default when the referred MariaDB instance (via mariaDbRef) has replication enabled.
	// If the MariaDB servers are manually provided by the user via the 'servers' field, this must be set by the user as well.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	ReplicationSSLEnabled *bool `json:"replicationSSLEnabled,omitempty"`
}

// SetDefaults sets reasonable defaults.

// MaxScaleMetrics defines the metrics for a Maxscale.
type MaxScaleMetrics struct {
	// Enabled is a flag to enable Metrics
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
	// Exporter defines the metrics exporter container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Exporter Exporter `json:"exporter"`
	// ServiceMonitor defines the ServiceMonior object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServiceMonitor ServiceMonitor `json:"serviceMonitor"`
}

// MaxScalePodTemplate defines a template for MaxScale Pods.
type MaxScalePodTemplate struct {
	// PodMetadata defines extra metadata for the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PodMetadata *Metadata `json:"podMetadata,omitempty"`
	// ImagePullSecrets is the list of pull Secrets to be used to pull the image.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ImagePullSecrets []LocalObjectReference `json:"imagePullSecrets,omitempty" webhook:"inmutable"`
	// SecurityContext holds pod-level security attributes and common container settings.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PodSecurityContext *PodSecurityContext `json:"podSecurityContext,omitempty"`
	// ServiceAccountName is the name of the ServiceAccount to be used by the Pods.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServiceAccountName *string `json:"serviceAccountName,omitempty" webhook:"inmutableinit"`
	// Affinity to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Affinity *AffinityConfig `json:"affinity,omitempty"`
	// NodeSelector to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// PriorityClassName to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PriorityClassName *string `json:"priorityClassName,omitempty" webhook:"inmutable"`
	// TopologySpreadConstraints to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	TopologySpreadConstraints []TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// SetDefaults sets reasonable defaults.

// ServiceAccountKey defines the key for the ServiceAccount object.

// MaxScaleSpec defines the desired state of MaxScale.
type MaxScaleSpec struct {
	// ContainerTemplate defines templates to configure Container objects.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ContainerTemplate `json:",inline"`
	// PodTemplate defines templates to configure Pod objects.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MaxScalePodTemplate `json:",inline"`
	// SuspendTemplate defines whether the MaxScale reconciliation loop is enabled. This can be useful for maintenance, as disabling the reconciliation loop prevents the operator from interfering with user operations during maintenance activities.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SuspendTemplate `json:",inline"`
	// MariaDBRef is a reference to the MariaDB that MaxScale points to. It is used to initialize the servers field.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MariaDBRef *MariaDBRef `json:"mariaDbRef,omitempty" webhook:"inmutable"`
	// PrimaryServer specifies the desired primary server. Setting this field triggers a switchover operation in MaxScale to the desired server.
	// This option is only valid when using monitors that support switchover, currently limited to the MariaDB monitor.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PrimaryServer *string `json:"primaryServer,omitempty"`
	// Servers are the MariaDB servers to forward traffic to. It is required if 'spec.mariaDbRef' is not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Servers []MaxScaleServer `json:"servers"`
	// Image name to be used by the MaxScale instances. The supported format is `<image>:<tag>`.
	// Only MaxScale official images are supported.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Image string `json:"image,omitempty"`
	// ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`.
	// +optional
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:imagePullPolicy","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// InheritMetadata defines the metadata to be inherited by children resources.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InheritMetadata *Metadata `json:"inheritMetadata,omitempty"`
	// Services define how the traffic is forwarded to the MariaDB servers. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Services []MaxScaleService `json:"services,omitempty"`
	// Monitor monitors MariaDB server instances. It is required if 'spec.mariaDbRef' is not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Monitor MaxScaleMonitor `json:"monitor,omitempty"`
	// Admin configures the admin REST API and GUI.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Admin MaxScaleAdmin `json:"admin,omitempty"`
	// Config defines the MaxScale configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Config MaxScaleConfig `json:"config,omitempty"`
	// Auth defines the credentials required for MaxScale to connect to MariaDB.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Auth MaxScaleAuth `json:"auth,omitempty"`
	// Metrics configures metrics and how to scrape them.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Metrics *MaxScaleMetrics `json:"metrics,omitempty"`
	// TLS defines the PKI to be used with MaxScale.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	TLS *MaxScaleTLS `json:"tls,omitempty"`
	// Connection provides a template to define the Connection for MaxScale.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Connection *ConnectionTemplate `json:"connection,omitempty"`
	// Replicas indicates the number of desired instances.
	// +kubebuilder:default=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas int32 `json:"replicas,omitempty"`
	// PodDisruptionBudget defines the budget for replica availability.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PodDisruptionBudget *PodDisruptionBudget `json:"podDisruptionBudget,omitempty"`
	// UpdateStrategy defines the update strategy for the StatefulSet object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:updateStrategy","urn:alm:descriptor:com.tectonic.ui:advanced"}
	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
	// KubernetesService defines a template for a Kubernetes Service object to connect to MaxScale.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	KubernetesService *ServiceTemplate `json:"kubernetesService,omitempty"`
	// GuiKubernetesService defines a template for a Kubernetes Service object to connect to MaxScale's GUI.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	GuiKubernetesService *ServiceTemplate `json:"guiKubernetesService,omitempty"`
	// RequeueInterval is used to perform requeue reconciliations. If not defined, it defaults to 10s.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	RequeueInterval *metav1.Duration `json:"requeueInterval,omitempty"`
}

// MaxScaleAPIStatus is the state of the servers in the MaxScale API.
type MaxScaleServerStatus struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// IsMaster indicates whether the current server is in Master state.

// IsReady indicates whether the current server is in ready state.

// InMaintenance indicates whether the current server is in maintenance state.

// MaxScaleResourceStatus indicates whether the resource is in a given state.
type MaxScaleResourceStatus struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type MaxScaleConfigSyncStatus struct {
	MaxScaleVersion int `json:"maxScaleVersion"`
	DatabaseVersion int `json:"databaseVersion"`
}

// MaxScaleTLSStatus aggregates the status of the certificates used by the MaxScale instance.
type MaxScaleTLSStatus struct {
	// CABundle is the status of the Certificate Authority bundle.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	CABundle []CertificateStatus `json:"caBundle,omitempty"`
	// AdminCert is the status of the admin certificate.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	AdminCert *CertificateStatus `json:"adminCert,omitempty"`
	// ListenerCert is the status of the listener certificate.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ListenerCert *CertificateStatus `json:"listenerCert,omitempty"`
	// ServerCert is the status of the MariaDB server certificate.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ServerCert *CertificateStatus `json:"serverCert,omitempty"`
}

// MaxScaleStatus defines the observed state of MaxScale
type MaxScaleStatus struct {
	// Conditions for the MaxScale object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Replicas indicates the number of current instances.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas int32 `json:"replicas,omitempty"`
	// PrimaryServer is the primary server in the MaxScale API.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	PrimaryServer *string `json:"primaryServer,omitempty"`
	// Servers is the state of the servers in the MaxScale API.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Servers []MaxScaleServerStatus `json:"servers,omitempty"`
	// Monitor is the state of the monitor in the MaxScale API.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Monitor *MaxScaleResourceStatus `json:"monitor,omitempty"`
	// Services is the state of the services in the MaxScale API.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Services []MaxScaleResourceStatus `json:"services,omitempty"`
	// Listeners is the state of the listeners in the MaxScale API.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Listeners []MaxScaleResourceStatus `json:"listeners,omitempty"`
	// ConfigSync is the state of config sync.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ConfigSync *MaxScaleConfigSyncStatus `json:"configSync,omitempty"`
	// TLS aggregates the status of the certificates used by the MaxScale instance.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	TLS *MaxScaleTLSStatus `json:"tls,omitempty"`
	// MonitorSpec is a hashed version of spec.monitor to be able to track changes during reconciliation.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	MonitorSpec string `json:"monitorSpec,omitempty"`
	// ServersSpec is a hashed version of spec.servers to be able to track changes during reconciliation.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ServersSpec string `json:"serversSpec,omitempty"`
	// ServicesSpec is a hashed version of spec.services to be able to track changes during reconciliation.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ServicesSpec string `json:"servicesSpec,omitempty"`
}

// SetCondition sets a status condition to MaxScale

// GetPrimaryServer obtains the current primary server.

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=mxs
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Primary",type="string",JSONPath=".status.primaryServer"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{MaxScale,v1alpha1},{User,v1alpha1},{Grant,v1alpha1},{Connection,v1alpha1},{Event,v1},{Service,v1},{Secret,v1},{ServiceAccount,v1},{StatefulSet,v1},{Deployment,v1},{PodDisruptionBudget,v1}}

// MaxScale is the Schema for the maxscales API. It is used to define MaxScale clusters.
type MaxScale struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MaxScaleSpec   `json:"spec,omitempty"`
	Status MaxScaleStatus `json:"status,omitempty"`
}

// SetDefaults sets default values.

// IsBeingDeleted indicates that MaxScale has been marked for deletion

// IsReady indicates whether the Maxscale instance is ready.

// IsHAEnabled indicated whether high availability is enabled.

// IsSuspended whether a MaxScale is suspended.

// AreMetricsEnabled indicates whether the MariaDB instance has metrics enabled

// IsTLSEnabled  indicates whether TLS is enabled

// IsSwitchingPrimary indicates whether a primary swichover operation is in progress.

// ShouldVerifyPeerCertificate indicates whether peer certificate should be verified

// ShouldVerifyPeerHost indicates whether peer host should be verified

// IsReplicationSSLEnabled indicates whether TLS for replication should be enabled

// APIUrl returns the URL of the admin API pointing to the Kubernetes Service.

// PodAPIUrl returns the URL of the admin API pointing to a Pod.

// ServerIDs returns the servers indexed by ID.

// ServerIDs returns the IDs of the servers.

// ServiceIndex returns the services indexed by ID.

// ServiceIDs returns the IDs of the services.

// ServiceForListener finds the service for a given listener

// Listeners returns the listeners

// ListenerIndex returns the listeners indexed by ID.

// ListenerIDs returns the IDs of the listeners.

// DefaultPort returns the default port.

// TLSAdminDNSNames are the Service DNS names used by admin TLS certificates.

// TLSListenerDNSNames are the Service DNS names used by listener TLS certificates.

//+kubebuilder:object:root=true

// MaxScaleList contains a list of MaxScale
type MaxScaleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MaxScale `json:"items"`
}

// ListItems gets a copy of the Items slice.
