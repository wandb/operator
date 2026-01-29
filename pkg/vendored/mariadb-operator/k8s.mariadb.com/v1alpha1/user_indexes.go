package v1alpha1

const (
	userPasswordSecretFieldPath           = ".spec.passwordSecretKeyRef.name"
	userPasswordHashSecretFieldPath       = ".spec.passwordHashSecretKeyRef.name"
	userPasswordPluginNameSecretFieldPath = ".spec.passwordPlugin.pluginNameSecretKeyRef.name"
	userPasswordPluginArgSecretFieldPath  = ".spec.passwordPlugin.pluginArgSecretKeyRef.name"
)

// IndexerFuncForFieldPath returns an indexer function for a given field path.

// IndexUser watches and indexes external resources referred by User resources.
