package altinity

import (
	"context"

	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// purgeAssociatedResource will remove PVCs by associated label *only* if the
// associated resource (i.e. ClickHouse installation) is not present.
func purgeAssociatedResources(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
) error {
	log := logx.GetSlog(ctx)
	installationName := InstallationName(specNamespacedName.Name)
	labelKey := "clickhouse.altinity.com/chi"
	tenantLabels := labels.Set{labelKey: installationName}

	// Delete PVCs with the specified label
	pvcList := &corev1.PersistentVolumeClaimList{}
	listOptions := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(tenantLabels),
		Namespace:     specNamespacedName.Namespace,
	}

	if err := cl.List(ctx, pvcList, listOptions); err != nil {
		return err
	}

	// Delete each PVC
	if len(pvcList.Items) > 0 {
		log.Info(
			"Purging associated PVCs",
			"count", len(pvcList.Items), "labelKey", labelKey, "labelValue", installationName,
		)
	} else {
		log.Debug(
			"No associated PVCs found to purge",
			"labelKey", labelKey, "labelValue", installationName,
		)
	}
	for _, pvc := range pvcList.Items {
		if err := cl.Delete(ctx, &pvc); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}
