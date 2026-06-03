package moco

import (
	"context"
	"fmt"
	"maps"
	"strings"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "InnoDBCluster"
	AppConnTypeName  = "MySQLAppConn"

	// InvalidReplicaCountReason: manifest sizing yielded a replica count Moco rejects.
	InvalidReplicaCountReason = "InvalidReplicaCount"

	// ScaleDownUnsupportedReason: a reconcile would shrink the running cluster, which Moco forbids.
	ScaleDownUnsupportedReason = "ScaleDownUnsupported"
)

func WriteState(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	desired *mocov1beta2.MySQLCluster,
	confMap *corev1.ConfigMap,
	wandbLabels map[string]string,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.Mysql)
	var actual = &mocov1beta2.MySQLCluster{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := common.GetResource(
		ctx, cl, nsnBuilder.ClusterNsName(), ResourceTypeName, actual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   MySQLCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}
	}
	if !found {
		actual = nil
	}

	// MOCO's mutating admission webhook fills in spec.serverIDBase with a random
	// positive integer on Create; its validating webhook requires the value to
	// stay positive on Update. Our generated desired spec leaves the field at
	// zero, which would re-trip validation. Preserve the live value.
	if actual != nil {
		desired.Spec.ServerIDBase = actual.Spec.ServerIDBase
	}

	// Sizing is resolved from the manifest at reconcile time, after the CR
	// admission webhook runs, so a bad value reaches here unvalidated. Moco
	// requires a positive odd replica count; refuse to forward anything else
	// (rather than rewriting it) so the manifest stays the source of truth.
	if desired.Spec.Replicas <= 0 || desired.Spec.Replicas%2 == 0 {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: InvalidReplicaCountReason,
				Message: fmt.Sprintf(
					"manifest sizing produced %d MySQL replicas; Moco requires a positive odd number",
					desired.Spec.Replicas,
				),
			},
		}
	}

	// Backstop for the webhook's scale-down check: the webhook only sees the CR
	// (explicit edits), not the manifest-resolved count or the live cluster.
	// Catch those shrinks here instead of letting Moco reject them opaquely.
	if actual != nil && desired.Spec.Replicas < actual.Spec.Replicas {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: ScaleDownUnsupportedReason,
				Message: fmt.Sprintf(
					"cannot scale managed MySQL down from %d to %d replicas; Moco does not support in-place replica reduction (use its manual stop-clustering procedure)",
					actual.Spec.Replicas, desired.Spec.Replicas,
				),
			},
			{
				Type:   MySQLCustomResourceType,
				Status: metav1.ConditionTrue,
				Reason: common.ResourceExistsReason,
			},
		}
	}

	result := make([]metav1.Condition, 0)

	if confMap != nil {
		var actualConfMap = &corev1.ConfigMap{}
		cmNsName := types.NamespacedName{Name: confMap.Name, Namespace: confMap.Namespace}
		cmFound, cmErr := common.GetResource(ctx, cl, cmNsName, "ConfigMap", actualConfMap)
		if cmErr != nil {
			result = append(result, metav1.Condition{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			})
		} else {
			if !cmFound {
				actualConfMap = nil
			}
			if _, cmErr := common.CrudResource(ctx, cl, confMap, actualConfMap); cmErr != nil {
				result = append(result, metav1.Condition{
					Type:   common.ReconciledType,
					Status: metav1.ConditionFalse,
					Reason: common.ApiErrorReason,
				})
			}
		}
	}

	action, err := common.CrudResource(ctx, cl, desired, actual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	}

	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   MySQLCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   MySQLCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction:
		result = append(result, metav1.Condition{
			Type:   MySQLCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   MySQLCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	if len(wandbLabels) > 0 {
		if err := ensurePVCLabels(ctx, cl, specNamespacedName.Namespace, nsnBuilder.ClusterName(), wandbLabels); err != nil {
			result = append(result, metav1.Condition{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			})
		}
	}

	return result
}

// ensurePVCLabels stamps the wandb labels onto Moco's PVCs (missing because Moco
// doesn't propagate them through its StatefulSet volumeClaimTemplates), so
// purgeAssociatedResources can select them by label on teardown. Moco names PVCs
// "<dataVolumeName>-<cluster.PrefixedName()>-<ordinal>" (see Moco pvc.go); the
// prefix is built from those same sources so it can't drift from upstream.
func ensurePVCLabels(
	ctx context.Context,
	cl client.Client,
	namespace string,
	clusterName string,
	labels map[string]string,
) error {
	log := logx.GetSlog(ctx)
	cluster := &mocov1beta2.MySQLCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}}
	prefix := fmt.Sprintf("%s-%s-", dataVolumeName, cluster.PrefixedName())

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(ctx, pvcList, &client.ListOptions{Namespace: namespace}); err != nil {
		return err
	}

	for _, pvc := range pvcList.Items {
		if !strings.HasPrefix(pvc.Name, prefix) {
			continue
		}
		if common.HasAllLabelKeys(pvc.Labels, labels) {
			continue
		}
		patch := client.MergeFrom(pvc.DeepCopy())
		if pvc.Labels == nil {
			pvc.Labels = make(map[string]string)
		}
		maps.Copy(pvc.Labels, labels)
		if err := cl.Patch(ctx, &pvc, patch); err != nil {
			log.Error("failed to patch PVC labels", logx.ErrAttr(err), "pvc", pvc.Name)
			return err
		}
		log.Debug("patched wandb labels onto PVC", "pvc", pvc.Name)
	}
	return nil
}
