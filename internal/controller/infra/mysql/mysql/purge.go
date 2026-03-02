package mysql

import (
	"context"
	"fmt"
	"strings"

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
	cl client.Client,
	specNamespacedName types.NamespacedName,
	onDeleteRule translator.OnDeleteRule,
) error {
	logx.WithSlog(ctx, MySQLCustomResourceType)
	if onDeleteRule.Policy != translator.Purge {
		return nil
	}
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	return purgeAssociatedResources(ctx, cl, specNamespacedName.Namespace, nsnBuilder.ClusterName(), onDeleteRule.Selector)
}

// purgeAssociatedResources deletes PVCs and Secrets associated with the MySQL cluster.
//
// PVCs are identified by name prefix (datadir-<clusterName>-) because the mysql-operator
// (oracle/mysql-operator) creates PVCs via StatefulSet volumeClaimTemplates and may not
// propagate custom labels from DatadirVolumeClaimTemplate.ObjectMeta to the actual PVCs.
//
// Secrets are identified by the wandb label selector since we create the db-password
// Secret directly with those labels.
func purgeAssociatedResources(
	ctx context.Context,
	cl client.Client,
	namespace string,
	clusterName string,
	onDeleteSelector labels.Selector,
) error {
	log := logx.GetSlog(ctx)

	pvcPrefix := fmt.Sprintf("datadir-%s-", clusterName)
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(ctx, pvcList, &client.ListOptions{Namespace: namespace}); err != nil {
		return err
	}

	var matchedPVCs []corev1.PersistentVolumeClaim
	for _, pvc := range pvcList.Items {
		if strings.HasPrefix(pvc.Name, pvcPrefix) {
			matchedPVCs = append(matchedPVCs, pvc)
		}
	}

	if len(matchedPVCs) > 0 {
		log.Info(
			"Purging associated PVCs",
			"count", len(matchedPVCs), "prefix", pvcPrefix,
		)
	} else {
		log.Debug(
			"No associated PVCs found to purge",
			"prefix", pvcPrefix,
		)
	}
	for _, pvc := range matchedPVCs {
		if err := cl.Delete(ctx, &pvc); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	secretList := &corev1.SecretList{}
	if err := cl.List(ctx, secretList, &client.ListOptions{Namespace: namespace, LabelSelector: onDeleteSelector}); err != nil {
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
