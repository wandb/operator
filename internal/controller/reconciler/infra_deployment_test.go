package reconciler

import (
	"context"
	"encoding/json"
	"testing"

	mocov1 "github.com/cybozu-go/moco/api/v1beta2"
	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity"
	"github.com/wandb/operator/internal/controller/infra/managed/kafka/bufstream"
	"github.com/wandb/operator/internal/controller/infra/managed/mysql/moco"
	"github.com/wandb/operator/internal/controller/infra/managed/objectstore/seaweedfs"
	chkv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse-keeper.altinity.com/v1"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func infraDeploymentFixture(t *testing.T, kind string) (client.Client, common.DeploymentResource) {
	t.Helper()
	redisClient, deployment := redisDeploymentFixture(t, kind, false)
	scheme := redisClient.Scheme()
	for _, add := range []func(*runtime.Scheme) error{mocov1.AddToScheme, chiv1.AddToScheme, chkv1.AddToScheme, seaweedv1.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	base := deployment.GetBaseDeploymentSpec()
	// A second application need not use W&B admission to initialize scheduling.
	base.Tolerations = nil
	base.Redis = nil
	base.MySQL = map[string]apiv2.MySQLSpec{
		"default": {ManagedMysql: &apiv2.ManagedMysqlSpec{Name: "example-mysql", Namespace: "test", Replicas: 1, StorageSize: "1Gi"}},
		"remote":  {ExternalMysql: &apiv2.MysqlConnection{Host: apiv2.LiteralValue("mysql.example.com"), Port: apiv2.LiteralValue("3306"), Database: apiv2.LiteralValue("remote"), Username: apiv2.LiteralValue("user")}},
	}
	base.ObjectStore = map[string]apiv2.ObjectStoreSpec{
		"default": {ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{Name: "example-store", Namespace: "test", Replicas: 1, StorageSize: "10Gi", Config: apiv2.ObjectStoreConfig{AccessKey: "admin"}}},
		"remote":  {ExternalObjectStore: &apiv2.ObjectStoreConnection{Endpoint: apiv2.LiteralValue("storage.example.com"), Bucket: apiv2.LiteralValue("remote"), Region: apiv2.LiteralValue("us-east-1")}},
	}
	base.ClickHouse = map[string]apiv2.ClickHouseSpec{
		"default": {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{Name: "example-clickhouse-chi", Namespace: "test", Replicas: 1, StorageSize: "1Gi", Keeper: apiv2.ClickHouseKeeperSpec{Replicas: 1, StorageSize: "1Gi"}}},
		"remote":  {ExternalClickHouse: &apiv2.ClickHouseConnection{Host: apiv2.LiteralValue("clickhouse.example.com"), HTTPPort: apiv2.LiteralValue("8123"), Database: apiv2.LiteralValue("remote")}},
	}
	base.Kafka.ManagedKafka = &apiv2.ManagedKafkaSpec{Name: "example-kafka", Namespace: "test", Replicas: 2, StorageSize: "1Gi"}
	credentials := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "moco-example-mysql", Namespace: "test"}, Data: map[string][]byte{"WRITABLE_PASSWORD": []byte("test-password")}}
	return fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(deployment).WithObjects(deployment, credentials).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				convertSecretStringData(obj)
				return c.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				convertSecretStringData(obj)
				return c.Update(ctx, obj, opts...)
			},
		}).Build(), deployment
}

