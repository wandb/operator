package tenant

import (
	"context"

	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func purgeAssociatedResources(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
) error {
	log := logx.GetSlog(ctx)
	tenantName := TenantName(specNamespacedName.Name)
	labelKey := "v1.min.io/tenant"
	tenantLabels := labels.Set{labelKey: tenantName}

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
			"count", len(pvcList.Items), "labelKey", labelKey, "labelValue", tenantName,
		)
	} else {
		log.Debug(
			"No associated PVCs found to purge",
			"labelKey", labelKey, "labelValue", tenantName,
		)
	}
	for _, pvc := range pvcList.Items {
		if err := cl.Delete(ctx, &pvc); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	// Delete Secrets with the specified label
	secretList := &corev1.SecretList{}
	if err := cl.List(ctx, secretList, listOptions); err != nil {
		return err
	}

	// Delete each Secret
	if len(secretList.Items) > 0 {
		log.Info(
			"Purging associated Secrets",
			"count", len(secretList.Items), "labelKey", labelKey, "labelValue", tenantName,
		)
	} else {
		log.Debug(
			"No associated Secrets found to purge",
			"labelKey", labelKey, "labelValue", tenantName,
		)
	}
	for _, secret := range secretList.Items {
		if err := cl.Delete(ctx, &secret); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}
