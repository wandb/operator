package retention

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func testWandb() *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "test", UID: "installation-uid"}, Spec: apiv2.WeightsAndBiasesSpec{
		MySQL:       map[string]apiv2.MySQLSpec{"analytics": {ManagedMysql: &apiv2.ManagedMysqlSpec{Name: "db", Namespace: "infra"}}},
		Redis:       map[string]apiv2.RedisSpec{"cache": {ManagedRedis: &apiv2.ManagedRedisSpec{Name: "cache"}}},
		ObjectStore: map[string]apiv2.ObjectStoreSpec{"default": {ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{Name: "wandb-seaweedfs"}}},
		ClickHouse:  map[string]apiv2.ClickHouseSpec{"history": {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{Name: "history-chi"}}},
		Kafka:       apiv2.KafkaSpec{ManagedKafka: &apiv2.ManagedKafkaSpec{Name: "broker"}},
	}}
}

func TestInventoryIncludesSupportingResources(t *testing.T) {
	w := testWandb()
	resources, err := Inventory(w)
	require.NoError(t, err)
	names := map[string][]string{}
	for _, r := range resources {
		names[r.Component] = append(names[r.Component], r.Kind+"/"+r.Namespace+"/"+r.Name)
		require.Equal(t, apiv2.DetachOnDelete, r.Policy)
	}
	require.ElementsMatch(t, []string{"MySQLCluster/infra/db", "ConfigMap/infra/db-mycnf", "Secret/infra/db-connection", "Secret/infra/db-db-password"}, names["mysql"])
	require.ElementsMatch(t, []string{"Redis/test/cache", "RedisSentinel/test/cache", "RedisReplication/test/cache-replica", "Secret/test/cache-connection"}, names["redis"])
	require.ElementsMatch(t, []string{"Seaweed/test/wandb-seaweedfs", "Secret/test/wandb-seaweedfs-s3-config", "Secret/test/wandb-objectstore-connection", "Tenant/test/wandb-minio"}, names["objectstore"])
	require.ElementsMatch(t, []string{"ClickHouseInstallation/test/history-chi", "ClickHouseKeeperInstallation/test/history-chk", "ServiceAccount/test/history-chi", "Secret/test/history-chi-connection"}, names["clickhouse"])
	require.ElementsMatch(t, []string{"Application/test/broker", "Application/test/broker-etcd", "ConfigMap/test/broker-config", "Secret/test/broker-storage-credentials", "Secret/test/broker-connection", "ServiceAccount/test/broker", "RoleBinding/test/broker-scc-nonroot-v2"}, names["kafka"])
}

func TestInventoryExcludesSuppliedIdentityAndInputSecrets(t *testing.T) {
	w := testWandb()
	w.Spec.Kafka.ManagedKafka.ServiceAccount = apiv2.ManagedServiceAccountSpec{Create: ptr.To(false), ServiceAccountName: "customer"}
	w.Spec.MySQL = map[string]apiv2.MySQLSpec{"default": {ExternalMysql: &apiv2.MysqlConnection{Password: apiv2.ValueFromSecret("customer-password", "password", false)}}}
	resources, err := Inventory(w)
	require.NoError(t, err)
	for _, r := range resources {
		require.NotEqual(t, "customer", r.Name)
		require.NotEqual(t, "customer-password", r.Name)
	}
	require.Contains(t, resources, Resource{APIVersion: "v1", Kind: "Secret", Namespace: "test", Name: "wandb-mysql-connection", Component: "mysql", Instance: "default", Policy: apiv2.DetachOnDelete, Stage: 2})
}

func TestPolicyResolution(t *testing.T) {
	w := testWandb()
	w.Spec.RetentionPolicy.OnDelete = apiv2.PurgeOnDelete
	p, err := Policy(w, apiv2.ManagedInfraSpec{})
	require.NoError(t, err)
	require.Equal(t, apiv2.PurgeOnDelete, p)
	p, err = Policy(w, apiv2.ManagedInfraSpec{RetentionPolicy: &apiv2.RetentionPolicy{}})
	require.NoError(t, err)
	require.Equal(t, apiv2.DetachOnDelete, p)
	w.Spec.RetentionPolicy.OnDelete = "typo"
	_, err = Inventory(w)
	require.ErrorContains(t, err, "invalid retention policy")
}

func TestInventoryRejectsNameCollision(t *testing.T) {
	w := testWandb()
	w.Spec.Redis["other"] = w.Spec.Redis["cache"]
	_, err := Inventory(w)
	require.ErrorContains(t, err, "shared")
}

func TestAssociationPreservesInternalOwnershipAndInstanceKey(t *testing.T) {
	w := testWandb()
	r := Resource{Component: "redis", Instance: "arbitrary/key with spaces"}
	obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"customer": "keep"}, OwnerReferences: []metav1.OwnerReference{{UID: w.UID}, {UID: "internal-parent"}}}}
	obj.Labels["weightsandbiases.apps.wandb.com/component"] = "clickhouse-keeper"
	associate(obj, w, r)
	require.False(t, ownedBy(obj, w.UID))
	require.True(t, ownedBy(obj, "internal-parent"))
	require.True(t, associated(obj, w, r))
	require.Len(t, obj.Labels[InstanceLabel], 63)
	require.Equal(t, r.Instance, obj.Annotations[InstanceAnnotation])
	require.Equal(t, "keep", obj.Labels["customer"])
	require.Equal(t, "clickhouse-keeper", obj.Labels["weightsandbiases.apps.wandb.com/component"])
	require.Equal(t, []metav1.OwnerReference{{UID: "internal-parent"}}, obj.OwnerReferences)
	other := r
	other.Instance = "arbitrary_key with spaces"
	require.False(t, associated(obj, w, other))
}

func TestInventoryAlwaysPurgesOnlyManagedGateways(t *testing.T) {
	for _, managed := range []bool{false, true} {
		w := testWandb()
		w.Spec.Networking = apiv2.NetworkingSpec{Mode: apiv2.NetworkingModeGatewayAPI, GatewayAPI: &apiv2.GatewayAPIConfig{Gateway: apiv2.GatewayConfig{Managed: managed}}}
		resources, err := Inventory(w)
		require.NoError(t, err)
		count := 0
		for _, r := range resources {
			if isGateway(r) {
				count++
				require.Equal(t, apiv2.PurgeOnDelete, r.Policy)
				require.Equal(t, w.Name+"-gateway", r.Name)
			}
		}
		if managed {
			require.Equal(t, 1, count)
		} else {
			require.Zero(t, count)
		}
	}
}
