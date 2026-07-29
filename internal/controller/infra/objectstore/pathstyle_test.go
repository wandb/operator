package objectstore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequiresPathStyle(t *testing.T) {
	cases := []struct {
		endpoint string
		want     bool
	}{
		{"", false},
		// Any explicit endpoint is path-style, AWS's own included: path-style
		// works against AWS, and VPC endpoints require it.
		{"s3.us-east-1.amazonaws.com", true},
		{"bucket.vpce-0abc.s3.us-west-2.vpce.amazonaws.com", true},
		{"minio.wandb.localhost", true},
		{"minio.wandb.localhost:8080", true},
		{"minio", true},
		{"minio:9000", true},
		{"seaweedfs.seaweedfs.svc.cluster.local:8333", true},
		{"http://minio.local:9000", true},
		{"https://s3.example.com", true},
		// CoreWeave object storage is virtual-hosted.
		{"cwobject.com", false},
		{"accel-object.ord1.coreweave.com", false},
		{"foo.cwlota.com", false},
		{"COREWEAVE.COM", false},
		// Suffix match must anchor on a label boundary.
		{"cwobject.com.evil.example", true},
		{"evil-cwobject.com", true},
	}
	for _, tc := range cases {
		t.Run(tc.endpoint, func(t *testing.T) {
			require.Equal(t, tc.want, RequiresPathStyle(tc.endpoint))
		})
	}
}
