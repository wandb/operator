package objectstore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemeForTLS(t *testing.T) {
	require.Equal(t, "https", SchemeForTLS(true))
	require.Equal(t, "http", SchemeForTLS(false))
}

func TestSplitScheme(t *testing.T) {
	cases := []struct {
		in         string
		wantScheme string
		wantRest   string
	}{
		{"https://minio.example.com:9000", "https", "minio.example.com:9000"},
		{"http://host", "http", "host"},
		{"host:9000", "", "host:9000"},
		{"host", "", "host"},
		{"", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			scheme, rest := SplitScheme(tc.in)
			require.Equal(t, tc.wantScheme, scheme)
			require.Equal(t, tc.wantRest, rest)
		})
	}
}

func TestEndpointURL(t *testing.T) {
	cases := []struct {
		name string
		ci   ConnInfo
		want string
	}{
		{"no endpoint (AWS S3)", ConnInfo{}, ""},
		{"host with port, tls off", ConnInfo{Endpoint: "minio.example.com", Port: "9000"}, "http://minio.example.com:9000"},
		{"host with port, tls on", ConnInfo{Endpoint: "minio.example.com", Port: "9000", TlsEnabled: true}, "https://minio.example.com:9000"},
		{"host without port", ConnInfo{Endpoint: "minio.example.com", TlsEnabled: true}, "https://minio.example.com"},
		{"scheme preserved over tls", ConnInfo{Endpoint: "http://seaweedfs.svc", Port: "80", TlsEnabled: true}, "http://seaweedfs.svc:80"},
		{"scheme and port already in endpoint", ConnInfo{Endpoint: "http://objstore.svc:8333"}, "http://objstore.svc:8333"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.ci.EndpointURL())
		})
	}
}

func TestAzureBlobURI(t *testing.T) {
	require.Equal(t,
		"https://acct.blob.core.windows.net/container",
		AzureBlobURI("acct", "container", ""))
	require.Equal(t,
		"https://acct.blob.core.windows.net/container/some/prefix",
		AzureBlobURI("acct", "container", "some/prefix"))
}
