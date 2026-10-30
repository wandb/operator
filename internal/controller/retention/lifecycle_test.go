package retention

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func lifecycleFixture(t *testing.T, policy apiv2.OnDeletePolicy, extra ...client.Object) (*apiv2.WeightsAndBiases, client.Client, *corev1.Secret) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))
	w := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "test", UID: "wandb-uid", Finalizers: []string{Finalizer}}, Spec: apiv2.WeightsAndBiasesSpec{RetentionPolicy: apiv2.RetentionPolicy{OnDelete: policy}, MySQL: map[string]apiv2.MySQLSpec{"default": {ExternalMysql: &apiv2.MysqlConnection{}}}}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wandb-mysql-connection", Namespace: w.Namespace, UID: "connection-uid", OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases", Name: w.Name, UID: w.UID}}}, Data: map[string][]byte{"password": []byte("preserve-this")}}
	objects := append([]client.Object{w, secret}, extra...)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithInterceptorFuncs(interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
		if obj.GetUID() == "" {
			obj.SetUID(uuid.NewUUID())
		}
		return c.Create(ctx, obj, opts...)
	}}).Build()
	return w, c, secret
}

func TestMigrationAndDetachAreIdempotent(t *testing.T) {
	ctx := context.Background()
	w, c, secret := lifecycleFixture(t, apiv2.DetachOnDelete)
	s, err := Open(ctx, c, c, w)
	require.NoError(t, err)
	require.Error(t, s.CheckReady(ctx))
	require.NoError(t, s.Reconcile(ctx))
	actual := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	require.Empty(t, actual.OwnerReferences)
	require.Equal(t, secret.Data, actual.Data)
	s, err = Open(ctx, c, c, w)
	require.NoError(t, err)
	require.NoError(t, s.Reconcile(ctx))
	require.NoError(t, s.CheckReady(ctx))
	done, err := s.Finalize(ctx)
	require.NoError(t, err)
	require.True(t, done)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	require.Equal(t, secret.UID, actual.UID)
}

func TestPurgeWaitsForDeletionAndPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	w, c, secret := lifecycleFixture(t, apiv2.PurgeOnDelete)
	secret.Finalizers = []string{"test/wait"}
	require.NoError(t, c.Update(ctx, secret))
	s, err := Open(ctx, c, c, w)
	require.NoError(t, err)
	require.NoError(t, s.Reconcile(ctx))
	done, err := s.Finalize(ctx)
	require.NoError(t, err)
	require.False(t, done)
	s, err = Open(ctx, c, c, w)
	require.NoError(t, err)
	done, err = s.Finalize(ctx)
	require.NoError(t, err)
	require.False(t, done)
	actual := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	require.False(t, actual.DeletionTimestamp.IsZero())
	actual.Finalizers = nil
	require.NoError(t, c.Update(ctx, actual))
	done, err = s.Finalize(ctx)
	require.NoError(t, err)
	require.True(t, done)
	require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(secret), actual)))
}

func TestPurgeNeverDeletesReplacementUID(t *testing.T) {
	ctx := context.Background()
	w, c, secret := lifecycleFixture(t, apiv2.PurgeOnDelete)
	s, err := Open(ctx, c, c, w)
	require.NoError(t, err)
	require.NoError(t, s.Reconcile(ctx))
	s.Ledger.Deleting = true
	require.NoError(t, s.save(ctx))
	actual := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	require.NoError(t, c.Delete(ctx, actual))
	actual.ResourceVersion = ""
	actual.UID = "replacement"
	require.NoError(t, c.Create(ctx, actual))
	done, err := s.Finalize(ctx)
	require.ErrorContains(t, err, "replaced")
	require.False(t, done)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(actual), actual))
	require.True(t, actual.DeletionTimestamp.IsZero())
}

func TestMigrationRetriesEveryObjectAfterRootAlreadyDetached(t *testing.T) {
	ctx := context.Background()
	w, c, secret := lifecycleFixture(t, apiv2.DetachOnDelete)
	failed := false
	flaky := interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
		if obj.GetName() == secret.Name && !failed {
			failed = true
			return fmt.Errorf("injected update failure")
		}
		return c.Update(ctx, obj, opts...)
	}})
	s, err := Open(ctx, flaky, c, w)
	require.NoError(t, err)
	require.ErrorContains(t, s.Reconcile(ctx), "injected")
	s, err = Open(ctx, c, c, w)
	require.NoError(t, err)
	require.NoError(t, s.Reconcile(ctx))
	require.NoError(t, s.CheckReady(ctx))
	actual := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	require.Empty(t, actual.OwnerReferences)
}

func TestScopedClientNeverAddsWandbOwnership(t *testing.T) {
	ctx := context.Background()
	w, c, secret := lifecycleFixture(t, apiv2.DetachOnDelete)
	s, err := Open(ctx, c, c, w)
	require.NoError(t, err)
	require.NoError(t, s.Reconcile(ctx))
	actual := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	desired := secret.DeepCopy()
	desired.Data["password"] = []byte("updated")
	_, err = common.CrudResource(ctx, s, desired, actual)
	require.NoError(t, err)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	require.Empty(t, actual.OwnerReferences)
	require.Equal(t, []byte("updated"), actual.Data["password"])
	require.NoError(t, c.Delete(ctx, actual))
	desired.ResourceVersion = ""
	desired.UID = "new-connection"
	_, err = common.CrudResource(ctx, s, desired, (*corev1.Secret)(nil))
	require.NoError(t, err)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	require.Empty(t, actual.OwnerReferences)
}