// The fake client does not perform the apiserver's Secret conversion. Model it
// here so one provider can consume another provider's generated connection.
func convertSecretStringData(obj client.Object) {
	secret, ok := obj.(*corev1.Secret)
	if !ok || len(secret.StringData) == 0 {
		return
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	for key, value := range secret.StringData {
		secret.Data[key] = []byte(value)
	}
	secret.StringData = nil
}

func assertDeploymentOwner(t *testing.T, deployment common.DeploymentResource, obj client.Object) {
	t.Helper()
	refs := obj.GetOwnerReferences()
	require.Len(t, refs, 1)
	require.Equal(t, deployment.GetUID(), refs[0].UID)
	if _, isWandb := deployment.(*apiv2.WeightsAndBiases); isWandb {
		require.Equal(t, "WeightsAndBiases", refs[0].Kind)
		require.Equal(t, "apps.wandb.com/v2", refs[0].APIVersion)
	} else {
		require.Equal(t, "ExampleDeployment", refs[0].Kind)
		require.Equal(t, "test.example.com/v1", refs[0].APIVersion)
	}
}

func assertNoInfraErrors(t *testing.T, conditions []metav1.Condition) {
	t.Helper()
	for _, condition := range conditions {
		require.NotEqual(t, common.ApiErrorReason, condition.Reason, condition.Message)
		require.NotEqual(t, common.ControllerErrorReason, condition.Reason, condition.Message)
	}
}

func TestInfrastructureDeploymentLifecycle(t *testing.T) {
	for _, kind := range []string{"WeightsAndBiases", "ExampleDeployment"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			c, deployment := infraDeploymentFixture(t, kind)
			recorder := record.NewFakeRecorder(20)
			base := deployment.GetBaseDeploymentSpec()

			mysqlConditions := mysqlWriteState(ctx, c, deployment, manifest.InfraConfig{})
			database := "application_db"
			if kind == "WeightsAndBiases" {
				database = "wandb_local"
			}
			mysqlConditions, mysqlConnections := mysqlReadState(ctx, c, deployment, mysqlConditions, database)
			require.Len(t, mysqlConnections, 2)
			mysqlSecret := &corev1.Secret{}
			require.NotNil(t, mysqlConnections["default"])
			require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "test", Name: mysqlConnections["default"].Database.SecretKeyRef().Name}, mysqlSecret))
			require.Equal(t, database, string(mysqlSecret.Data["Database"]))
			_, err := mysqlInferStatus(ctx, c, recorder, deployment, mysqlConditions, mysqlConnections)
			require.NoError(t, err)

			storeConditions, storeConnections := objectStoreWriteState(ctx, c, deployment, manifest.InfraConfig{})
			require.Len(t, storeConnections, 2)
			storeConditions = objectStoreReadState(ctx, c, deployment, storeConditions)
			_, err = objectStoreInferStatus(ctx, c, recorder, deployment, storeConditions, storeConnections)
			require.NoError(t, err)

			// Kafka must wait for its object store; another application gets the
			// same dependency handling without any W&B status or manifest fields.
			pending := kafkaWriteState(ctx, c, deployment, manifest.KafkaConfig{})
			require.Contains(t, pending, metav1.Condition{Type: bufstream.ObjectStoreReadyType, Status: metav1.ConditionFalse, Reason: common.PendingCreateReason})
			status := deployment.GetBaseDeploymentStatus()
			store := status.ObjectStoreStatus["default"]
			store.Ready = true
			status.ObjectStoreStatus["default"] = store
			kafkaConditions := kafkaWriteState(ctx, c, deployment, manifest.KafkaConfig{})
			kafkaConditions, kafkaConnection := kafkaReadState(ctx, c, deployment, kafkaConditions)
			require.NotNil(t, kafkaConnection)
			_, err = kafkaInferStatus(ctx, c, recorder, deployment, kafkaConditions, kafkaConnection)
			require.NoError(t, err)

			clickhouseConditions := clickHouseWriteState(ctx, c, deployment, manifest.InfraConfig{}, manifest.InfraConfig{})
			chi := &chiv1.ClickHouseInstallation{}
			require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "test", Name: "example-clickhouse-chi"}, chi))
			// Simulate the Altinity operator publishing its connection endpoint.
			chi.Status = &chiv1.Status{Endpoint: "clickhouse.test.svc"}
			require.NoError(t, c.Update(ctx, chi))
			clickhouseConditions, clickhouseConnections := clickHouseReadState(ctx, c, deployment, clickhouseConditions)
			require.Len(t, clickhouseConnections, 2)
			require.NotNil(t, clickhouseConnections["default"])
			_, err = clickHouseInferStatus(ctx, c, recorder, deployment, clickhouseConditions, clickhouseConnections)
			require.NoError(t, err)
			for _, conditions := range []map[string][]metav1.Condition{mysqlConditions, storeConditions, clickhouseConditions} {
				for _, instance := range conditions {
					assertNoInfraErrors(t, instance)
				}
			}
			assertNoInfraErrors(t, kafkaConditions)

			for name, obj := range map[string]client.Object{
				"example-mysql": &mocov1.MySQLCluster{}, "example-store": &seaweedv1.Seaweed{},
				"example-clickhouse-chi": &chiv1.ClickHouseInstallation{}, "example-clickhouse-chk": &chkv1.ClickHouseKeeperInstallation{},
				"example-kafka": &apiv2.Application{}, "example-kafka-etcd": &apiv2.Application{},
				// Assert historical names literally, independent of provider constants.
				"wandb-mysql-connection-remote": &corev1.Secret{}, "wandb-objectstore-connection-remote": &corev1.Secret{}, "wandb-clickhouse-connection-remote": &corev1.Secret{},
			} {
				require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "test", Name: name}, obj))
				assertDeploymentOwner(t, deployment, obj)
			}
			actual := deployment.DeepCopyObject().(common.DeploymentResource)
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(deployment), actual))
			require.Equal(t, deployment.GetBaseDeploymentStatus(), actual.GetBaseDeploymentStatus())
			require.True(t, actual.GetBaseDeploymentStatus().MySQLStatus["remote"].Ready)
			require.True(t, actual.GetBaseDeploymentStatus().ObjectStoreStatus["remote"].Ready)
			require.True(t, actual.GetBaseDeploymentStatus().ClickHouseStatus["remote"].Ready)
			require.False(t, mysqlAllReady(actual))
			require.False(t, clickHouseAllReady(actual))
			switch actual := actual.(type) {
			case *apiv2.WeightsAndBiases:
				require.Equal(t, "keep.example.com", actual.Status.Wandb.Hostname)
			case *exampleDeployment:
				require.Equal(t, "keep", actual.Status.ApplicationState)
			}

			require.NoError(t, runMysqlRetentionFinalizer(ctx, c, deployment, "default", base.MySQL["default"]))
			require.NoError(t, runObjectStoreRetentionFinalizer(ctx, c, deployment, "default", base.ObjectStore["default"]))
			require.NoError(t, runClickHouseRetentionFinalizer(ctx, c, deployment, "default", base.ClickHouse["default"]))
			require.NoError(t, runRetentionFinalizer(ctx, c, deployment, base.Kafka.ManagedKafka.ManagedInfraSpec, kafkaPurgeFinalizer, kafkaDetachFinalizer))
			for name, obj := range map[string]client.Object{
				"example-mysql": &mocov1.MySQLCluster{}, "example-store": &seaweedv1.Seaweed{},
				"example-clickhouse-chi": &chiv1.ClickHouseInstallation{}, "example-kafka": &apiv2.Application{},
			} {
				require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "test", Name: name}, obj))
				require.Empty(t, obj.GetOwnerReferences(), name)
			}
		})
	}
}

