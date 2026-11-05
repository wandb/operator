package bufstream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPurgeFinalizerFindsClaimsFromEtcdTemplate(t *testing.T) {
	wandb := testWandb()
	nsn := types.NamespacedName{Namespace: "default", Name: "wandb-kafka"}
	scheme := testScheme(t)
	require.NoError(t, corev1.AddToScheme(scheme))
	app, err := ToEtcdApplication(wandb, CreateNsNameBuilder(nsn), scheme, manifest.Manifest{})
	require.NoError(t, err)
	claim := app.Spec.VolumeClaimTemplates[0].DeepCopy()
	claim.Name = "data-wandb-kafka-etcd-0"
	claim.Namespace = nsn.Namespace
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim).Build()
	rule := ToKafkaOnDeleteRule(wandb, apiv2.RetentionPolicy{OnDelete: apiv2.PurgeOnDelete})
	require.NoError(t, PurgeFinalizer(context.Background(), cl, nsn, rule))
	err = cl.Get(context.Background(), types.NamespacedName{Namespace: claim.Namespace, Name: claim.Name}, &corev1.PersistentVolumeClaim{})
	require.True(t, apierrors.IsNotFound(err), "template-generated claim was not purged: %v", err)
}

func TestPurgeFinalizerHandlesLegacyClaimsWithoutDeletingOtherData(t *testing.T) {
	for _, tc := range []struct {
		name        string
		policy      common.OnDeletePolicy
		change      func(*corev1.PersistentVolumeClaim)
		wantDeleted bool
	}{
		{name: "legacy", policy: common.Purge, wantDeleted: true},
		{name: "higher ordinal", policy: common.Purge, change: func(p *corev1.PersistentVolumeClaim) { p.Name = "data-wandb-kafka-etcd-12" }, wantDeleted: true},
		{name: "detach", policy: common.Detach},
		{name: "another namespace", policy: common.Purge, change: func(p *corev1.PersistentVolumeClaim) { p.Namespace = "other" }},
		{name: "another workload", policy: common.Purge, change: func(p *corev1.PersistentVolumeClaim) { p.Labels["app.kubernetes.io/name"] = "other-etcd" }},
		{name: "another instance", policy: common.Purge, change: func(p *corev1.PersistentVolumeClaim) { p.Labels["app.kubernetes.io/instance"] = "other" }},
		{name: "missing identity", policy: common.Purge, change: func(p *corev1.PersistentVolumeClaim) { p.Labels = nil }},
		{name: "another CR", policy: common.Purge, change: func(p *corev1.PersistentVolumeClaim) { p.Labels[common.WandbNameLabel] = "other" }},
		{name: "another component", policy: common.Purge, change: func(p *corev1.PersistentVolumeClaim) { p.Labels[common.WandbComponentLabel] = "mysql" }},
		{name: "backup", policy: common.Purge, change: func(p *corev1.PersistentVolumeClaim) { p.Name += "-backup" }},
		{name: "different prefix", policy: common.Purge, change: func(p *corev1.PersistentVolumeClaim) { p.Name = "data-other-etcd-0" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := testScheme(t)
			require.NoError(t, corev1.AddToScheme(scheme))
			nsn := types.NamespacedName{Namespace: "default", Name: "wandb-kafka"}
			claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
				Name: "data-wandb-kafka-etcd-0", Namespace: nsn.Namespace,
				Labels: map[string]string{"app.kubernetes.io/name": "wandb-kafka-etcd", "app.kubernetes.io/instance": nsn.Namespace},
			}}
			if tc.change != nil {
				tc.change(claim)
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim).Build()
			rule := ToKafkaOnDeleteRule(testWandb(), apiv2.RetentionPolicy{OnDelete: apiv2.PurgeOnDelete})
			rule.Policy = tc.policy
			require.NoError(t, PurgeFinalizer(context.Background(), cl, nsn, rule))
			err := cl.Get(context.Background(), types.NamespacedName{Name: claim.Name, Namespace: claim.Namespace}, &corev1.PersistentVolumeClaim{})
			if tc.wantDeleted {
				require.True(t, apierrors.IsNotFound(err), "claim not deleted: %v", err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
