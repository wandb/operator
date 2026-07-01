package bufstream

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/external/objectstore"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	return scheme
}

func testWandb() *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Spec: apiv2.WeightsAndBiasesSpec{
			Kafka: apiv2.KafkaSpec{
				ManagedKafka: &apiv2.ManagedKafkaSpec{
					Name:        "wandb-kafka",
					Namespace:   "default",
					Replicas:    2,
					StorageSize: "20Gi",
				},
			},
		},
	}
}

func TestToEtcdApplication(t *testing.T) {
	wandb := testWandb()
	nsn := CreateNsNameBuilder(types.NamespacedName{Namespace: "default", Name: "wandb-kafka"})

	app, err := ToEtcdApplication(wandb, nsn, testScheme(t), manifest.Manifest{})
	require.NoError(t, err)

	require.Equal(t, "wandb-kafka-etcd", app.Name)
	require.Equal(t, "StatefulSet", app.Spec.Kind)
	require.Len(t, app.Spec.VolumeClaimTemplates, 1)
	require.Equal(t, EtcdDataVolumeName, app.Spec.VolumeClaimTemplates[0].Name)
	require.Equal(t, "20Gi", app.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String())
	require.Len(t, app.Spec.PodTemplate.Spec.Containers, 1)
	require.Equal(t, defaultEtcdImage, app.Spec.PodTemplate.Spec.Containers[0].Image)
	require.NotNil(t, app.Spec.ServiceTemplate)
}

func TestToEtcdApplicationHA(t *testing.T) {
	wandb := testWandb()
	nsn := CreateNsNameBuilder(types.NamespacedName{Namespace: "default", Name: "wandb-kafka"})

	app, err := ToEtcdApplication(wandb, nsn, testScheme(t), manifest.Manifest{})
	require.NoError(t, err)

	// Odd-sized HA cluster fronted by a headless service.
	require.NotNil(t, app.Spec.Replicas)
	require.Equal(t, int32(EtcdReplicas), *app.Spec.Replicas)
	require.Equal(t, "wandb-kafka-etcd", app.Spec.ServiceName)
	require.Equal(t, corev1.ClusterIPNone, app.Spec.ServiceTemplate.ClusterIP)
	require.True(t, app.Spec.ServiceTemplate.PublishNotReadyAddresses)

	// Each member derives its identity from the downward API and the static
	// membership list contains every peer.
	env := map[string]corev1.EnvVar{}
	for _, e := range app.Spec.PodTemplate.Spec.Containers[0].Env {
		env[e.Name] = e
	}
	require.NotNil(t, env["POD_NAME"].ValueFrom)
	require.Equal(t, "metadata.name", env["POD_NAME"].ValueFrom.FieldRef.FieldPath)
	require.Equal(t, "$(POD_NAME)", env["ETCD_NAME"].Value)

	initialCluster := env["ETCD_INITIAL_CLUSTER"].Value
	for i := 0; i < EtcdReplicas; i++ {
		member := nsn.EtcdPodName(i)
		require.Contains(t, initialCluster, member+"=http://"+nsn.EtcdPodFQDN(i))
	}

	// Anti-affinity spreads members across nodes by default.
	require.NotNil(t, app.Spec.PodTemplate.Spec.Affinity)
	require.NotNil(t, app.Spec.PodTemplate.Spec.Affinity.PodAntiAffinity)

	require.NotNil(t, app.Spec.PodTemplate.Spec.Containers[0].ReadinessProbe)
}

func testStorage() objectstore.ConnInfo {
	return objectstore.ConnInfo{
		Provider:       apiv2.ObjectStoreProviderS3,
		URI:            "s3://bucket",
		Bucket:         "bucket",
		Endpoint:       "http://seaweedfs:80",
		Region:         "us-east-1",
		AccessKey:      "ak",
		SecretKey:      "sk",
		ForcePathStyle: true,
	}
}

func TestToBufstreamApplication(t *testing.T) {
	wandb := testWandb()
	nsn := CreateNsNameBuilder(types.NamespacedName{Namespace: "default", Name: "wandb-kafka"})

	app, err := ToBufstreamApplication(wandb, nsn, testStorage(), testScheme(t), manifest.Manifest{})
	require.NoError(t, err)

	require.Equal(t, "wandb-kafka", app.Name)
	require.Equal(t, "Deployment", app.Spec.Kind)
	require.NotNil(t, app.Spec.Replicas)
	require.Len(t, app.Spec.PodTemplate.Spec.InitContainers, 1)
	require.Equal(t, "ensure-bucket", app.Spec.PodTemplate.Spec.InitContainers[0].Name)
	require.Equal(t, int32(2), *app.Spec.Replicas)
	require.Len(t, app.Spec.PodTemplate.Spec.Containers, 1)

	container := app.Spec.PodTemplate.Spec.Containers[0]
	require.Equal(t, defaultBufstreamImage, container.Image)

	envNames := map[string]bool{}
	for _, e := range container.Env {
		envNames[e.Name] = true
		require.NotNil(t, e.ValueFrom, "credentials must come from a secret ref, not inline")
	}
	require.True(t, envNames[EnvStorageAccessKeyID])
	require.True(t, envNames[EnvStorageSecretAccessKey])
}

func TestToBufstreamApplicationDefaultsReplicas(t *testing.T) {
	wandb := testWandb()
	wandb.Spec.Kafka.ManagedKafka.Replicas = 0
	nsn := CreateNsNameBuilder(types.NamespacedName{Namespace: "default", Name: "wandb-kafka"})

	app, err := ToBufstreamApplication(wandb, nsn, testStorage(), testScheme(t), manifest.Manifest{})
	require.NoError(t, err)
	require.Equal(t, int32(BufstreamReplicas), *app.Spec.Replicas)
}

func TestToCredentialsSecret(t *testing.T) {
	wandb := testWandb()
	nsn := CreateNsNameBuilder(types.NamespacedName{Namespace: "default", Name: "wandb-kafka"})
	storage := objectstore.ConnInfo{AccessKey: "ak", SecretKey: "sk"}

	secret, err := ToCredentialsSecret(wandb, nsn, storage, testScheme(t))
	require.NoError(t, err)
	require.Equal(t, "ak", secret.StringData[EnvStorageAccessKeyID])
	require.Equal(t, "sk", secret.StringData[EnvStorageSecretAccessKey])
}
