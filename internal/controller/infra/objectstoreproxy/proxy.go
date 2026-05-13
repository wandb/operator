package objectstoreproxy

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

const (
	SchemeKey     = "ProxyScheme"
	UpstreamKey   = "ProxyUpstream"
	HostHeaderKey = "ProxyHostHeader"
	SSLNameKey    = "ProxySSLName"
)

func Enrich(data map[string]string) {
	for k, v := range Fields(data["url"], data["Host"], data["Port"]) {
		data[k] = v
	}
}

func Fields(rawURL, endpoint, port string) map[string]string {
	endpointScheme, host, endpointPort := parseEndpoint(endpoint)
	if port == "" {
		port = endpointPort
	}

	parsedURL, _ := url.Parse(rawURL)
	if host == "" && parsedURL != nil {
		host = parsedURL.Hostname()
	}
	if port == "" && parsedURL != nil {
		port = parsedURL.Port()
	}

	scheme := inferScheme(parsedURL, endpointScheme, port)
	upstreamPort := port
	if upstreamPort == "" {
		upstreamPort = defaultPort(scheme)
	}

	fields := map[string]string{
		SchemeKey: scheme,
	}
	if host == "" {
		return fields
	}

	fields[UpstreamKey] = joinHostPort(host, upstreamPort)
	fields[HostHeaderKey] = host
	if upstreamPort != "" && upstreamPort != defaultPort(scheme) {
		fields[HostHeaderKey] = joinHostPort(host, upstreamPort)
	}
	fields[SSLNameKey] = host
	return fields
}

func parseEndpoint(endpoint string) (scheme, host, port string) {
	if endpoint == "" {
		return "", "", ""
	}

	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err == nil {
			return parsed.Scheme, parsed.Hostname(), parsed.Port()
		}
	}

	if h, p, err := net.SplitHostPort(endpoint); err == nil {
		return "", h, p
	}

	if strings.Count(endpoint, ":") == 1 {
		hostPart, portPart, _ := strings.Cut(endpoint, ":")
		if _, err := strconv.ParseUint(portPart, 10, 16); err == nil {
			return "", hostPart, portPart
		}
	}

	return "", endpoint, ""
}

func inferScheme(parsedURL *url.URL, endpointScheme, port string) string {
	if endpointScheme == "http" || endpointScheme == "https" {
		return endpointScheme
	}
	if parsedURL != nil {
		if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
			return parsedURL.Scheme
		}
		if tlsValue := parsedURL.Query().Get("tls"); tlsValue != "" {
			if enabled, err := strconv.ParseBool(tlsValue); err == nil && !enabled {
				return "http"
			}
			return "https"
		}
	}
	switch port {
	case "80", "9000":
		return "http"
	case "443":
		return "https"
	default:
		return "https"
	}
}

func defaultPort(scheme string) string {
	if scheme == "http" {
		return "80"
	}
	return "443"
}

func joinHostPort(host, port string) string {
	if port == "" {
		return host
	}
	return net.JoinHostPort(host, port)
}
