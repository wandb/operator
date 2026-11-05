package retention

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestArgoAssociationPreservesParentTrackingIdentity(t *testing.T) {
	for _, policy := range []apiv2.OnDeletePolicy{apiv2.DetachOnDelete, apiv2.PurgeOnDelete} {
		t.Run(string(policy), func(t *testing.T) {
			w := testWandb()
			w.Annotations = map[string]string{trackingIDAnnotation: "production:apps.wandb.com/WeightsAndBiases:test/wandb", installationIDAnnotation: "argocd-east", "argocd.argoproj.io/hook": "PreSync", syncOptionsAnnotation: "Force=true"}
			obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generated", Annotations: map[string]string{syncOptionsAnnotation: "ServerSideApply=true, Prune=true,Delete=true", "customer": "keep"}}}
			associate(obj, w, Resource{Component: "mysql", Instance: "default", Policy: policy})
			require.Equal(t, w.Annotations[trackingIDAnnotation], obj.Annotations[trackingIDAnnotation])
			require.Equal(t, "argocd-east", obj.Annotations[installationIDAnnotation])
			require.Equal(t, "ServerSideApply=true,Prune=false,Delete=false", obj.Annotations[syncOptionsAnnotation])
			require.Equal(t, "keep", obj.Annotations["customer"])
			require.NotContains(t, obj.Annotations, "argocd.argoproj.io/hook")
			require.True(t, argoProtected(obj, w))
			require.Empty(t, obj.OwnerReferences)
			before := obj.DeepCopy()
			protectArgo(obj, w)
			require.Equal(t, before, obj)
		})
	}
}

func TestArgoMetadataChangesRemoveStaleAssociation(t *testing.T) {
	w := testWandb()
	w.Annotations = map[string]string{trackingIDAnnotation: "app:apps.wandb.com/WeightsAndBiases:test/wandb", installationIDAnnotation: "old"}
	obj := &corev1.ConfigMap{}
	protectArgo(obj, w)
	delete(w.Annotations, installationIDAnnotation)
	require.False(t, argoProtected(obj, w))
	protectArgo(obj, w)
	require.NotContains(t, obj.Annotations, installationIDAnnotation)
	w.Annotations = nil
	protectArgo(obj, w)
	require.NotContains(t, obj.Annotations, trackingIDAnnotation)
	require.Equal(t, "Prune=false,Delete=false", obj.Annotations[syncOptionsAnnotation])
}

func TestLabelOnlyTrackingIsNotPropagated(t *testing.T) {
	w := testWandb()
	w.Labels = map[string]string{"app.kubernetes.io/instance": "argocd-app", "custom.tracking/instance": "other-app"}
	obj := &corev1.Secret{}
	associate(obj, w, Resource{Component: "mysql", Instance: "default"})
	require.NotContains(t, obj.Labels, "app.kubernetes.io/instance")
	require.NotContains(t, obj.Labels, "custom.tracking/instance")
	require.Equal(t, "Prune=false,Delete=false", obj.Annotations[syncOptionsAnnotation])
}
