package v1alpha1

const (
	mariadbMyCnfConfigMapFieldPath = ".spec.myCnfConfigMapKeyRef.name"

	mariadbMetricsPasswordSecretFieldPath = ".spec.metrics.passwordSecretKeyRef"

	mariadbTLSServerCASecretFieldPath   = ".spec.tls.serverCASecretRef"
	mariadbTLSServerCertSecretFieldPath = ".spec.tls.serverCertSecretRef"
	mariadbTLSClientCASecretFieldPath   = ".spec.tls.clientCASecretRef"
	mariadbTLSClientCertSecretFieldPath = ".spec.tls.clientCertSecretRef"

	mariadbMaxScaleRefNameFieldPath = ".spec.maxScaleRef.name"
)

// nolint:gocyclo
// IndexerFuncForFieldPath returns an indexer function for a given field path.

// IndexMariaDB watches and indexes external resources referred by MariaDB resources.
