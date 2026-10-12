package reconciler

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	externalredis "github.com/wandb/operator/internal/controller/infra/external/redis"
	"github.com/wandb/operator/internal/controller/infra/managed/redis/opstree"
	redisv1 "github.com/wandb/operator/pkg/vendored/redis-operator/redis/v1beta2"
	replicationv1 "github.com/wandb/operator/pkg/vendored/redis-operator/redisreplication/v1beta2"
	sentinelv1 "github.com/wandb/operator/pkg/vendored/redis-operator/redissentinel/v1beta2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// This test-only CR deliberately has no WeightsAndBiases fields or methods.
// Registering it separately also exercises GVK resolution and status persistence.
type exampleDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              apiv2.BaseDeploymentSpec `json:"spec"`
	Status            exampleDeploymentStatus  `json:"status"`
}

type exampleDeploymentStatus struct {
	apiv2.BaseDeploymentStatus `json:",inline"`
	ApplicationState           string `json:"applicationState"`
}

func (d *exampleDeployment) GetBaseDeploymentSpec() *apiv2.BaseDeploymentSpec { return &d.Spec }
func (d *exampleDeployment) GetBaseDeploymentStatus() *apiv2.BaseDeploymentStatus {
	return &d.Status.BaseDeploymentStatus
}
func (d *exampleDeployment) DeepCopyObject() runtime.Object {
	out := *d
	d.DeepCopyInto(&out.ObjectMeta)
	d.Spec.DeepCopyInto(&out.Spec)
	d.Status.DeepCopyInto(&out.Status.BaseDeploymentStatus)
	return &out
}

var _ common.DeploymentResource = (*exampleDeployment)(nil)

func redisDeploymentFixture(t *testing.T, kind string, sentinel bool) (client.Client, common.DeploymentResource) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, apiv2.AddToScheme, redisv1.AddToScheme, replicationv1.AddToScheme, sentinelv1.AddToScheme,
	} {
		require.NoError(t, add(scheme))
	}
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "test.example.com", Version: "v1", Kind: "ExampleDeployment"}, &exampleDeployment{})

	var deployment common.DeploymentResource
	if kind == "WeightsAndBiases" {
		deployment = &apiv2.WeightsAndBiases{Status: apiv2.WeightsAndBiasesStatus{
			Wandb: apiv2.WandbStatus{BaseWorkloadStatus: apiv2.BaseWorkloadStatus{Hostname: "keep.example.com"}},
		}}
	} else {
		deployment = &exampleDeployment{Status: exampleDeploymentStatus{ApplicationState: "keep"}}
	}
	deployment.SetName("example")
	deployment.SetNamespace("test")
	deployment.SetUID("deployment-uid")
	deployment.SetGeneration(7)
	*deployment.GetBaseDeploymentSpec() = apiv2.BaseDeploymentSpec{
		RetentionPolicy: apiv2.RetentionPolicy{OnDelete: apiv2.DetachOnDelete},
		Global: apiv2.GlobalSpec{
			ImageRegistry:    "mirror.example.com",
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "pull-secret"}},
		},
		Affinity:    &corev1.Affinity{PodAffinity: &corev1.PodAffinity{}},
		Tolerations: &[]corev1.Toleration{{Key: "global", Operator: corev1.TolerationOpExists}},
		Redis: map[string]apiv2.RedisSpec{
			"default": {ManagedRedis: &apiv2.ManagedRedisSpec{
				Name: "example-redis", Namespace: "test", StorageSize: "1Gi",
				Telemetry: apiv2.Telemetry{Enabled: true},
				Sentinel:  apiv2.RedisSentinelSpec{Enabled: sentinel},
			}},
			"external": {ExternalRedis: &apiv2.RedisConnection{
				Host: apiv2.ValueFromSecret("source", "host", false),
				Port: apiv2.ValueFromSecret("source", "port", false),
			}},
		},
	}
	source := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "source", Namespace: "test"},
		Data: map[string][]byte{"host": []byte("redis.example.com"), "port": []byte("6379")}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(deployment).
		WithObjects(deployment, source).Build()
	return c, deployment
}

