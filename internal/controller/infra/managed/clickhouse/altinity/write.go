package altinity

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity/keeper"
	"github.com/wandb/operator/internal/logx"
	chkv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse-keeper.altinity.com/v1"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	ResourceTypeName = "ClickHouseInstallation"
	AppConnTypeName  = "ClickHouseAppConn"
)

// WriteState reconciles the Keeper ensemble (first, since ReplicatedMergeTree
// depends on it) and the ClickHouse installation.
func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desiredKeeper *chkv1.ClickHouseKeeperInstallation,
	desired *chiv1.ClickHouseInstallation,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.ClickHouse)
	results := make([]metav1.Condition, 0)

	results = append(results, keeper.WriteState(
		ctx, client,
		types.NamespacedName{Namespace: desiredKeeper.Namespace, Name: desiredKeeper.Name},
		desiredKeeper,
	)...)
	results = append(results, writeClickHouseInstallation(ctx, client, specNamespacedName, desired)...)

	return results
}

// writeClickHouseInstallation patches only the fields we own onto the CHI.
// CreateOrPatch diffs via unstructured JSON (dropping the vendored runtime/status
// fields whose unexported members panic Semantic.DeepEqual) and patches just the
// diff, so the Altinity-managed finalizer and status survive untouched.
func writeClickHouseInstallation(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	desired *chiv1.ClickHouseInstallation,
) []metav1.Condition {
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	obj := &chiv1.ClickHouseInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.InstallationName(),
			Namespace: nsnBuilder.Namespace(),
		},
	}

	op, err := controllerutil.CreateOrPatch(ctx, cl, obj, func() error {
		applyOwnedMetadata(obj, desired)
		obj.Spec = desired.Spec
		return nil
	})
	if err != nil {
		return []metav1.Condition{
			{Type: common.ReconciledType, Status: metav1.ConditionFalse, Reason: common.ApiErrorReason},
			{Type: ClickHouseCustomResourceType, Status: metav1.ConditionUnknown, Reason: common.ApiErrorReason},
		}
	}

	return []metav1.Condition{customResourceConditionForOp(ClickHouseCustomResourceType, op)}
}

// applyOwnedMetadata sets the metadata we own (merged labels, owner references),
// leaving finalizers/annotations owned by the resource's operator untouched.
func applyOwnedMetadata(obj, desired metav1.Object) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for k, v := range desired.GetLabels() {
		labels[k] = v
	}
	obj.SetLabels(labels)
	obj.SetOwnerReferences(desired.GetOwnerReferences())
}

// customResourceConditionForOp maps a create-or-update result to the resource's
// existence condition (created => pending, updated/unchanged => exists).
func customResourceConditionForOp(conditionType string, op controllerutil.OperationResult) metav1.Condition {
	if op == controllerutil.OperationResultCreated {
		return metav1.Condition{Type: conditionType, Status: metav1.ConditionFalse, Reason: common.PendingCreateReason}
	}
	return metav1.Condition{Type: conditionType, Status: metav1.ConditionTrue, Reason: common.ResourceExistsReason}
}
