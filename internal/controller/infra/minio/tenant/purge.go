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

func PurgeFinalizer(ctx context.Context, client client.Client, specNamespacedName types.NamespacedName) error {
	onDeleteSelector := labels.SelectorFromSet(map[string]string{
		"app.kubernetes.io/managed-by": "wandb-operator",
		"app.kubernetes.io/component":  "minio",
		"app.kubernetes.io/instance":   "minio",
	})

	return purgeAssociatedResources(ctx, client, onDeleteSelector)
}

func purgeAssociatedResources(
	ctx context.Context,
	cl client.Client,
	onDeleteSelector labels.Selector,
) error {
	log := logx.GetSlog(ctx)
	// Delete PVCs with the specified label
	pvcList := &corev1.PersistentVolumeClaimList{}
	listOptions := &client.ListOptions{
		LabelSelector: onDeleteSelector,
	}

	if err := cl.List(ctx, pvcList, listOptions); err != nil {
		return err
	}

	// Delete each PVC
	if len(pvcList.Items) > 0 {
		log.Info(
			"Purging associated PVCs",
			"count", len(pvcList.Items), "selector", onDeleteSelector.String(),
		)
	} else {
		log.Debug(
			"No associated PVCs found to purge",
			"selector", onDeleteSelector.String(),
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
			"count", len(secretList.Items), "selector", onDeleteSelector.String(),
		)
	} else {
		log.Debug(
			"No associated Secrets found to purge",
			"selector", onDeleteSelector.String(),
		)
	}
	for _, secret := range secretList.Items {
		if err := cl.Delete(ctx, &secret); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}
