package bufstream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func detachedBufstreamApp(replicas int32) *apiv2.Application {
	return &apiv2.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-kafka",
			Namespace: "default",
			// no owner references => detached
		},
		Spec: apiv2.ApplicationSpec{
			Replicas: ptr.To(replicas),
		},
	}
}

// A detached app sitting at the HA floor must be re-adoptable even though dev
// sizing requests fewer brokers than the floor, otherwise re-applying the CR
// can never reclaim the existing infra.
func TestCheckDetachedReadoptsBelowHAFloor(t *testing.T) {
	cl := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(detachedBufstreamApp(BufstreamReplicas)).
		Build()

	nsn := types.NamespacedName{Namespace: "default", Name: "wandb-kafka"}
	conds := CheckDetached(context.Background(), cl, nsn, types.UID("new-cr-uid"), 1)
	require.Nil(t, conds)
}

// A genuine size increase on a detached cluster must still block, so the
// operator never silently resizes infra a user deliberately detached.
func TestCheckDetachedBlocksRealMismatch(t *testing.T) {
	cl := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(detachedBufstreamApp(BufstreamReplicas)).
		Build()

	nsn := types.NamespacedName{Namespace: "default", Name: "wandb-kafka"}
	conds := CheckDetached(context.Background(), cl, nsn, types.UID("new-cr-uid"), 5)
	require.Len(t, conds, 1)
	require.Equal(t, common.DetachedSpecMismatch, conds[0].Reason)
}

// An app still owned by the CR is not detached, so CheckDetached is a no-op.
func TestCheckDetachedSkipsWhenOwned(t *testing.T) {
	app := detachedBufstreamApp(BufstreamReplicas)
	app.OwnerReferences = []metav1.OwnerReference{{UID: types.UID("cr-uid")}}
	cl := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(app).
		Build()

	nsn := types.NamespacedName{Namespace: "default", Name: "wandb-kafka"}
	conds := CheckDetached(context.Background(), cl, nsn, types.UID("cr-uid"), 5)
	require.Nil(t, conds)
}
