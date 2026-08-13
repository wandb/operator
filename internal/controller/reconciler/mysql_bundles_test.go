package reconciler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysqlconnection"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplyMySQLBundlesToWorkloadScopesMountsAndDigest(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	defaultSecret := mysqlBundleTestSecret("default-conn", "default-password")
	analyticsSecret := mysqlBundleTestSecret("analytics-conn", "analytics-password")
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(defaultSecret, analyticsSecret).Build()
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Status: apiv2.WeightsAndBiasesStatus{MySQLStatus: map[string]apiv2.MysqlInfraStatus{
			apiv2.DefaultInstanceName: {Connection: *mysqlconnection.Connection(defaultSecret.Name)},
			"analytics":               {Connection: *mysqlconnection.Connection(analyticsSecret.Name)},
		}},
	}

	volumes, mounts, firstDigest, err := applyMySQLBundlesToWorkload(
		t.Context(),
		client,
		wandb,
		map[string]struct{}{"analytics": {}},
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Len(t, volumes, 1)
	require.Len(t, mounts, 1)
	assert.Equal(t, analyticsSecret.Name, volumes[0].Secret.SecretName)
	assert.Equal(t, mysqlconnection.MountPath("analytics"), mounts[0].MountPath)
	assert.NotEmpty(t, firstDigest)

	itemKeys := make([]string, 0, len(volumes[0].Secret.Items))
	for _, item := range volumes[0].Secret.Items {
		itemKeys = append(itemKeys, item.Key)
	}
	assert.ElementsMatch(t, []string{mysqlconnection.SSLCAKey}, itemKeys)
	assert.NotContains(t, itemKeys, mysqlconnection.PasswordKey)

	updated := &corev1.Secret{}
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{
		Namespace: analyticsSecret.Namespace,
		Name:      analyticsSecret.Name,
	}, updated))
	updated.Data[mysqlconnection.PasswordKey] = []byte("rotated-password")
	require.NoError(t, client.Update(t.Context(), updated))

	_, _, secondDigest, err := applyMySQLBundlesToWorkload(
		t.Context(),
		client,
		wandb,
		map[string]struct{}{"analytics": {}},
		nil,
		nil,
	)
	require.NoError(t, err)
	assert.NotEqual(t, firstDigest, secondDigest)
}

func TestApplyMySQLBundlesToWorkloadIgnoresLegacyConnectionSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	legacy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy", Namespace: "default"},
		Data:       map[string][]byte{mysqlconnection.URLKey: []byte("mysql://legacy")},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(legacy).Build()
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Status: apiv2.WeightsAndBiasesStatus{MySQLStatus: map[string]apiv2.MysqlInfraStatus{
			apiv2.DefaultInstanceName: {Connection: *mysqlconnection.Connection(legacy.Name)},
		}},
	}

	volumes, mounts, digest, err := applyMySQLBundlesToWorkload(
		t.Context(),
		client,
		wandb,
		map[string]struct{}{apiv2.DefaultInstanceName: {}},
		nil,
		nil,
	)
	require.NoError(t, err)
	assert.Empty(t, volumes)
	assert.Empty(t, mounts)
	assert.Empty(t, digest)
}

func mysqlBundleTestSecret(name, password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Annotations: map[string]string{
				mysqlconnection.BundleVersionAnnotation: mysqlconnection.BundleVersion,
			},
		},
		Data: map[string][]byte{
			mysqlconnection.URLKey:      []byte("mysql://wandb@mysql:3306/wandb"),
			mysqlconnection.PasswordKey: []byte(password),
			mysqlconnection.SSLCAKey:    []byte("CA"),
		},
	}
}
