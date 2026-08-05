package objectstore

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// SchemeForTLS returns the URL scheme implied by whether TLS is enabled.
func SchemeForTLS(tls bool) string {
	if tls {
		return "https"
	}
	return "http"
}

// SplitScheme separates a "scheme://rest" endpoint into its scheme and remainder.
// When no scheme is present it returns an empty scheme and the input unchanged.
func SplitScheme(endpoint string) (scheme, rest string) {
	if i := strings.Index(endpoint, "://"); i >= 0 {
		return endpoint[:i], endpoint[i+len("://"):]
	}
	return "", endpoint
}

// EndpointURL renders the S3-compatible API endpoint as "scheme://host[:port]",
// or "" when no endpoint override is set (i.e. plain AWS S3). A scheme already
// present in Endpoint is preserved; otherwise it is derived from TlsEnabled. Port
// is appended only when set and the host does not already carry one.
func (c ConnInfo) EndpointURL() string {
	if c.Endpoint == "" {
		return ""
	}
	scheme, host := SplitScheme(c.Endpoint)
	if scheme == "" {
		scheme = SchemeForTLS(c.TlsEnabled)
	}
	if c.Port != "" && !strings.Contains(host, ":") {
		host += ":" + c.Port
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

// AzureBlobURI builds the Azure Blob container URL for a storage account, e.g.
// "https://<account>.blob.core.windows.net/<container>[/<prefix>]".
func AzureBlobURI(account, container, prefix string) string {
	uri := fmt.Sprintf("https://%s.blob.core.windows.net/%s", account, container)
	if prefix != "" {
		uri += "/" + prefix
	}
	return uri
}

// ParseLegacyBucket splits a v1 bucket.name/path into endpoint, port, bucket and prefix.
func ParseLegacyBucket(provider, name, path string) (endpoint, port, bucket, prefix string) {
	prefix = strings.Trim(path, "/")

	// host[:port]/bucket
	if slash := strings.IndexByte(name, '/'); slash >= 0 {
		host, bkt := name[:slash], name[slash+1:]
		if colon := strings.IndexByte(host, ':'); colon >= 0 {
			return host[:colon], host[colon+1:], bkt, prefix
		}
		return host, "", bkt, prefix
	}

	// host:port name, bucket in path
	if host, hostPort := splitHostPort(name); host != "" && prefix != "" && S3Compatible(provider) {
		bkt, rest, _ := strings.Cut(prefix, "/")
		return host, hostPort, bkt, rest
	}

	return "", "", name, prefix
}

// splitHostPort parses "host:port", returning "","" if the port is missing or invalid.
func splitHostPort(name string) (host, port string) {
	host, port, err := net.SplitHostPort(name)
	if err != nil || host == "" {
		return "", ""
	}
	if n, err := strconv.ParseUint(port, 10, 16); err != nil || n == 0 {
		return "", ""
	}
	return host, port
}

// S3Compatible reports whether the provider uses an S3-style endpoint.
func S3Compatible(provider string) bool {
	return provider == "" || provider == "s3" || provider == "cw"
}
