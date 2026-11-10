package objectstore

import (
	"testing"

	"github.com/stretchr/testify/require"

	apiv2 "github.com/wandb/operator/api/v2"
)

func TestToSecretData_ManagedSeaweedShape(t *testing.T) {
	// Mirrors the values the managed SeaweedFS path populates; the resulting
	// secret must contain exactly the historical key set (no Path).
	ci := ConnInfo{
		Provider:       apiv2.ObjectStoreProviderS3,
		URL:            "s3://ak:sk@object-store-s3.wandb.svc.cluster.local:8333/bucket?tls=false",
		Endpoint:       "object-store-s3.wandb.svc.cluster.local",
		Port:           "8333",
		AccessKey:      "ak",
		SecretKey:      "sk",
		Region:         "us-east-1",
		Bucket:         "bucket",
		Scheme:         "http",
		TlsEnabled:     false,
		ForcePathStyle: true,
	}

	require.Equal(t, map[string]string{
		"url":            "s3://ak:sk@object-store-s3.wandb.svc.cluster.local:8333/bucket?tls=false",
		"Host":           "object-store-s3.wandb.svc.cluster.local",
		"Port":           "8333",
		"AccessKey":      "ak",
		"SecretKey":      "sk",
		"Region":         "us-east-1",
		"Bucket":         "bucket",
		"Scheme":         "http",
		"TlsEnabled":     "false",
		"Provider":       "s3",
		"ForcePathStyle": "true",
	}, ci.ToSecretData())
}

func TestToSecretData_OmitsEmptyDiscreteKeys(t *testing.T) {
	ci := ConnInfo{
		Provider: apiv2.ObjectStoreProviderS3,
		URL:      "s3://my-bucket",
		Bucket:   "my-bucket",
	}
	data := ci.ToSecretData()
	require.Equal(t, "s3://my-bucket", data["url"])
	require.NotContains(t, data, "Host")
	require.NotContains(t, data, "AccessKey")
	require.NotContains(t, data, "SecretKey")
	require.NotContains(t, data, "Region")
	require.NotContains(t, data, "Path")
	require.NotContains(t, data, "Scheme")
	// TlsEnabled is always written; ForcePathStyle is written for S3.
	require.Equal(t, "false", data["TlsEnabled"])
	require.Equal(t, "false", data["ForcePathStyle"])
}

func TestToSecretData_ForcePathStyleIsS3Only(t *testing.T) {
	for _, provider := range []apiv2.ObjectStoreProvider{apiv2.ObjectStoreProviderGCS, apiv2.ObjectStoreProviderAzure} {
		ci := ConnInfo{Provider: provider, URL: "gs://b", Bucket: "b"}
		require.NotContains(t, ci.ToSecretData(), "ForcePathStyle", "path-style is an S3-only concept")
	}
}

func TestToObjectStoreConnection_RequireAll(t *testing.T) {
	ci := ConnInfo{
		Provider:       apiv2.ObjectStoreProviderS3,
		URL:            "s3://ak:sk@host:8333/bucket?tls=false",
		Endpoint:       "host",
		Port:           "8333",
		AccessKey:      "ak",
		SecretKey:      "sk",
		Region:         "us-east-1",
		Bucket:         "bucket",
		Scheme:         "http",
		ForcePathStyle: true,
	}
	conn := ci.ToObjectStoreConnection("conn-secret", true)

	// Every emitted selector points at conn-secret and is required.
	for _, s := range []apiv2.ValueOrSecret{
		conn.URL, conn.Provider, conn.Endpoint, conn.Port, conn.AccessKey,
		conn.SecretKey, conn.Region, conn.Bucket, conn.TlsEnabled, conn.ForcePathStyle,
	} {
		ref := s.SecretKeyRef()
		require.NotNil(t, ref)
		require.Equal(t, "conn-secret", ref.Name)
		require.NotNil(t, ref.Optional)
		require.False(t, *ref.Optional)
	}
	require.Equal(t, "Host", conn.Endpoint.SecretKeyRef().Key)
	require.Equal(t, "url", conn.URL.SecretKeyRef().Key)
	// Path is not written for the managed shape, so its selector stays empty.
	require.Nil(t, conn.Path.SecretKeyRef())
}

func TestToObjectStoreConnection_ExternalOptionality(t *testing.T) {
	ci := ConnInfo{
		Provider:  apiv2.ObjectStoreProviderS3,
		URL:       "s3://ak:sk@host:9000/bucket",
		Endpoint:  "host",
		Port:      "9000",
		AccessKey: "ak",
		SecretKey: "sk",
		Region:    "us-west-2",
		Bucket:    "bucket",
	}
	conn := ci.ToObjectStoreConnection("conn-secret", false)

	// url/Provider/Bucket are required...
	for _, s := range []apiv2.ValueOrSecret{conn.URL, conn.Provider, conn.Bucket} {
		ref := s.SecretKeyRef()
		require.NotNil(t, ref)
		require.NotNil(t, ref.Optional)
		require.False(t, *ref.Optional)
	}
	// ...everything else is optional.
	for _, s := range []apiv2.ValueOrSecret{conn.Endpoint, conn.Port, conn.AccessKey, conn.SecretKey, conn.Region} {
		ref := s.SecretKeyRef()
		require.NotNil(t, ref)
		require.NotNil(t, ref.Optional)
		require.True(t, *ref.Optional)
	}
}