func TestInfrastructureDeploymentPurge(t *testing.T) {
	ctx := context.Background()
	c, deployment := infraDeploymentFixture(t, "ExampleDeployment")
	base := deployment.GetBaseDeploymentSpec()
	base.RetentionPolicy.OnDelete = apiv2.PurgeOnDelete
	for _, component := range []string{moco.MysqlModuleName, seaweedfs.ObjectStoreModuleName, altinity.ClickhouseModuleName, bufstream.KafkaModuleName} {
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: component, Namespace: "test", Labels: common.BuildWandbLabels(deployment, component)}}
		require.NoError(t, c.Create(ctx, pvc))
	}
	unrelated := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: "test"}}
	require.NoError(t, c.Create(ctx, unrelated))
	require.NoError(t, runMysqlRetentionFinalizer(ctx, c, deployment, "default", base.MySQL["default"]))
	require.NoError(t, runObjectStoreRetentionFinalizer(ctx, c, deployment, "default", base.ObjectStore["default"]))
	require.NoError(t, runClickHouseRetentionFinalizer(ctx, c, deployment, "default", base.ClickHouse["default"]))
	require.NoError(t, runRetentionFinalizer(ctx, c, deployment, base.Kafka.ManagedKafka.ManagedInfraSpec, kafkaPurgeFinalizer, kafkaDetachFinalizer))
	pvcs := &corev1.PersistentVolumeClaimList{}
	require.NoError(t, c.List(ctx, pvcs, client.InNamespace("test")))
	require.Len(t, pvcs.Items, 1)
	require.Equal(t, "unrelated", pvcs.Items[0].Name)
}

