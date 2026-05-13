package objectstoreproxy

import "testing"

func TestFields(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		endpoint string
		port     string
		want     map[string]string
	}{
		{
			name:     "managed minio service uses http",
			rawURL:   "s3://admin:password@minio.default.svc.cluster.local:80/bucket",
			endpoint: "minio.default.svc.cluster.local",
			port:     "80",
			want: map[string]string{
				SchemeKey:     "http",
				UpstreamKey:   "minio.default.svc.cluster.local:80",
				HostHeaderKey: "minio.default.svc.cluster.local",
				SSLNameKey:    "minio.default.svc.cluster.local",
			},
		},
		{
			name:     "s3 compatible endpoint honors tls false",
			rawURL:   "s3://access:secret@minio.example.com:9000/bucket?tls=false&forcePathStyle=true",
			endpoint: "minio.example.com",
			port:     "9000",
			want: map[string]string{
				SchemeKey:     "http",
				UpstreamKey:   "minio.example.com:9000",
				HostHeaderKey: "minio.example.com:9000",
				SSLNameKey:    "minio.example.com",
			},
		},
		{
			name:     "https endpoint without port uses 443 upstream",
			rawURL:   "s3://access:secret@s3.us-west-2.amazonaws.com/bucket?tls=true",
			endpoint: "s3.us-west-2.amazonaws.com",
			want: map[string]string{
				SchemeKey:     "https",
				UpstreamKey:   "s3.us-west-2.amazonaws.com:443",
				HostHeaderKey: "s3.us-west-2.amazonaws.com",
				SSLNameKey:    "s3.us-west-2.amazonaws.com",
			},
		},
		{
			name:     "endpoint URL carries scheme host and port",
			endpoint: "http://minio.local:9000/bucket",
			want: map[string]string{
				SchemeKey:     "http",
				UpstreamKey:   "minio.local:9000",
				HostHeaderKey: "minio.local:9000",
				SSLNameKey:    "minio.local",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Fields(tt.rawURL, tt.endpoint, tt.port)
			for key, want := range tt.want {
				if got[key] != want {
					t.Fatalf("Fields()[%s] = %q, want %q; all fields: %#v", key, got[key], want, got)
				}
			}
		})
	}
}
