package tenant

import (
	"context"

	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PurgeFinalizer(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	onDeleteRule translator.OnDeleteRule,
) error {
	ctx, _ = logx.WithSlog(ctx, logx.Minio)
	if onDeleteRule.Policy != translator.Purge {
		return nil
	}
	return purgeAssociatedResources(ctx, client, specNamespacedName.Namespace, onDeleteRule.Selector)
}

func purgeAssociatedResources(
	ctx context.Context,
	cl client.Client,
	namespace string,
	onDeleteSelector labels.Selector,
) error {
	log := logx.GetSlog(ctx)
	listOptions := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: onDeleteSelector,
	}

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(ctx, pvcList, listOptions); err != nil {
		return err
	}

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

	secretList := &corev1.SecretList{}
	if err := cl.List(ctx, secretList, listOptions); err != nil {
		return err
	}

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