func TestLegacyMinioCleanupRemainsWandbSpecific(t *testing.T) {
	for _, policy := range []apiv2.OnDeletePolicy{apiv2.DetachOnDelete, apiv2.PurgeOnDelete} {
		t.Run(string(policy), func(t *testing.T) {
			ctx := context.Background()
			c, deployment := infraDeploymentFixture(t, "WeightsAndBiases")
			wandb := deployment.(*apiv2.WeightsAndBiases)
			wandb.Spec.RetentionPolicy.OnDelete = policy
			tenant := &unstructured.Unstructured{}
			tenant.SetGroupVersionKind(schema.GroupVersionKind{Group: "minio.min.io", Version: "v2", Kind: "Tenant"})
			tenant.SetName("example-minio")
			tenant.SetNamespace("test")
			tenant.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases", Name: wandb.Name, UID: wandb.UID}})
			require.NoError(t, c.Create(ctx, tenant))
			// Generic provisioning does not touch a legacy MinIO tenant, even if
			// its caller happens to be W&B. Cleanup is an explicit W&B step.
			objectStoreWriteState(ctx, c, deployment, manifest.InfraConfig{})
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(tenant), tenant))
			require.Len(t, tenant.GetOwnerReferences(), 1)
			cleanupWandbLegacyMinio(ctx, c, wandb)
			err := c.Get(ctx, client.ObjectKeyFromObject(tenant), tenant)
			if policy == apiv2.PurgeOnDelete {
				require.True(t, apierrors.IsNotFound(err))
			} else {
				require.NoError(t, err)
				require.Empty(t, tenant.GetOwnerReferences())
			}
		})
	}
}

func TestInfrastructureObjectStoreSelection(t *testing.T) {
	ctx := context.Background()
	c, deployment := infraDeploymentFixture(t, "ExampleDeployment")
	base := deployment.GetBaseDeploymentSpec()
	for _, name := range []string{"bufstream", "clickhouse"} {
		base.ObjectStore[name] = apiv2.ObjectStoreSpec{ExternalObjectStore: &apiv2.ObjectStoreConnection{
			Endpoint: apiv2.LiteralValue("storage.example.com"), Bucket: apiv2.LiteralValue(name + "-bucket"), Region: apiv2.LiteralValue("us-east-1"),
		}}
	}
	require.NoError(t, c.Update(ctx, deployment))
	conditions, connections := objectStoreWriteState(ctx, c, deployment, manifest.InfraConfig{})
	conditions = objectStoreReadState(ctx, c, deployment, conditions)
	_, err := objectStoreInferStatus(ctx, c, record.NewFakeRecorder(10), deployment, conditions, connections)
	require.NoError(t, err)
	require.False(t, deployment.GetBaseDeploymentStatus().ObjectStoreStatus["default"].Ready)
	assertNoInfraErrors(t, kafkaWriteState(ctx, c, deployment, manifest.KafkaConfig{}))
	config := &corev1.ConfigMap{}
	nsn := bufstream.CreateNsNameBuilder(client.ObjectKey{Namespace: "test", Name: "example-kafka"})
	require.NoError(t, c.Get(ctx, nsn.ConfigMapNsName(), config))
	require.Contains(t, config.Data[bufstream.ConfigFileName], "bufstream-bucket")
	for _, conditions := range clickHouseWriteState(ctx, c, deployment, manifest.InfraConfig{}, manifest.InfraConfig{}) {
		assertNoInfraErrors(t, conditions)
	}
	chi := &chiv1.ClickHouseInstallation{}
	require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "test", Name: "example-clickhouse-chi"}, chi))
	encoded, err := json.Marshal(chi.Spec.Configuration.Settings)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "clickhouse-bucket")
}
