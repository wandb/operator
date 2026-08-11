package moco

import (
	"context"
	"fmt"
	"maps"
	"strings"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	apiv2 "github.com/wandb/operator/api/v2"
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
	} else if len(desired.OwnerReferences) == 0 {
		if err := removeLegacyCrossNamespaceOwnerReferences(ctx, cl, actual); err != nil {
			return []metav1.Condition{
				{
					Type:   common.ReconciledType,
					Status: metav1.ConditionFalse,
					Reason: common.ApiErrorReason,
				},
			}
		}
	}

	// MOCO's mutating admission webhook fills in spec.serverIDBase with a random
	// positive integer on Create; its validating webhook requires the value to
	// stay positive on Update. Our generated desired spec leaves the field at
	// zero, which would re-trip validation. Preserve the live value.
	if actual != nil {
		desired.Spec.ServerIDBase = actual.Spec.ServerIDBase
	}

	// Sizing is resolved from the manifest at reconcile time, after the CR
	// admission webhook runs, so a bad value reaches here unvalidated.
	if !apiv2.ValidMysqlReplicaCount(desired.Spec.Replicas) {
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
			} else if len(confMap.OwnerReferences) == 0 {
				if cmErr := removeLegacyCrossNamespaceOwnerReferences(ctx, cl, actualConfMap); cmErr != nil {
					result = append(result, metav1.Condition{
						Type:   common.ReconciledType,
						Status: metav1.ConditionFalse,
						Reason: common.ApiErrorReason,
					})
				}
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
	case common.UpdateAction, common.UnchangedAction:
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
		managedAnnotations := map[string]string{
			RetentionPolicyAnnotation: desired.Annotations[RetentionPolicyAnnotation],
		}
		if err := ensurePVCMetadata(ctx, cl, specNamespacedName.Namespace, nsnBuilder.ClusterName(), wandbLabels, managedAnnotations); err != nil {
			result = append(result, metav1.Condition{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			})
		}
		if err := ensureCredentialMetadata(ctx, cl, specNamespacedName.Namespace, nsnBuilder.ClusterName(), wandbLabels, managedAnnotations); err != nil {
			result = append(result, metav1.Condition{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			})
		}
	}

	return result
}

func removeLegacyCrossNamespaceOwnerReferences(ctx context.Context, cl client.Client, obj client.Object) error {
	ownerReferences := obj.GetOwnerReferences()
	filtered := make([]metav1.OwnerReference, 0, len(ownerReferences))
	for _, ref := range ownerReferences {
		if ref.Kind == "WeightsAndBiases" && strings.HasPrefix(ref.APIVersion, apiv2.GroupVersion.Group+"/") {
			continue
		}
		filtered = append(filtered, ref)
	}
	if len(filtered) == len(ownerReferences) {
		return nil
	}
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	obj.SetOwnerReferences(filtered)
	return cl.Patch(ctx, obj, patch)
}

func ensureCredentialMetadata(
	ctx context.Context,
	cl client.Client,
	namespace string,
	clusterName string,
	desiredLabels map[string]string,
	desiredAnnotations map[string]string,
) error {
	secret := &corev1.Secret{}
	nsName := types.NamespacedName{Namespace: namespace, Name: "moco-" + clusterName}
	if err := cl.Get(ctx, nsName, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		return err
	}
	if hasMetadataValues(secret, desiredLabels, desiredAnnotations) {
		return nil
	}
	patch := client.MergeFrom(secret.DeepCopy())
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	maps.Copy(secret.Labels, desiredLabels)
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	maps.Copy(secret.Annotations, desiredAnnotations)
	return cl.Patch(ctx, secret, patch)
}

func hasMetadataValues(obj client.Object, desiredLabels, desiredAnnotations map[string]string) bool {
	for key, value := range desiredLabels {
		if obj.GetLabels()[key] != value {
			return false
		}
	}
	for key, value := range desiredAnnotations {
		if obj.GetAnnotations()[key] != value {
			return false
		}
	}
	return true
}

// ensurePVCMetadata stamps provenance and retention metadata onto Moco's PVCs
// (Moco does not propagate it through StatefulSet volumeClaimTemplates), so
// lifecycle cleanup can select the complete instance inventory. Moco names
// PVCs "<dataVolumeName>-<cluster.PrefixedName()>-<ordinal>" (see Moco pvc.go).
func ensurePVCMetadata(
	ctx context.Context,
	cl client.Client,
	namespace string,
	clusterName string,
	desiredLabels map[string]string,
	desiredAnnotations map[string]string,
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
		if hasMetadataValues(&pvc, desiredLabels, desiredAnnotations) {
			continue
		}
		patch := client.MergeFrom(pvc.DeepCopy())
		if pvc.Labels == nil {
			pvc.Labels = make(map[string]string)
		}
		maps.Copy(pvc.Labels, desiredLabels)
		if pvc.Annotations == nil {
			pvc.Annotations = make(map[string]string)
		}
		maps.Copy(pvc.Annotations, desiredAnnotations)
		if err := cl.Patch(ctx, &pvc, patch); err != nil {
			log.Error("failed to patch PVC metadata", logx.ErrAttr(err), "pvc", pvc.Name)
			return err
		}
		log.Debug("patched wandb metadata onto PVC", "pvc", pvc.Name)
	}
	return nil
}
