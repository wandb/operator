package objectstore

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConnInfo is the resolved object-store connection: the read-side counterpart to WriteState, decoded back from the connection secret.
type ConnInfo struct {
	Provider apiv2.ObjectStoreProvider
	// URI is the provider-native location, e.g. "s3://bucket", "gs://bucket/prefix", or "https://acct.blob.core.windows.net/container".
	URI string
	// Bucket is the bare bucket/container name.
	Bucket string
	// Endpoint overrides the S3 API endpoint for S3-compatible providers (SeaweedFS, MinIO); empty for AWS S3, GCS, and Azure.
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
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

// Resolve reads the connection's secret selectors into a shared ConnInfo.
func Resolve(
	ctx context.Context,
	cl client.Client,
	namespace string,
	conn *apiv2.ObjectStoreConnection,
) (ConnInfo, error) {
	if conn == nil {
		return ConnInfo{}, fmt.Errorf("object store connection is not available yet")
	}

	resolver := &utils.ConnSecretResolver{Client: cl, Namespace: namespace, Cache: map[string]*corev1.Secret{}}

	info := ConnInfo{
		AccessKeyRef: conn.AccessKey,
		SecretKeyRef: conn.SecretKey,
	}

	provider, err := resolver.Value(ctx, conn.Provider)
	if err != nil {
		return ConnInfo{}, err
	}
	info.Provider = apiv2.ObjectStoreProvider(provider)

	if info.Bucket, err = resolver.Value(ctx, conn.Bucket); err != nil {
		return ConnInfo{}, err
	}
	if info.Endpoint, err = resolver.Value(ctx, conn.Endpoint); err != nil {
		return ConnInfo{}, err
	}
	if info.Port, err = resolver.Value(ctx, conn.Port); err != nil {
		return ConnInfo{}, err
	}
	if info.Region, err = resolver.Value(ctx, conn.Region); err != nil {
		return ConnInfo{}, err
	}
	if info.AccessKey, err = resolver.Value(ctx, conn.AccessKey); err != nil {
		return ConnInfo{}, err
	}
	if info.SecretKey, err = resolver.Value(ctx, conn.SecretKey); err != nil {
		return ConnInfo{}, err
	}
	// A half-configured pair silently picks the wrong credential mode downstream.
	if (info.AccessKey == "") != (info.SecretKey == "") {
		return ConnInfo{}, fmt.Errorf("object store access key and secret key must be configured together")
	}
	if info.Path, err = resolver.Value(ctx, conn.Path); err != nil {
		return ConnInfo{}, err
	}

	forcePathStyleString, err := resolver.Value(ctx, conn.ForcePathStyle)
	if err != nil {
		return ConnInfo{}, err
	}
	if fps, parseErr := strconv.ParseBool(forcePathStyleString); parseErr == nil {
		info.ForcePathStyle = fps
	} else {
		// Connection secrets written before the operator derived this key lack it.
		info.ForcePathStyle = RequiresPathStyle(info.Endpoint)
	}

	tlsEnabledString, err := resolver.Value(ctx, conn.TlsEnabled)
	if err != nil {
		return ConnInfo{}, err
	}
	if tls, parseErr := strconv.ParseBool(tlsEnabledString); parseErr == nil {
		info.TlsEnabled = tls
	}

	return info, nil
}

// ParseConnection decodes an object-store connection secret's canonical `url` (scheme->provider, userinfo->creds, path->bucket, query->tls/region/forcePathStyle), falling back to discrete keys.
func ParseConnection(data map[string][]byte) (ConnInfo, error) {
	get := func(k string) string { return string(data[k]) }

	raw := get("url")
	if raw == "" {
		return ConnInfo{}, fmt.Errorf("object store connection secret missing url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ConnInfo{}, fmt.Errorf("parse object store url %q: %w", raw, err)
	}

	info := ConnInfo{}
	if u.User != nil {
		info.AccessKey = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			info.SecretKey = pw
		}
	}
	if info.AccessKey == "" {
		info.AccessKey = get("AccessKey")
	}
	if info.SecretKey == "" {
		info.SecretKey = get("SecretKey")
	}

	q := u.Query()
	info.Region = q.Get("region")
	if info.Region == "" {
		info.Region = get("Region")
	}

	switch strings.ToLower(u.Scheme) {
	case "s3", "cw":
		info.Provider = apiv2.ObjectStoreProviderS3
		bucket := strings.TrimPrefix(u.Path, "/")
		host := u.Host
		if bucket == "" {
			// No path: bucket is the host (s3://my-bucket) or opaque part (s3:my-bucket), with no endpoint override.
			if u.Opaque != "" {
				bucket = u.Opaque
			} else {
				bucket = host
			}
			host = ""
		}
		info.Bucket = bucket
		info.URI = "s3://" + bucket
		// A host alongside a bucket path means an S3-compatible endpoint (SeaweedFS, MinIO); AWS S3 has no endpoint override.
		if host != "" {
			endpointScheme := "http"
			if tls, _ := strconv.ParseBool(q.Get("tls")); tls {
				endpointScheme = "https"
			}
			info.Endpoint = fmt.Sprintf("%s://%s", endpointScheme, host)
		}
		if fps := q.Get("forcePathStyle"); fps != "" {
			info.ForcePathStyle, _ = strconv.ParseBool(fps)
		} else {
			// Non-AWS S3-compatible endpoints generally require path-style.
			info.ForcePathStyle = info.Endpoint != ""
		}
	case "gs", "gcs":
		info.Provider = apiv2.ObjectStoreProviderGCS
		info.Bucket = u.Host
		info.URI = "gs://" + u.Host + u.Path
	case "azure", "az":
		// az://<account>/<container>/<prefix>
		info.Provider = apiv2.ObjectStoreProviderAzure
		account := u.Host
		container, prefix := splitBucketPath(u.Path)
		info.Bucket = container
		info.URI = azureBlobURI(account, container, prefix)
	case "http", "https":
		if !strings.Contains(u.Host, "blob.core.windows.net") {
			return ConnInfo{}, fmt.Errorf("unsupported object store url scheme %q", u.Scheme)
		}
		info.Provider = apiv2.ObjectStoreProviderAzure
		container, _ := splitBucketPath(u.Path)
		info.Bucket = container
		// Pass the container URI through verbatim (sans credentials/query).
		info.URI = (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
	default:
		return ConnInfo{}, fmt.Errorf("unsupported object store url scheme %q", u.Scheme)
	}

	if info.Bucket == "" && info.Provider != "" {
		return ConnInfo{}, fmt.Errorf("object store url %q has no bucket/container", raw)
	}
	return info, nil
}

func azureBlobURI(account, container, prefix string) string {
	uri := fmt.Sprintf("https://%s.blob.core.windows.net/%s", account, container)
	if prefix != "" {
		uri += "/" + prefix
	}
	return uri
}
