package objectstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv2 "github.com/wandb/operator/api/v2"
)

const sourceSecretName = "ext-objectstore"

func sel(key string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: sourceSecretName},
		Key:                  key,
	}
}

// writeStateFixture builds a fake client seeded with a source Secret holding
// the given keys, plus a wandb whose externalObjectStore selectors point at
// the keys named in present.
func writeStateFixture(t *testing.T, sourceData map[string]string, present map[string]bool) (*apiv2.WeightsAndBiases, *corev1.Secret, []metav1.Condition, *apiv2.ObjectStoreConnection) {
	t.Helper()
	return writeStateFixtureProvider(t, "", sourceData, present)
}

func writeStateFixtureProvider(t *testing.T, provider apiv2.ObjectStoreProvider, sourceData map[string]string, present map[string]bool) (*apiv2.WeightsAndBiases, *corev1.Secret, []metav1.Condition, *apiv2.ObjectStoreConnection) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	data := map[string][]byte{}
	for k, v := range sourceData {
		data[k] = []byte(v)
	}
	source := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: sourceSecretName, Namespace: "default"},
		Data:       data,
	}

	ext := &apiv2.ObjectStoreConnection{Provider: provider}
	if present["Host"] {
		ext.Endpoint = sel("Host")
	}
	if present["Port"] {
		ext.Port = sel("Port")
	}
	if present["AccessKey"] {
		ext.AccessKey = sel("AccessKey")
	}
	if present["SecretKey"] {
		ext.SecretKey = sel("SecretKey")
	}
	if present["Bucket"] {
		ext.Bucket = sel("Bucket")
	}
	if present["Region"] {
		ext.Region = sel("Region")
	}

	wandb := &apiv2.WeightsAndBiases{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Spec: apiv2.WeightsAndBiasesSpec{
			ObjectStore: apiv2.ObjectStoreSpec{ExternalObjectStore: ext},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, source).Build()
	conditions, conn := WriteState(context.Background(), c, wandb)

	written := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: ConnectionSecretName, Namespace: "default"}, written))
	return wandb, written, conditions, conn
}

// connectionData merges Data and StringData since WriteConnectionSecret writes
// via StringData and the fake client does not run the apiserver's
// StringData->Data normalization.
func connectionData(s *corev1.Secret) map[string]string {
	out := map[string]string{}
	for k, v := range s.Data {
		out[k] = string(v)
	}
	for k, v := range s.StringData {
		out[k] = v
	}
	return out
}

func TestWriteState_MinioNoRegion(t *testing.T) {
	_, written, conditions, conn := writeStateFixture(t,
		map[string]string{
			"Host":      "minio.local",
			"Port":      "9000",
			"AccessKey": "minio",
			"SecretKey": "minio123",
			"Bucket":    "my-bucket",
		},
		map[string]bool{"Host": true, "Port": true, "AccessKey": true, "SecretKey": true, "Bucket": true},
	)
	require.Nil(t, conditions, "expected success")
	require.NotNil(t, conn)

	data := connectionData(written)
	require.Equal(t, "s3://minio:minio123@minio.local:9000/my-bucket", data["url"])
	require.NotContains(t, data, "Region")
}

func TestWriteState_AwsIamNoHostNoKeys(t *testing.T) {
	_, written, conditions, conn := writeStateFixture(t,
		map[string]string{
			"Region": "us-east-1",
			"Bucket": "my-bucket",
		},
		map[string]bool{"Region": true, "Bucket": true},
	)
	require.Nil(t, conditions, "region-only config must not fail")
	require.NotNil(t, conn)

	data := connectionData(written)
	require.Equal(t, "s3://my-bucket", data["url"], "no host means no authority, no creds")
	require.Equal(t, "us-east-1", data["Region"])
	require.NotContains(t, data, "Host")
	require.NotContains(t, data, "AccessKey")
	require.NotContains(t, data, "SecretKey")
}

func TestWriteState_HostNoPortNoKeys(t *testing.T) {
	_, written, conditions, _ := writeStateFixture(t,
		map[string]string{
			"Host":   "minio.local",
			"Bucket": "my-bucket",
		},
		map[string]bool{"Host": true, "Bucket": true},
	)
	require.Nil(t, conditions)

	data := connectionData(written)
	require.Equal(t, "s3://minio.local/my-bucket", data["url"], "host without port or creds")
}

