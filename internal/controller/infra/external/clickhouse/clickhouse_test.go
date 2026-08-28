package clickhouse

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const sourceSecretName = "external-clickhouse"

func sourceSel(key string) apiv2.ValueOrSecret {
	return apiv2.ValueFromSecret(sourceSecretName, key, false)
}

func externalTestWandb(conn *apiv2.ClickHouseConnection) *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Spec: apiv2.WeightsAndBiasesSpec{
			ClickHouse: map[string]apiv2.ClickHouseSpec{
				apiv2.DefaultInstanceName: {ExternalClickHouse: conn},
			},
		},
	}
}

// externalTestClient seeds the user's source Secret with the given keys, plus any
// extra objects.
func externalTestClient(t *testing.T, wandb *apiv2.WeightsAndBiases, sourceData map[string][]byte, extra ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	source := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: sourceSecretName, Namespace: "default"},
		Data:       sourceData,
	}
	objects := append([]client.Object{source, wandb}, extra...)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

// withConnectionSecret seeds an already-written connection Secret in Data form,
// the way a GET returns it: the apiserver folds StringData into Data on write,
// and never returns StringData.
func withConnectionSecret(data map[string][]byte) client.Object {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: ConnectionSecretName, Namespace: "default"},
		Data:       data,
	}
}

func connectionSecret(t *testing.T, c client.Client) *corev1.Secret {
	t.Helper()
	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Namespace: "default", Name: ConnectionSecretName}, secret))
	return secret
}

func fullyDeclaredConnection() *apiv2.ClickHouseConnection {
	return &apiv2.ClickHouseConnection{
		Host:        sourceSel("Host"),
		HTTPPort:    sourceSel("HTTPPort"),
		TCPPort:     sourceSel("TCPPort"),
		Username:    sourceSel("User"),
		Password:    sourceSel("Password"),
		Database:    sourceSel("Database"),
		Replicated:  sourceSel("Replicated"),
		ClusterName: sourceSel("ClusterName"),
	}
}

func baseSourceData() map[string][]byte {
	return map[string][]byte{
		"Host":     []byte("clickhouse.example.com"),
		"HTTPPort": []byte("8123"),
		"TCPPort":  []byte("9000"),
		"User":     []byte("wandb"),
		"Password": []byte("secret"),
		"Database": []byte("wandb"),
	}
}

// Replication is connection info, so a declared topology has to land in the
// connection Secret next to host and database — that Secret is the only thing
// applications read.
func TestWriteStateCopiesDeclaredTopologyIntoConnectionSecret(t *testing.T) {
	wandb := externalTestWandb(fullyDeclaredConnection())
	data := baseSourceData()
	data["Replicated"] = []byte("true")
	data["ClusterName"] = []byte("weavecluster")
	c := externalTestClient(t, wandb, data)

	WriteState(context.Background(), c, wandb, apiv2.DefaultInstanceName, fullyDeclaredConnection())

	secret := connectionSecret(t, c)
	require.Equal(t, "true", secret.StringData["Replicated"])
	require.Equal(t, "weavecluster", secret.StringData["ClusterName"])
}

// An external ClickHouse that declares no topology must keep working: the
// selectors are unset, so the keys are simply absent.
func TestWriteStateOmitsUndeclaredTopology(t *testing.T) {
	declared := fullyDeclaredConnection()
	declared.Replicated = apiv2.ValueOrSecret{}
	declared.ClusterName = apiv2.ValueOrSecret{}

	wandb := externalTestWandb(declared)
	c := externalTestClient(t, wandb, baseSourceData())

	WriteState(context.Background(), c, wandb, apiv2.DefaultInstanceName, declared)

	secret := connectionSecret(t, c)
	require.NotContains(t, secret.StringData, "Replicated")
	require.NotContains(t, secret.StringData, "ClusterName")
	require.Equal(t, "clickhouse.example.com", secret.StringData["Host"],
		"the rest of the connection must still be written")
}

// A declared "false" is not the same as an absent value: applications need to be
// able to be told explicitly not to replicate.
func TestWriteStateCopiesExplicitlyFalseReplicated(t *testing.T) {
	wandb := externalTestWandb(fullyDeclaredConnection())
	data := baseSourceData()
	data["Replicated"] = []byte("false")
	data["ClusterName"] = []byte("")
	c := externalTestClient(t, wandb, data)

	WriteState(context.Background(), c, wandb, apiv2.DefaultInstanceName, fullyDeclaredConnection())

	secret := connectionSecret(t, c)
	require.Equal(t, "false", secret.StringData["Replicated"])
}

func TestReadStatePublishesTopologySelectorsWhenPresent(t *testing.T) {
	wandb := externalTestWandb(fullyDeclaredConnection())
	data := baseSourceData()
	data["Replicated"] = []byte("true")
	data["ClusterName"] = []byte("weavecluster")
	c := externalTestClient(t, wandb, data, withConnectionSecret(data))

	_, conn := ReadState(context.Background(), c, wandb, apiv2.DefaultInstanceName, nil)

	require.NotNil(t, conn)
	require.Equal(t, ConnectionSecretName, conn.Replicated.SecretKeyRef().Name)
	require.Equal(t, "Replicated", conn.Replicated.SecretKeyRef().Key)
	require.Equal(t, ConnectionSecretName, conn.ClusterName.SecretKeyRef().Name)
	require.Equal(t, "ClusterName", conn.ClusterName.SecretKeyRef().Key)
}

// Pointing a container at a key that isn't in the Secret keeps it from starting,
// so an absent topology must leave the selector unset.
func TestReadStateLeavesTopologySelectorsUnsetWhenAbsent(t *testing.T) {
	declared := fullyDeclaredConnection()
	declared.Replicated = apiv2.ValueOrSecret{}
	declared.ClusterName = apiv2.ValueOrSecret{}

	wandb := externalTestWandb(declared)
	c := externalTestClient(t, wandb, baseSourceData(), withConnectionSecret(baseSourceData()))

	_, conn := ReadState(context.Background(), c, wandb, apiv2.DefaultInstanceName, nil)

	require.NotNil(t, conn)
	require.Nil(t, conn.Replicated.SecretKeyRef())
	require.Nil(t, conn.ClusterName.SecretKeyRef())
	require.Equal(t, "Host", conn.Host.SecretKeyRef().Key, "the rest of the connection must still resolve")
}
