package mysql

import (
	"context"
	"fmt"
	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"maps"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

const (
	ResourceTypeName = "InnoDBCluster"
	AppConnTypeName  = "MySQLAppConn"
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

// ensurePVCLabels patches any PVCs belonging to the mysql cluster that are
// missing the wandb labels. PVCs are identified by the name prefix
// "datadir-<clusterName>-" since the mysql-operator creates them via
// StatefulSet volumeClaimTemplates and may not propagate custom labels.
func ensurePVCLabels(
	ctx context.Context,
	cl client.Client,
	namespace string,
	clusterName string,
	labels map[string]string,
) error {
	log := logx.GetSlog(ctx)
	prefix := fmt.Sprintf("datadir-%s-", clusterName)

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
