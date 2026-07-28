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

func TestS3Compatible(t *testing.T) {
	for _, p := range []string{"", "s3", "cw"} {
		require.True(t, S3Compatible(p), p)
	}
	for _, p := range []string{"gcs", "az", "azure"} {
		require.False(t, S3Compatible(p), p)
	}
}

func TestParseLegacyBucket(t *testing.T) {
	cases := []struct {
		desc                        string
		provider, name, path        string
		endpoint, port, bkt, prefix string
	}{
		{"empty", "", "", "", "", "", "", ""},
		{"bare bucket", "s3", "my-bucket", "", "", "", "my-bucket", ""},
		{"bare bucket with prefix", "s3", "my-bucket", "prefix", "", "", "my-bucket", "prefix"},
		{"embedded host", "", "minio.example.com/wandb", "", "minio.example.com", "", "wandb", ""},
		{"embedded host:port", "", "minio.example.com:9000/wandb", "", "minio.example.com", "9000", "wandb", ""},
		{"embedded short host:port", "", "minio:9000/wandb", "", "minio", "9000", "wandb", ""},
		{"embedded fqdn host:port", "", "minio.minio.svc.cluster.local:9000/bucket", "", "minio.minio.svc.cluster.local", "9000", "bucket", ""},
		{"host:port name, bucket in path", "s3", "minio.minio.svc.cluster.local:9000", "lsahu-minio-bucket", "minio.minio.svc.cluster.local", "9000", "lsahu-minio-bucket", ""},
		{"host:port name, bucket + prefix in path", "s3", "minio:9000", "/bucket/team/project/", "minio", "9000", "bucket", "team/project"},
		{"host:port name, no provider", "", "minio:9000", "bucket", "minio", "9000", "bucket", ""},
		{"aws bucket with path is not endpoint", "s3", "my-aws-bucket", "prefix", "", "", "my-aws-bucket", "prefix"},
		{"host:port name but non-s3 provider", "gcs", "foo:9000", "bucket", "", "", "foo:9000", "bucket"},
		{"ipv6 host:port name", "s3", "[fd00::1]:9000", "bucket", "fd00::1", "9000", "bucket", ""},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			e, p, b, pre := ParseLegacyBucket(tc.provider, tc.name, tc.path)
			require.Equal(t, tc.endpoint, e)
			require.Equal(t, tc.port, p)
			require.Equal(t, tc.bkt, b)
			require.Equal(t, tc.prefix, pre)
		})
	}
}
