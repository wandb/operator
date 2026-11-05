package bufstream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReadStatePreservesEtcdClaimsWhenBrokerApplicationIsMissing(t *testing.T) {
	ctx := context.Background()
	wandb := testWandb()
	wandb.Spec.RetentionPolicy.OnDelete = apiv2.PurgeOnDelete
	nsn := types.NamespacedName{Namespace: "default", Name: "wandb-kafka"}
	scheme := testScheme(t)
	require.NoError(t, corev1.AddToScheme(scheme))
	etcdApp, err := ToEtcdApplication(wandb, CreateNsNameBuilder(nsn), scheme, manifest.Manifest{})
	require.NoError(t, err)

	claim := etcdApp.Spec.VolumeClaimTemplates[0].DeepCopy()
	claim.Name = "data-wandb-kafka-etcd-0"
	claim.Namespace = nsn.Namespace
	legacyClaim := claim.DeepCopy()
	legacyClaim.Name = "data-wandb-kafka-etcd-1"
	legacyClaim.Labels = map[string]string{
		"app.kubernetes.io/name": "wandb-kafka-etcd", "app.kubernetes.io/instance": nsn.Namespace,
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, etcdApp, claim, legacyClaim).Build()
	conditions, connection := ReadState(ctx, cl, nsn, wandb)

	for _, pvc := range []*corev1.PersistentVolumeClaim{claim, legacyClaim} {
		actual := &corev1.PersistentVolumeClaim{}
		require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(pvc), actual), "a missing broker must not delete etcd data")
		require.True(t, actual.DeletionTimestamp.IsZero())
	}
	require.Nil(t, connection)
	require.Contains(t, conditions, metav1.Condition{
		Type: BufstreamApplicationType, Status: metav1.ConditionFalse, Reason: common.NoResourceReason,
	})
}
