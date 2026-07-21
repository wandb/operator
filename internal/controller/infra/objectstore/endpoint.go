package objectstore

import (
	"fmt"
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
