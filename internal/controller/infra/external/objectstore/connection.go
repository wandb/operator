package objectstore

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
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
}

// HasStaticCredentials reports whether explicit keys were provided; when false, credentials come from ambient identity (IAM role / workload identity).
func (c ConnInfo) HasStaticCredentials() bool {
	return c.AccessKey != "" && c.SecretKey != ""
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
