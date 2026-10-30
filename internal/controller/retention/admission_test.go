package retention

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeletionAdmissionRequiresFreshMigration(t *testing.T) {
	ctx := context.Background()
	w, c, secret := lifecycleFixture(t, apiv2.DetachOnDelete)
	require.ErrorContains(t, ValidateDelete(ctx, c, w), "inventory is not ready")
	s, err := Open(ctx, c, c, w)
	require.NoError(t, err)
	require.ErrorContains(t, ValidateDelete(ctx, c, w), "migration is not ready")
	require.NoError(t, s.Reconcile(ctx))
	require.NoError(t, ValidateDelete(ctx, c, w))

	actual := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	actual.OwnerReferences = secret.OwnerReferences
	require.NoError(t, c.Update(ctx, actual))
	require.ErrorContains(t, ValidateDelete(ctx, c, w), "has not completed")
	require.NoError(t, s.Reconcile(ctx))

	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(w), w))
	w.Spec.RetentionPolicy.OnDelete = apiv2.PurgeOnDelete
	require.NoError(t, c.Update(ctx, w))
	require.ErrorContains(t, ValidateDelete(ctx, c, w), "current configuration")
	s, err = Open(ctx, c, c, w)
	require.NoError(t, err)
	require.NoError(t, s.Reconcile(ctx))
	require.NoError(t, ValidateDelete(ctx, c, w))

	stale := w.DeepCopy()
	stale.UID = "previous-installation"
	require.ErrorContains(t, ValidateDelete(ctx, c, stale), "changed during")
	w.Finalizers = nil
	require.NoError(t, c.Update(ctx, w))
	require.ErrorContains(t, ValidateDelete(ctx, c, w), "finalizer")
}

func TestDeletionAdmissionRefusesTerminatingAndUnprotectedResources(t *testing.T) {
	ctx := context.Background()
	w, c, secret := lifecycleFixture(t, apiv2.DetachOnDelete)
	s, err := Open(ctx, c, c, w)
	require.NoError(t, err)
	require.NoError(t, s.Reconcile(ctx))
	actual := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	delete(actual.Annotations, "argocd.argoproj.io/sync-options")
	require.NoError(t, c.Update(ctx, actual))
	require.ErrorContains(t, ValidateDelete(ctx, c, w), "Argo CD deletion protection")
	require.NoError(t, s.Reconcile(ctx))
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), actual))
	actual.Finalizers = []string{"test/hold"}
	require.NoError(t, c.Update(ctx, actual))
	require.NoError(t, c.Delete(ctx, actual, client.PropagationPolicy(metav1.DeletePropagationForeground)))
	require.ErrorContains(t, ValidateDelete(ctx, c, w), "has not completed")
}
