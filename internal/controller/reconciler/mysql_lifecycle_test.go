package reconciler

import (
	"testing"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/managed/mysql/moco"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileStaleManagedMySQLDetachesRemovedInstance(t *testing.T) {
	scheme := mysqlLifecycleScheme(t)
	wandb := mysqlLifecycleWandb(apiv2.DetachOnDelete)
	labels := moco.BuildWandbMysqlLabels(wandb, "analytics")
	cluster := &mocov1beta2.MySQLCluster{ObjectMeta: metav1.ObjectMeta{
		Name:        "analytics",
		Namespace:   "database",
		Labels:      labels,
		Annotations: map[string]string{moco.RetentionPolicyAnnotation: string(apiv2.DetachOnDelete)},
	}}
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      moco.MyCnfConfigMapName(cluster.Name),
		Namespace: cluster.Namespace,
		Labels:    labels,
	}}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, cluster, configMap).Build()

	require.NoError(t, reconcileStaleManagedMySQL(t.Context(), client, wandb))
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{
		Name: cluster.Name, Namespace: cluster.Namespace,
	}, cluster))
	assert.Equal(t, "true", cluster.Annotations[moco.DetachedAnnotation])
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{
		Name: configMap.Name, Namespace: configMap.Namespace,
	}, configMap))
	assert.Equal(t, "true", configMap.Annotations[moco.DetachedAnnotation])
}

func TestReconcileStaleManagedMySQLPurgesRemovedInstance(t *testing.T) {
	scheme := mysqlLifecycleScheme(t)
	wandb := mysqlLifecycleWandb(apiv2.PurgeOnDelete)
	labels := moco.BuildWandbMysqlLabels(wandb, "analytics")
	cluster := &mocov1beta2.MySQLCluster{ObjectMeta: metav1.ObjectMeta{
		Name:        "analytics",
		Namespace:   "database",
		Labels:      labels,
		Annotations: map[string]string{moco.RetentionPolicyAnnotation: string(apiv2.PurgeOnDelete)},
	}}
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      moco.MyCnfConfigMapName(cluster.Name),
		Namespace: cluster.Namespace,
		Labels:    labels,
	}}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: "mysql-data-moco-analytics-0", Namespace: cluster.Namespace, Labels: labels,
	}}
	credentials := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: "moco-" + cluster.Name, Namespace: cluster.Namespace,
	}}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, cluster, configMap, pvc, credentials).Build()

	require.NoError(t, reconcileStaleManagedMySQL(t.Context(), client, wandb))
	assert.True(t, apierrors.IsNotFound(client.Get(t.Context(), types.NamespacedName{
		Name: cluster.Name, Namespace: cluster.Namespace,
	}, &mocov1beta2.MySQLCluster{})))
	assert.True(t, apierrors.IsNotFound(client.Get(t.Context(), types.NamespacedName{
		Name: configMap.Name, Namespace: configMap.Namespace,
	}, &corev1.ConfigMap{})))
	assert.True(t, apierrors.IsNotFound(client.Get(t.Context(), types.NamespacedName{
		Name: pvc.Name, Namespace: pvc.Namespace,
	}, &corev1.PersistentVolumeClaim{})))
	assert.True(t, apierrors.IsNotFound(client.Get(t.Context(), types.NamespacedName{
		Name: credentials.Name, Namespace: credentials.Namespace,
	}, &corev1.Secret{})))
}

