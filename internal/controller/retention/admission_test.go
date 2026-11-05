package retention

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
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

func TestDeletionAdmissionRequiresMigrationAfterLifecycleUpgrade(t *testing.T) {
	ctx := context.Background()
	w, c, secret := lifecycleFixture(t, apiv2.DetachOnDelete)
	require.NoError(t, gatewayv1.Install(c.Scheme()))
	s, err := Open(ctx, c, c, w)
	require.NoError(t, err)
	require.NoError(t, s.Reconcile(ctx))

	// Earlier controllers marked this spec ready without inventorying the
	// managed Gateway. Reproduce their serialized fingerprint and ownership.
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(w), w))
	w.Spec.Networking = apiv2.NetworkingSpec{Mode: apiv2.NetworkingModeGatewayAPI, GatewayAPI: &apiv2.GatewayAPIConfig{Gateway: apiv2.GatewayConfig{Managed: true}}}
	require.NoError(t, c.Update(ctx, w))
	gw := &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: w.Name + "-gateway", Namespace: w.Namespace, UID: "legacy-gateway", OwnerReferences: secret.OwnerReferences}}
	require.NoError(t, c.Create(ctx, gw))
	data, err := json.Marshal(struct {
		Spec                   apiv2.WeightsAndBiasesSpec
		Tracking, Installation string
	}{w.Spec, w.Annotations["argocd.argoproj.io/tracking-id"], w.Annotations["argocd.argoproj.io/installation-id"]})
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	s.Ledger.ReadyFingerprint = hex.EncodeToString(sum[:])
	require.NoError(t, s.save(ctx))

	require.ErrorContains(t, ValidateDelete(ctx, c, w), "current configuration")
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(gw), gw))
	require.NotEmpty(t, gw.OwnerReferences, "admission must not perform migration")
	s, err = Open(ctx, c, c, w)
	require.NoError(t, err)
	require.Empty(t, s.Ledger.ReadyFingerprint)
	require.NoError(t, s.Reconcile(ctx))
	require.NoError(t, ValidateDelete(ctx, c, w))
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(gw), gw))
	require.Empty(t, gw.OwnerReferences)
	r, known, err := s.objectResource(gw)
	require.NoError(t, err)
	require.True(t, known)
	require.Equal(t, gw.UID, r.UID)
	require.Equal(t, apiv2.PurgeOnDelete, r.Policy)
}