func TestRemovedInstanceRemainsInDeletionInventory(t *testing.T) {
	ctx := context.Background()
	w, c, secret := lifecycleFixture(t, apiv2.PurgeOnDelete)
	s, err := Open(ctx, c, c, w)
	require.NoError(t, err)
	require.NoError(t, s.Reconcile(ctx))
	w.Spec.MySQL = nil
	s, err = Open(ctx, c, c, w)
	require.NoError(t, err)
	require.NoError(t, s.Reconcile(ctx))
	done, err := s.Finalize(ctx)
	require.NoError(t, err)
	require.False(t, done)
	require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{})))
}

func TestReadoptionRequiresExplicitUIDAndDeletedPreviousInstallation(t *testing.T) {
	ctx := context.Background()
	w, c, secret := lifecycleFixture(t, apiv2.DetachOnDelete)
	s, err := Open(ctx, c, c, w)
	require.NoError(t, err)
	require.NoError(t, s.Reconcile(ctx))
	newW := w.DeepCopy()
	newW.Name = "new-installation"
	newW.UID = "new-uid"
	newW.ResourceVersion = ""
	require.NoError(t, c.Create(ctx, newW))
	next, err := Open(ctx, c, c, newW)
	require.NoError(t, err)
	require.Error(t, next.Reconcile(ctx))
	newW.Annotations = map[string]string{AdoptFromAnnotation: string(w.UID)}
	next, err = Open(ctx, c, c, newW)
	require.NoError(t, err)
	require.ErrorContains(t, next.Reconcile(ctx), "still exists")
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(w), w))
	w.Finalizers = nil
	require.NoError(t, c.Update(ctx, w))
	require.NoError(t, c.Delete(ctx, w))
	next, err = Open(ctx, c, c, newW)
	require.NoError(t, err)
	require.NoError(t, next.Reconcile(ctx))
	actual := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	require.Equal(t, secret.UID, actual.UID)
	require.Equal(t, string(newW.UID), actual.Labels[UIDLabel])
}

func TestMigrationRejectsIndirectOwnershipPath(t *testing.T) {
	ctx := context.Background()
	w, c, secret := lifecycleFixture(t, apiv2.DetachOnDelete)
	parent := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "app-config", Namespace: w.Namespace, UID: "app-config-uid", OwnerReferences: secret.OwnerReferences}}
	require.NoError(t, c.Create(ctx, parent))
	secret.OwnerReferences = append(secret.OwnerReferences, metav1.OwnerReference{APIVersion: "v1", Kind: "ConfigMap", Name: parent.Name, UID: parent.UID})
	require.NoError(t, c.Update(ctx, secret))
	s, err := Open(ctx, c, c, w)
	require.NoError(t, err)
	require.ErrorContains(t, s.Reconcile(ctx), "ownership path")
}

func TestDependencyValidationMatchesProvisioningFallback(t *testing.T) {
	w := testWandb()
	w.Spec.Kafka.ManagedKafka = nil
	store := w.Spec.ObjectStore["default"]
	store.ManagedObjectStore.RetentionPolicy = &apiv2.RetentionPolicy{OnDelete: apiv2.PurgeOnDelete}
	w.Spec.ObjectStore["default"] = store
	require.ErrorContains(t, ValidatePolicies(w), "objectStore[default]")
	w.Spec.ObjectStore["clickhouse"] = apiv2.ObjectStoreSpec{ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{Name: "ch-storage"}}
	require.NoError(t, ValidatePolicies(w))
	require.Equal(t, []string{"objectstore/clickhouse"}, Dependencies(w)["clickhouse/history"])
	delete(w.Spec.ObjectStore, "clickhouse")
	w.Spec.ClickHouse["history"].ManagedClickHouse.RetentionPolicy = &apiv2.RetentionPolicy{OnDelete: apiv2.PurgeOnDelete}
	require.NoError(t, ValidatePolicies(w))
}

func TestRecordedDependencySurvivesSpecRemoval(t *testing.T) {
	s := &Session{parent: &apiv2.WeightsAndBiases{}, Ledger: Ledger{Resources: map[string]Resource{"consumer": {Component: "clickhouse", Instance: "retired", Policy: apiv2.DetachOnDelete}, "store": {Component: "objectstore", Instance: "default", Policy: apiv2.PurgeOnDelete}}, Dependencies: map[string][]string{"clickhouse/retired": {"objectstore/default"}}}}
	require.ErrorContains(t, s.validateDependencies(), "clickhouse/retired")
}

func TestClaimDiscoveryDoesNotCrossInstanceOrNameBoundary(t *testing.T) {
	w := testWandb()
	s := &Session{parent: w}
	sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "test", UID: "statefulset"}, Spec: appsv1.StatefulSetSpec{VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data", Labels: map[string]string{"instance": "cache"}}}}}}
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(sts)
	require.NoError(t, err)
	obj := resourceObject(Resource{APIVersion: "apps/v1", Kind: "StatefulSet"})
	obj.Object = objMap
	obj.SetKind("StatefulSet")
	owners := map[types.UID]Resource{sts.UID: {Component: "redis", Instance: "cache"}}
	for _, tc := range []struct {
		name   string
		labels map[string]string
		want   bool
	}{{"data-cache-0", map[string]string{"instance": "cache"}, true}, {"data-cache-10", map[string]string{"instance": "cache"}, true}, {"data-cache-extra-0", map[string]string{"instance": "cache"}, false}, {"data-cache-0", map[string]string{"instance": "other"}, false}} {
		t.Run(tc.name+fmt.Sprint(tc.want), func(t *testing.T) {
			claim := resourceObject(Resource{APIVersion: "v1", Kind: "PersistentVolumeClaim", Name: tc.name, Namespace: "test"})
			claim.SetLabels(tc.labels)
			_, found := s.claimParent(claim, []*unstructured.Unstructured{obj}, owners)
			require.Equal(t, tc.want, found)
		})
	}
}