func TestWriteState_FullConfig(t *testing.T) {
	_, written, conditions, conn := writeStateFixture(t,
		map[string]string{
			"Host":      "s3.example.com",
			"Port":      "443",
			"AccessKey": "access",
			"SecretKey": "secret",
			"Bucket":    "my-bucket",
			"Region":    "us-west-2",
		},
		map[string]bool{"Host": true, "Port": true, "AccessKey": true, "SecretKey": true, "Bucket": true, "Region": true},
	)
	require.Nil(t, conditions)
	require.NotNil(t, conn)

	data := connectionData(written)
	require.Equal(t, "s3://access:secret@s3.example.com:443/my-bucket", data["url"])
	require.Equal(t, "us-west-2", data["Region"])

	// Optional flags: only url and Bucket are required.
	require.NotNil(t, conn.URL.Optional)
	require.False(t, *conn.URL.Optional)
	require.NotNil(t, conn.Bucket.Optional)
	require.False(t, *conn.Bucket.Optional)
	for _, s := range []corev1.SecretKeySelector{conn.Endpoint, conn.Port, conn.AccessKey, conn.SecretKey, conn.Region} {
		require.NotNil(t, s.Optional)
		require.True(t, *s.Optional)
	}
}

func TestWriteState_GCSWorkloadIdentity(t *testing.T) {
	_, written, conditions, conn := writeStateFixtureProvider(t, apiv2.ObjectStoreProviderGCS,
		map[string]string{"Bucket": "my-gcs-bucket"},
		map[string]bool{"Bucket": true},
	)
	require.Nil(t, conditions)
	require.NotNil(t, conn)
	require.Equal(t, apiv2.ObjectStoreProviderGCS, conn.Provider)

	data := connectionData(written)
	require.Equal(t, "gs://my-gcs-bucket", data["url"], "workload identity carries no credentials")
	require.Equal(t, "gcs", data["Provider"])
}

func TestWriteState_GCSWithPrefixAndKey(t *testing.T) {
	_, written, conditions, _ := writeStateFixtureProvider(t, apiv2.ObjectStoreProviderGCS,
		map[string]string{
			"Bucket":    "my-gcs-bucket/sub/prefix",
			"AccessKey": "sa@project.iam.gserviceaccount.com",
			"SecretKey": "pemkey",
		},
		map[string]bool{"Bucket": true, "AccessKey": true, "SecretKey": true},
	)
	require.Nil(t, conditions)

	data := connectionData(written)
	require.Equal(t, "gs://sa%40project.iam.gserviceaccount.com:pemkey@my-gcs-bucket/sub/prefix", data["url"])
}

func TestWriteState_AzureWithKey(t *testing.T) {
	_, written, conditions, conn := writeStateFixtureProvider(t, apiv2.ObjectStoreProviderAzure,
		map[string]string{
			"AccessKey": "mystorageaccount",
			"SecretKey": "accountkey==",
			"Bucket":    "mycontainer",
		},
		map[string]bool{"AccessKey": true, "SecretKey": true, "Bucket": true},
	)
	require.Nil(t, conditions)
	require.NotNil(t, conn)
	require.Equal(t, apiv2.ObjectStoreProviderAzure, conn.Provider)

	data := connectionData(written)
	require.Equal(t, "az://:accountkey==@mystorageaccount/mycontainer", data["url"])
	require.Equal(t, "azure", data["Provider"])
}

func TestWriteState_AzureAmbientIdentityWithPrefix(t *testing.T) {
	_, written, conditions, _ := writeStateFixtureProvider(t, apiv2.ObjectStoreProviderAzure,
		map[string]string{
			"AccessKey": "mystorageaccount",
			"Bucket":    "mycontainer/team/prefix",
		},
		map[string]bool{"AccessKey": true, "Bucket": true},
	)
	require.Nil(t, conditions)

	data := connectionData(written)
	require.Equal(t, "az://mystorageaccount/mycontainer/team/prefix", data["url"], "no key means ambient identity / AZURE_STORAGE_KEY")
}
