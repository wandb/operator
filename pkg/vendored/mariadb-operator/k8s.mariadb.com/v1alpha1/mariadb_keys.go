package v1alpha1

// RootPasswordSecretKeyRef defines the key selector for the root password Secret.

// PasswordSecretKeyRef defines the key selector for the initial user password Secret.

// ReplPasswordSecretKeyRef defines the key selector for for the password to be used by the replication "repl" user

// DefaultConfigMapKeyRef defines the key selector for the default my.cnf ConfigMap.

// MyCnfConfigMapKeyRef defines the key selector for the my.cnf ConfigMap.

// TLSCABundleSecretKeyRef defines the key selector for the TLS Secret trust bundle

// TLSConfigMapKeyRef defines the key selector for the TLS ConfigMap

// TLSServerCASecretKey defines the key for the TLS server CA.

// TLSServerCertSecretKey defines the key for the TLS server cert.

// TLSClientCASecretKey defines the key for the TLS client CA.

// TLSClientCertSecretKey defines the key for the TLS client cert.

// RestoreKey defines the key for the Restore resource used to bootstrap.

// InternalServiceKey defines the key for the internal headless Service

// InternalServiceName defines the name for the internal headless Service

// PrimaryServiceKey defines the key for the primary Service

// PrimaryConnectioneKey defines the key for the primary Connection

// SecondaryServiceKey defines the key for the secondary Service

// SecondaryConnectioneKey defines the key for the secondary Connection

// MetricsKey defines the key for the metrics related resources

// MaxScaleKey defines the key for the MaxScale resource.

// MetricsPasswordSecretKeyRef defines the key selector for for the password to be used by the metrics user

// MetricsConfigSecretKeyRef defines the key selector for the metrics Secret configuration

// InitKey defines the keys for the init objects.

// PhysicalBackupInitJobKey defines the keys for the PhysicalBackup init Job objects.

// PhysicalBackupStagingPVCKey defines the key for the PhysicalBackup staging PVC object.

// PhysicalBackupScaleOutKey defines the key for the PhysicalBackup scale out object.

// PhysicalBackupScaleOutKey defines the key for the PhysicalBackup replica recovery object.

// RecoveryJobKey defines the key for a Galera recovery Job

// PVCKey defines the PVC keys.

// MariadbSysUserKey defines the key for the 'mariadb.sys' User resource.

// MariadbSysGrantKey defines the key for the 'mariadb.sys' Grant resource.

// MariadbDatabaseKey defines the key for the initial database

// MariadbUserKey defines the key for the initial user

// MariadbGrantKey defines the key for the initial grant

// AgentAuthSecretKeyRef defines the Secret key selector for the agent password
