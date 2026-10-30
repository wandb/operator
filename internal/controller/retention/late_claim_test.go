package retention

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestLateRecreatedClaimRequiresDeleteIntentAndIndependentProvenance(t *testing.T) {
	for _, tc := range []struct {
		name                                          string
		intent, association, template, detach, accept bool
	}{
		{"verified vendor recreation", true, true, true, false, true},
		{"no previous delete intent", false, true, true, false, false},
		{"unassociated replacement", true, false, true, false, false},
		{"wrong template labels", true, true, false, false, false},
		{"retained replacement", true, true, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			policy := apiv2.PurgeOnDelete
			if tc.detach {
				policy = apiv2.DetachOnDelete
			}
			w, c, secret := lifecycleFixture(t, policy)
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: w.Namespace, UID: "workload", OwnerReferences: []metav1.OwnerReference{{APIVersion: "v1", Kind: "Secret", Name: secret.Name, UID: secret.UID}}}, Spec: appsv1.StatefulSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "cache"}}, VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}}}}}
			claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-cache-0", Namespace: w.Namespace, UID: "original", Labels: map[string]string{"app": "cache"}}}
			require.NoError(t, c.Create(ctx, sts))
			require.NoError(t, c.Create(ctx, claim))
			s, err := Open(ctx, c, c, w)
			require.NoError(t, err)
			require.NoError(t, s.Reconcile(ctx))
			key := "PersistentVolumeClaim/" + w.Namespace + "/" + claim.Name
			r := s.Ledger.Resources[key]
			r.DeleteRequested = tc.intent
			s.Ledger.Resources[key] = r
			s.Ledger.Deleting = true
			require.NoError(t, s.save(ctx))
			require.NoError(t, c.Delete(ctx, claim))
			require.NoError(t, c.Delete(ctx, sts))
			claim.ResourceVersion, claim.UID = "", "replacement"
			if tc.association {
				associate(claim, w, r)
			}
			if !tc.template {
				claim.Labels["app"] = "another-workload"
			}
			require.NoError(t, c.Create(ctx, claim))
			require.NoError(t, s.discover(ctx))
			if tc.accept {
				require.Equal(t, claim.UID, s.Ledger.Resources[key].UID)
				require.False(t, s.Ledger.Resources[key].DeleteRequested)
			} else {
				require.Equal(t, r.UID, s.Ledger.Resources[key].UID)
			}
			var done bool
			for i := 0; i < 5; i++ {
				done, err = s.Finalize(ctx)
				if done || err != nil {
					break
				}
			}
			if tc.accept {
				require.NoError(t, err)
				require.True(t, done)
				require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(claim), claim)))
			} else {
				require.ErrorContains(t, err, "replaced resource")
				require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(claim), claim))
				require.True(t, claim.DeletionTimestamp.IsZero())
			}
		})
	}
}