func TestReconcileStaleManagedMySQLPurgeKeepsOtherInstanceStorage(t *testing.T) {
	scheme := mysqlLifecycleScheme(t)
	wandb := mysqlLifecycleWandb(apiv2.PurgeOnDelete)
	wandb.Spec.MySQL["default"] = apiv2.MySQLSpec{ManagedMysql: &apiv2.ManagedMysqlSpec{
		Name: "primary", Namespace: "database",
	}}
	// A cluster from before the per-instance labels existed: only the
	// deployment-wide labels and a legacy owner reference identify it.
	legacyLabels := common.BuildWandbLabels(wandb, moco.MysqlModuleName)
	cluster := &mocov1beta2.MySQLCluster{ObjectMeta: metav1.ObjectMeta{
		Name: "analytics", Namespace: "database", Labels: legacyLabels,
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: apiv2.GroupVersion.String(),
			Kind:       "WeightsAndBiases",
			Name:       wandb.Name,
			UID:        wandb.UID,
		}},
	}}
	stalePVC := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: "mysql-data-moco-analytics-0", Namespace: "database", Labels: legacyLabels,
	}}
	otherInstancePVC := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: "mysql-data-moco-primary-0", Namespace: "database", Labels: legacyLabels,
	}}
	client := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(wandb, cluster, stalePVC, otherInstancePVC).Build()

	require.NoError(t, reconcileStaleManagedMySQL(t.Context(), client, wandb))
	assert.True(t, apierrors.IsNotFound(client.Get(t.Context(), types.NamespacedName{
		Name: stalePVC.Name, Namespace: stalePVC.Namespace,
	}, &corev1.PersistentVolumeClaim{})), "stale instance PVC should be purged")
	assert.NoError(t, client.Get(t.Context(), types.NamespacedName{
		Name: otherInstancePVC.Name, Namespace: otherInstancePVC.Namespace,
	}, &corev1.PersistentVolumeClaim{}), "still-desired instance PVC must survive")
}

func TestReconcileStaleManagedMySQLKeepsRenamedInstanceKey(t *testing.T) {
	scheme := mysqlLifecycleScheme(t)
	wandb := mysqlLifecycleWandb(apiv2.DetachOnDelete)
	wandb.Spec.MySQL["renamed"] = apiv2.MySQLSpec{ManagedMysql: &apiv2.ManagedMysqlSpec{
		Name: "analytics", Namespace: "database",
	}}
	cluster := &mocov1beta2.MySQLCluster{ObjectMeta: metav1.ObjectMeta{
		Name:      "analytics",
		Namespace: "database",
		Labels:    moco.BuildWandbMysqlLabels(wandb, "analytics"),
	}}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, cluster).Build()

	require.NoError(t, reconcileStaleManagedMySQL(t.Context(), client, wandb))
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{
		Name: cluster.Name, Namespace: cluster.Namespace,
	}, cluster))
	assert.Empty(t, cluster.Annotations[moco.DetachedAnnotation])
}

func TestReconcileStaleManagedMySQLReadoptsRestoredInstance(t *testing.T) {
	scheme := mysqlLifecycleScheme(t)
	wandb := mysqlLifecycleWandb(apiv2.DetachOnDelete)
	wandb.Spec.MySQL["analytics"] = apiv2.MySQLSpec{ManagedMysql: &apiv2.ManagedMysqlSpec{
		Name: "analytics", Namespace: "database",
	}}
	labels := moco.BuildWandbMysqlLabels(wandb, "analytics")
	cluster := &mocov1beta2.MySQLCluster{ObjectMeta: metav1.ObjectMeta{
		Name:        "analytics",
		Namespace:   "database",
		Labels:      labels,
		Annotations: map[string]string{moco.DetachedAnnotation: "true"},
	}}
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:        moco.MyCnfConfigMapName(cluster.Name),
		Namespace:   cluster.Namespace,
		Labels:      labels,
		Annotations: map[string]string{moco.DetachedAnnotation: "true"},
	}}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, cluster, configMap).Build()

	require.NoError(t, reconcileStaleManagedMySQL(t.Context(), client, wandb))
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{
		Name: cluster.Name, Namespace: cluster.Namespace,
	}, cluster))
	assert.Empty(t, cluster.Annotations[moco.DetachedAnnotation])
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{
		Name: configMap.Name, Namespace: configMap.Namespace,
	}, configMap))
	assert.Empty(t, configMap.Annotations[moco.DetachedAnnotation])
}

func mysqlLifecycleWandb(policy apiv2.OnDeletePolicy) *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wandb", Namespace: "app", UID: types.UID("wandb-uid"),
		},
		Spec: apiv2.WeightsAndBiasesSpec{
			RetentionPolicy: apiv2.RetentionPolicy{OnDelete: policy},
			MySQL:           map[string]apiv2.MySQLSpec{},
		},
	}
}

func mysqlLifecycleScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, mocov1beta2.AddToScheme(scheme))
	return scheme
}
