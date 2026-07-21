package objectstore

import (
	"net"
	"strings"
)

// coreweaveDomains identify CoreWeave AI Object Storage endpoints, which are
// virtual-hosted (matching the server's cw:// handling).
var coreweaveDomains = []string{"cwobject.com", "cwlota.com", "coreweave.com"}

// RequiresPathStyle reports whether an S3 endpoint needs path-style addressing:
// true for any custom endpoint except CoreWeave's virtual-hosted object storage.
func RequiresPathStyle(endpoint string) bool {
	host := strings.ToLower(endpoint)
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "" {
		return false
	}
	for _, domain := range coreweaveDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return false
		}
	}
	return true
}
