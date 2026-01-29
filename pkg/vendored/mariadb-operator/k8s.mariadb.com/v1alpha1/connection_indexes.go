package v1alpha1

const (
	connectionPasswordSecretFieldPath      = ".spec.passwordSecretKeyRef.name"
	connectionTLSClientCertSecretFieldPath = ".spec.tlsClientCertSecretRef.name"
)

// IndexerFuncForFieldPath returns an indexer function for a given field path.

// IndexConnection watches and indexes external resources referred by Connection resources.
