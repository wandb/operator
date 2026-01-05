package strimzi

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func RetainKafkaFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	return backupKafkaConnInfo(ctx, cl, specNamespacedName)
}

func backupKafkaConnInfo(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
) error {
	log := ctrl.LoggerFrom(ctx)

	nsnBuilder := createNsNameBuilder(specNamespacedName)
	backupNsName := nsnBuilder.ConnectionBackupNsName()

	// Get *current* Kafka connection info; cannot retain without it
	connInfo, err := readKafkaConnInfo(ctx, cl, nsnBuilder)
	if err != nil {
		return err
	}
	if connInfo == nil {
		return nil
	}

	// Delete (old) Kafka connection backup secret if it exists, just in case
	backup := &corev1.Secret{}
	backupFound, err := common.GetResource(
		ctx, cl, backupNsName, AppConnTypeName, backup,
	)
	if err != nil {
		return nil
	}
	if backupFound {
		if err = cl.Delete(ctx, backup); err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "delete failure of old backup Kafka connInfo secret")
				return err
			}
		}
	}

	// Build the new backup connInfo secret into with existing connInfo secret info
	// and create the new backup secret
	newBackup := buildConnInfoSecret(backupNsName, connInfo, nil)
	if err = cl.Create(ctx, newBackup); err != nil {
		log.Error(err, "create failure of new backup Kafka connInfo secret")
		return err
	}

	return nil
}
