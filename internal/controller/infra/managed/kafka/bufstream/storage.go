package bufstream

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// parseStorageConnection turns an object-store connection secret into the
// resolved values Bufstream needs. It parses the canonical `url` connection
// string the same way the rest of the platform does (scheme selects the
// provider, userinfo carries credentials, the path is the bucket/container, and
// query params carry tls/region/forcePathStyle), falling back to the discrete
// secret keys the operator also writes (Region, AccessKey, SecretKey) when a
// value is absent from the URL.
func parseStorageConnection(data map[string][]byte) (storageConnInfo, error) {
	get := func(k string) string { return string(data[k]) }

	raw := get("url")
	if raw == "" {
		return storageConnInfo{}, fmt.Errorf("object store connection secret missing url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return storageConnInfo{}, fmt.Errorf("parse object store url %q: %w", raw, err)
	}

	info := storageConnInfo{}
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
		info.Provider = providerS3
		bucket := strings.TrimPrefix(u.Path, "/")
		host := u.Host
		if bucket == "" {
			// No path component: the bucket is encoded as the host
			// (s3://my-bucket) or the opaque part (s3:my-bucket), and there is
			// no S3-compatible endpoint override.
			if u.Opaque != "" {
				bucket = u.Opaque
			} else {
				bucket = host
			}
			host = ""
		}
		info.Bucket = bucket
		info.URI = "s3://" + bucket
		// A host alongside a bucket path means an S3-compatible endpoint
		// (SeaweedFS, MinIO, …); AWS S3 has no endpoint override.
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
		info.Provider = providerGCS
		info.Bucket = u.Host
		info.URI = "gs://" + u.Host + u.Path
	case "azure", "az":
		// az://<account>/<container>/<prefix>
		info.Provider = providerAzure
		account := u.Host
		container, prefix := splitContainerPrefix(u.Path)
		info.Bucket = container
		info.URI = azureBlobURI(account, container, prefix)
	case "http", "https":
		if !strings.Contains(u.Host, "blob.core.windows.net") {
			return storageConnInfo{}, fmt.Errorf("unsupported object store url scheme %q", u.Scheme)
		}
		info.Provider = providerAzure
		container, _ := splitContainerPrefix(u.Path)
		info.Bucket = container
		// Pass the container URI through verbatim (sans credentials/query).
		info.URI = (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
	default:
		return storageConnInfo{}, fmt.Errorf("unsupported object store url scheme %q", u.Scheme)
	}

	if info.Bucket == "" && info.Provider != "" {
		return storageConnInfo{}, fmt.Errorf("object store url %q has no bucket/container", raw)
	}
	return info, nil
}

// splitContainerPrefix splits a "/container/prefix..." path into its first
// segment (container) and the remainder (prefix).
func splitContainerPrefix(path string) (container, prefix string) {
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return "", ""
	}
	if slash := strings.IndexByte(trimmed, '/'); slash >= 0 {
		return trimmed[:slash], trimmed[slash+1:]
	}
	return trimmed, ""
}

func azureBlobURI(account, container, prefix string) string {
	uri := fmt.Sprintf("https://%s.blob.core.windows.net/%s", account, container)
	if prefix != "" {
		uri += "/" + prefix
	}
	return uri
}
