package v1alpha1

const (
	maxScaleMetricsPasswordSecretFieldPath = ".spec.auth.metricsPasswordSecretKeyRef.name"

	maxscaleTLSAdminCASecretFieldPath      = ".spec.tls.adminCASecretRef"
	maxscaleTLSAdminCertSecretFieldPath    = ".spec.tls.adminCertSecretRef"
	maxscaleTLSListenerCASecretFieldPath   = ".spec.tls.listenerCASecretRef"
	maxscaleTLSListenerCertSecretFieldPath = ".spec.tls.listenerCertSecretRef"
	maxscaleTLSServerCASecretFieldPath     = ".spec.tls.serverCASecretRef"
	maxscaleTLSServerCertSecretFieldPath   = ".spec.tls.serverCertSecretRef"

	maxscaleMariaDbRefNameFieldPath = ".spec.mariaDbRef.name"
)

// nolint:gocyclo
// IndexerFuncForFieldPath returns an indexer function for a given field path.

// IndexMaxScale watches and indexes external resources referred by MaxScale resources.
