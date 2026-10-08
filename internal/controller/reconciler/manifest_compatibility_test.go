package reconciler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/wandb/operator/api/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/observability/telemetry"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func compatibilityFixture(t *testing.T, payload string) (client.WithWatch, *apiv2.WeightsAndBiases, *record.FakeRecorder) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server.yaml"), []byte(payload), 0600))
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, rbacv1.AddToScheme(scheme))
	w := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "test", Generation: 2, Finalizers: []string{CleanupFinalizer}},
		Status:     apiv2.WeightsAndBiasesStatus{Ready: true, ObservedGeneration: 1},
	}
	w.Spec.Wandb.Version = "server"
	w.Spec.Wandb.ManifestRepository = "file://" + dir
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(w).WithObjects(w).Build()
	return c, w, record.NewFakeRecorder(20)
}

func TestManifestCompatibilityBlocksBeforeLegacyWrites(t *testing.T) {
	for _, tc := range []struct{ name, payload, reason string }{
		{"unsupported", "manifestVersion: 2\napplications: [future]\n", "UnsupportedManifestVersion"},
		{"invalid", "manifestVersion: null\n", "InvalidManifestVersion"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, w, events := compatibilityFixture(t, tc.payload)
			// This annotation would otherwise create a Secret before loading.
			w.Annotations = map[string]string{apiv1.MySQLPendingAnnotation: `{"host":"mysql.example.com","user":"wandb","password":"secret"}`}
			require.NoError(t, c.Update(context.Background(), w))
			guard := interceptor.NewClient(c, interceptor.Funcs{
				Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
					t.Fatal("unexpected resource create")
					return nil
				},
				Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
					t.Fatal("unexpected resource update")
					return nil
				},
				Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
					t.Fatal("unexpected resource patch")
					return nil
				},
				Delete: func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error {
					t.Fatal("unexpected resource delete")
					return nil
				},
			})
			res, err := Reconcile(context.Background(), guard, events, w, telemetry.TelemetryRuntimeConfig{})
			require.NoError(t, err)
			require.Equal(t, defaultRequeueDuration, res.RequeueAfter)
			actual := &apiv2.WeightsAndBiases{}
			require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(w), actual))
			require.False(t, actual.Status.Ready)
			require.EqualValues(t, 1, actual.Status.ObservedGeneration)
			condition := apimeta.FindStatusCondition(actual.Status.Conditions, manifestCompatibleCondition)
			require.NotNil(t, condition)
			require.Equal(t, tc.reason, condition.Reason)
			require.EqualValues(t, 2, condition.ObservedGeneration)
			require.Equal(t, metav1.ConditionFalse, condition.Status)
			require.Equal(t, 1, len(events.Events))
			firstTransition := condition.LastTransitionTime
			_, err = Reconcile(context.Background(), guard, events, actual, telemetry.TelemetryRuntimeConfig{})
			require.NoError(t, err)
			require.Equal(t, 1, len(events.Events))
			require.Equal(t, firstTransition, apimeta.FindStatusCondition(actual.Status.Conditions, manifestCompatibleCondition).LastTransitionTime)
			// A compatible second instance uses the same operator independently.
			other := w.DeepCopy()
			other.Name, other.ResourceVersion, other.Status = "other", "", apiv2.WeightsAndBiasesStatus{}
			other.Spec.Wandb.Version = "compatible"
			require.NoError(t, os.WriteFile(filepath.Join(w.Spec.Wandb.ManifestRepository[len("file://"):], "compatible.yaml"), []byte("manifestVersion: 1\n"), 0600))
			require.NoError(t, c.Create(context.Background(), other))
			_, result, err := loadCompatibleManifest(context.Background(), c, events, other)
			require.NoError(t, err)
			require.Zero(t, result.RequeueAfter)
		})
	}
}

func TestManifestCompatibilityRecoveryAndRetrievalFailure(t *testing.T) {
	c, w, events := compatibilityFixture(t, "manifestVersion: 2\n")
	_, _, err := loadCompatibleManifest(context.Background(), c, events, w)
	require.NoError(t, err)
	w.Spec.Wandb.Version = "missing"
	_, _, err = loadCompatibleManifest(context.Background(), c, events, w)
	require.Error(t, err)
	require.Equal(t, metav1.ConditionUnknown, apimeta.FindStatusCondition(w.Status.Conditions, manifestCompatibleCondition).Status)
	w.Spec.Wandb.Version = "server"
	dir := w.Spec.Wandb.ManifestRepository[len("file://"):]
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server.yaml"), []byte("features: {}\n"), 0600))
	m, res, err := loadCompatibleManifest(context.Background(), c, events, w)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.Equal(t, 1, m.ManifestVersion)
	condition := apimeta.FindStatusCondition(w.Status.Conditions, manifestCompatibleCondition)
	require.Equal(t, metav1.ConditionTrue, condition.Status)
	require.Contains(t, condition.Message, "defaulted")
	require.False(t, w.Status.Ready, "compatibility alone must not mark the deployment ready")
	require.Equal(t, "Reconciling", apimeta.FindStatusCondition(w.Status.Conditions, "Ready").Reason)
	require.Equal(t, 3, len(events.Events))
}

func TestUnsupportedManifestDoesNotBlockFinalization(t *testing.T) {
	c, w, events := compatibilityFixture(t, "manifestVersion: 2\n")
	require.NoError(t, c.Delete(context.Background(), w))
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(w), w))
	_, err := Reconcile(context.Background(), c, events, w, telemetry.TelemetryRuntimeConfig{})
	require.NoError(t, err)
	require.Empty(t, events.Events, "finalization must not load or validate the manifest")
	actual := &apiv2.WeightsAndBiases{}
	require.True(t, apierrors.IsNotFound(c.Get(context.Background(), client.ObjectKeyFromObject(w), actual)))
}