func TestRedisDeploymentLifecycle(t *testing.T) {
	for _, kind := range []string{"WeightsAndBiases", "ExampleDeployment"} {
		for _, sentinel := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/sentinel=%t", kind, sentinel), func(t *testing.T) {
				ctx := context.Background()
				c, deployment := redisDeploymentFixture(t, kind, sentinel)
				base := deployment.GetBaseDeploymentSpec()
				config := manifest.InfraConfig{Images: map[string]manifest.ImageRef{}}
				for _, component := range []string{"standalone", "sentinel", "replication", "exporter"} {
					config.Images[component] = manifest.ImageRef{Repository: "redis/" + component, Tag: "test"}
				}
				conditions := redisWriteState(ctx, c, deployment, config)
				for _, instanceConditions := range conditions {
					for _, condition := range instanceConditions {
						require.NotEqual(t, common.ControllerErrorReason, condition.Reason)
						require.NotEqual(t, common.ApiErrorReason, condition.Reason)
					}
				}

				var resources []client.Object
				if sentinel {
					s := &sentinelv1.RedisSentinel{}
					r := &replicationv1.RedisReplication{}
					require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "test", Name: "example-redis"}, s))
					require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "test", Name: "example-redis-replica"}, r))
					require.Equal(t, "mirror.example.com/redis/sentinel:test", s.Spec.KubernetesConfig.Image)
					require.Equal(t, "mirror.example.com/redis/replication:test", r.Spec.KubernetesConfig.Image)
					require.Equal(t, "gorilla", s.Spec.RedisSentinelConfig.MasterGroupName)
					require.Equal(t, base.Tolerations, s.Spec.Tolerations)
					require.Equal(t, base.Affinity, r.Spec.Affinity)
					resources = []client.Object{s, r}
				} else {
					r := &redisv1.Redis{}
					require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "test", Name: "example-redis"}, r))
					require.Equal(t, "mirror.example.com/redis/standalone:test", r.Spec.KubernetesConfig.Image)
					require.Equal(t, base.Global.ImagePullSecrets, *r.Spec.KubernetesConfig.ImagePullSecrets)
					require.Equal(t, "mirror.example.com/redis/exporter:test", r.Spec.RedisExporter.Image)
					require.Equal(t, base.Affinity, r.Spec.Affinity)
					require.Equal(t, base.Tolerations, r.Spec.Tolerations)
					require.Equal(t, map[string]string{
						"weightsandbiases.apps.wandb.com/name":      "example",
						"weightsandbiases.apps.wandb.com/namespace": "test",
						"weightsandbiases.apps.wandb.com/component": "redis",
					}, r.Spec.Storage.VolumeClaimTemplate.Labels)
					resources = []client.Object{r}
				}
				for _, resource := range resources {
					owner := metav1.GetControllerOf(resource)
					require.NotNil(t, owner)
					require.Equal(t, kind, owner.Kind)
					require.Equal(t, deployment.GetUID(), owner.UID)
				}

				conditions, connections := redisReadState(ctx, c, deployment, conditions)
				require.Len(t, connections, 2)
				require.Equal(t, "example-redis-connection", connections["default"].URL.SecretKeyRef().Name)
				require.Equal(t, "wandb-redis-connection-external", connections["external"].URL.SecretKeyRef().Name)
				for _, name := range []string{"example-redis-connection", "wandb-redis-connection-external"} {
					secret := &corev1.Secret{}
					require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "test", Name: name}, secret))
					require.Len(t, secret.OwnerReferences, 1)
					require.Equal(t, kind, secret.OwnerReferences[0].Kind)
					require.Equal(t, deployment.GetUID(), secret.OwnerReferences[0].UID)
				}

				_, err := redisInferStatus(ctx, c, record.NewFakeRecorder(10), deployment, conditions, connections)
				require.NoError(t, err)
				actual := deployment.DeepCopyObject().(common.DeploymentResource)
				require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(deployment), actual))
				require.Equal(t, deployment.GetBaseDeploymentStatus(), actual.GetBaseDeploymentStatus())
				require.True(t, actual.GetBaseDeploymentStatus().RedisStatus["external"].Ready)
				require.False(t, actual.GetBaseDeploymentStatus().RedisStatus["default"].Ready, "no Redis pods exist yet")
				for _, condition := range actual.GetBaseDeploymentStatus().RedisStatus["default"].Conditions {
					require.EqualValues(t, 7, condition.ObservedGeneration)
				}
				switch actual := actual.(type) {
				case *apiv2.WeightsAndBiases:
					require.Equal(t, "keep.example.com", actual.Status.Wandb.Hostname)
				case *exampleDeployment:
					require.Equal(t, "keep", actual.Status.ApplicationState)
				}
				// Equal base status must not invoke the Kubernetes client.
				require.NoError(t, updateDeploymentStatusIfChanged(ctx, nil, actual, actual.GetBaseDeploymentStatus().DeepCopy()))

				// Detach uses the concrete owner's UID for every managed resource.
				require.NoError(t, runRedisRetentionFinalizer(ctx, c, deployment, "default", base.Redis["default"]))
				resources = append(resources, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "example-redis-connection", Namespace: "test"}})
				for _, resource := range resources {
					require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(resource), resource))
					require.Empty(t, resource.GetOwnerReferences())
				}
			})
		}
	}
}

func TestRedisDeploymentRetentionOverrides(t *testing.T) {
	ctx := context.Background()
	c, deployment := redisDeploymentFixture(t, "ExampleDeployment", false)
	base := deployment.GetBaseDeploymentSpec()
	base.Redis["default"].ManagedRedis.RetentionPolicy = &apiv2.RetentionPolicy{OnDelete: apiv2.PurgeOnDelete}
	matching := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "owned", Namespace: "test", Labels: opstree.BuildRedisLabels(deployment)}}
	unrelated := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: "test"}}
	require.NoError(t, c.Create(ctx, matching))
	require.NoError(t, c.Create(ctx, unrelated))
	require.NoError(t, runRedisRetentionFinalizer(ctx, c, deployment, "default", base.Redis["default"]))
	require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(matching), &corev1.PersistentVolumeClaim{})))
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(unrelated), &corev1.PersistentVolumeClaim{}))

	// External Redis inherits deployment retention and retains its legacy name.
	require.Empty(t, externalredis.WriteState(ctx, c, deployment, "default", base.Redis["external"].ExternalRedis))
	base.RetentionPolicy.OnDelete = apiv2.PurgeOnDelete
	require.NoError(t, runRedisRetentionFinalizer(ctx, c, deployment, "default", base.Redis["external"]))
	require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKey{Namespace: "test", Name: "wandb-redis-connection"}, &corev1.Secret{})))
}
