package bufstream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv2 "github.com/wandb/operator/api/v2"
)

// resolveStorageFixture seeds a connection secret with the given keys and a
// wandb whose default object-store status selects every key from it.
func resolveStorageFixture(t *testing.T, data map[string]string) (ctrlclient.Client, *apiv2.WeightsAndBiases, *apiv2.ManagedKafkaSpec) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	secretData := map[string][]byte{}
	for k, v := range data {
		secretData[k] = []byte(v)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-objectstore-connection", Namespace: "wandb"},
		Data:       secretData,
	}

	sel := func(key string) corev1.SecretKeySelector {
		return corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secret.Name},
			Key:                  key,
		}
	}
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb"},
		Status: apiv2.WeightsAndBiasesStatus{
			ObjectStoreStatus: map[string]apiv2.ObjectStoreInfraStatus{
				apiv2.DefaultInstanceName: {
					WBInfraStatus: apiv2.WBInfraStatus{Ready: true},
					Connection: apiv2.ObjectStoreConnection{
						Provider:       sel("Provider"),
						Bucket:         sel("Bucket"),
						Endpoint:       sel("Host"),
						Port:           sel("Port"),
						Region:         sel("Region"),
						AccessKey:      sel("AccessKey"),
						SecretKey:      sel("SecretKey"),
						ForcePathStyle: sel("ForcePathStyle"),
						TlsEnabled:     sel("TlsEnabled"),
					},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	return cl, wandb, &apiv2.ManagedKafkaSpec{Namespace: "wandb"}
}

func TestResolveStorage_ForcePathStyleFallbackOnMissingKey(t *testing.T) {
	cl, wandb, spec := resolveStorageFixture(t, map[string]string{
		"Provider": "s3",
		"Bucket":   "wandb",
		"Host":     "minio.wandb.localhost",
		"Port":     "8080",
	})

	info, ready, err := resolveStorage(context.Background(), cl, wandb, spec)
	require.NoError(t, err)
	require.True(t, ready)
	require.True(t, info.ForcePathStyle, "secrets predating the derived key must fall back to the endpoint rule")
}

func TestResolveStorage_ForcePathStyleExplicitRespected(t *testing.T) {
	cl, wandb, spec := resolveStorageFixture(t, map[string]string{
		"Provider":       "s3",
		"Bucket":         "wandb",
		"Host":           "minio.wandb.localhost",
		"ForcePathStyle": "false",
	})

	info, _, err := resolveStorage(context.Background(), cl, wandb, spec)
	require.NoError(t, err)
	require.False(t, info.ForcePathStyle)
}

func TestResolveStorage_NoEndpointStaysVirtualHosted(t *testing.T) {
	cl, wandb, spec := resolveStorageFixture(t, map[string]string{
		"Provider": "s3",
		"Bucket":   "wandb",
	})

	info, _, err := resolveStorage(context.Background(), cl, wandb, spec)
	require.NoError(t, err)
	require.False(t, info.ForcePathStyle, "native AWS S3 must not force path-style")
}
