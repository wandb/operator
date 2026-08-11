package moco

import (
	"context"
	"fmt"
	"strings"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	if err := purgeClusterPVCs(ctx, cl, specNamespacedName); err != nil {
		return err
	}
	return purgeAssociatedResources(ctx, cl, specNamespacedName.Namespace, onDeleteRule.Selector)
}

// purgeClusterPVCs deletes the PVCs Moco created for this cluster, selected by
// Moco's PVC naming scheme rather than by label, so purging works for clusters
// created before the provenance labels existed without a selector wide enough
// to also match a sibling instance's storage.
func purgeClusterPVCs(ctx context.Context, cl client.Client, specNamespacedName types.NamespacedName) error {
	log := logx.GetSlog(ctx)
	cluster := &mocov1beta2.MySQLCluster{ObjectMeta: metav1.ObjectMeta{Name: specNamespacedName.Name}}
	prefix := fmt.Sprintf("%s-%s-", dataVolumeName, cluster.PrefixedName())

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(ctx, pvcList, &client.ListOptions{Namespace: specNamespacedName.Namespace}); err != nil {
		return err
	}
	for i := range pvcList.Items {
		pvc := &pvcList.Items[i]
		if !strings.HasPrefix(pvc.Name, prefix) {
			continue
		}
		if err := cl.Delete(ctx, pvc); err != nil && !errors.IsNotFound(err) {
			return err
		}
		log.Info("Purged MySQL cluster PVC", "pvc", pvc.Name)
	}
	return nil
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
