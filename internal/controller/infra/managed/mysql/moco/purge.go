package moco

import (
	"context"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PurgeFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	onDeleteRule common.OnDeleteRule,
) error {
	ctx, _ = logx.WithSlog(ctx, logx.Mysql)
	if onDeleteRule.Policy != common.Purge {
		return nil
	}
	cluster := &mocov1beta2.MySQLCluster{}
	if err := cl.Get(ctx, specNamespacedName, cluster); err == nil {
		if err := cl.Delete(ctx, cluster); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else if !errors.IsNotFound(err) {
		return err
	}

	configMap := &corev1.ConfigMap{}
	configMapName := types.NamespacedName{
		Namespace: specNamespacedName.Namespace,
		Name:      MyCnfConfigMapName(specNamespacedName.Name),
	}
	if err := cl.Get(ctx, configMapName, configMap); err == nil {
		if err := cl.Delete(ctx, configMap); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else if !errors.IsNotFound(err) {
		return err
	}
	credentials := &corev1.Secret{}
	credentialsName := types.NamespacedName{
		Namespace: specNamespacedName.Namespace,
		Name:      "moco-" + specNamespacedName.Name,
	}
	if err := cl.Get(ctx, credentialsName, credentials); err == nil {
		if err := cl.Delete(ctx, credentials); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else if !errors.IsNotFound(err) {
		return err
	}
	return purgeAssociatedResources(ctx, cl, specNamespacedName.Namespace, onDeleteRule.Selector)
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

	// PVCs
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

	// Secrets
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
