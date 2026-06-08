package keeper

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	chkv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse-keeper.altinity.com/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ResourceTypeName is the kind used for logging/error reporting of the CHK CR.
const ResourceTypeName = "ClickHouseKeeperInstallation"

// WriteState patches only the fields we own onto the CHK. CreateOrPatch diffs via
// unstructured JSON (dropping the vendored runtime/status fields whose unexported
// members panic Semantic.DeepEqual) and patches just the diff, so the
// Altinity-managed finalizer and status survive untouched.
func WriteState(
	ctx context.Context,
	cl client.Client,
	keeperNsName types.NamespacedName,
	desired *chkv1.ClickHouseKeeperInstallation,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.ClickHouse)

	obj := &chkv1.ClickHouseKeeperInstallation{
		ObjectMeta: metav1.ObjectMeta{Name: keeperNsName.Name, Namespace: keeperNsName.Namespace},
	}

	op, err := controllerutil.CreateOrPatch(ctx, cl, obj, func() error {
		// Set only the fields we own; leave finalizers/status/annotations intact.
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		for k, v := range desired.GetLabels() {
			labels[k] = v
		}
		obj.SetLabels(labels)
		obj.SetOwnerReferences(desired.GetOwnerReferences())
		obj.Spec = desired.Spec
		return nil
	})
	if err != nil {
		return []metav1.Condition{
			{Type: KeeperCustomResourceType, Status: metav1.ConditionUnknown, Reason: common.ApiErrorReason},
		}
	}

	if op == controllerutil.OperationResultCreated {
		return []metav1.Condition{
			{Type: KeeperCustomResourceType, Status: metav1.ConditionFalse, Reason: common.PendingCreateReason},
		}
	}
	return []metav1.Condition{
		{Type: KeeperCustomResourceType, Status: metav1.ConditionTrue, Reason: common.ResourceExistsReason},
	}
}
