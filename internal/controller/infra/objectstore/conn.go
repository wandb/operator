// Package objectstore holds the unified object-store connection model shared by
// the managed (SeaweedFS) and external reconcile paths: a single ConnInfo struct
// plus mapper functions that convert between it, the connection Secret's data,
// and the apiv2.ObjectStoreConnection selectors.
package objectstore

import (
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
)

// DefaultRegion is the region assumed when a connection carries none; S3 SDKs and
// S3-compatible backends (SeaweedFS, MinIO) require some region to be set.
const DefaultRegion = "us-east-1"

// ConnInfo is the resolved object-store connection: the read-side counterpart to
// the connection secret, and the value both write paths populate before encoding
// it back out via ToSecretData / ToObjectStoreConnection.
type ConnInfo struct {
	Provider apiv2.ObjectStoreProvider
	// URI is the provider-native location, e.g. "s3://bucket", "gs://bucket/prefix", or "https://acct.blob.core.windows.net/container".
	URI string
	// URL is the canonical connection URL persisted under the secret's "url" key.
	// It is built per-flow (managed appends "?tls=<bool>"; external is provider-specific), so ToSecretData serializes it verbatim.
	URL string
	// Bucket is the bare bucket/container name.
	Bucket string
	// Endpoint is the S3 API endpoint for S3-compatible providers (SeaweedFS, MinIO); empty for AWS S3, GCS, and Azure. It maps to the secret's "Host" key.
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
	// Scheme is the object-store URL scheme ("http"/"https"); only set by the managed path, persisted under the secret's "Scheme" key.
	Scheme string
	// ForcePathStyle is required by most non-AWS S3-compatible providers.
	ForcePathStyle bool
	TlsEnabled     bool
	Port           string
	Path           string
	// Credential selectors, kept for consumers that inject creds by reference.
	AccessKeyRef corev1.SecretKeySelector
	SecretKeyRef corev1.SecretKeySelector
}

// HasStaticCredentials reports whether explicit keys were provided; when false, credentials come from ambient identity (IAM role / workload identity).
func (c ConnInfo) HasStaticCredentials() bool {
	return c.AccessKey != "" && c.SecretKey != ""
}

// ProviderURI builds the provider-native base URI for the connection's bucket:
// "s3://<bucket>", "gs://<bucket>", or the Azure blob container URL. It returns ""
// for an unknown provider. Any object prefix is the caller's to append.
func (c ConnInfo) ProviderURI() string {
	switch c.Provider {
	case apiv2.ObjectStoreProviderS3:
		return "s3://" + c.Bucket
	case apiv2.ObjectStoreProviderGCS:
		return "gs://" + c.Bucket
	case apiv2.ObjectStoreProviderAzure:
		// Azure carries the storage account in AccessKey.
		return AzureBlobURI(c.AccessKey, c.Bucket, "")
	default:
		return ""
	}
}
