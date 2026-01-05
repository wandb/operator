package strimzi

import (
	"context"
	"errors"

	"github.com/wandb/operator/internal/controller/common"
	strimziv1 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// restoreKafkaBackupConnInfo will setup the Kafka status will some configuration details for new Kafka NodePools when:
// * a connection info secret is present from the previous cluster
// * a PVC still exists from the previous cluster
// returns whether restoration steps have completed and any error
func restoreKafkaBackupConnInfo(
	ctx context.Context,
	cl client.Client,
	wandbOwner client.Object,
	nsnBuilder *NsNameBuilder,
	desired *strimziv1.KafkaNodePool,
	actual *strimziv1.KafkaNodePool,
) error {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var connFound, backupFound bool

	log.Info("Kafka restore connection info...")

	// get *backup* connection info; stop the restore attempt, otherwise
	backupConnSecret := &corev1.Secret{}
	if backupFound, err = common.GetResource(
		ctx, cl, nsnBuilder.ConnectionBackupNsName(), AppBackupConnTypeName, backupConnSecret,
	); err != nil {
		return err
	}
	if !backupFound {
		return nil
	}
	backupConnInfo := toKafkaConnInfo(ctx, backupConnSecret)

	// get *current* connection info if it exists
	connSecret := &corev1.Secret{}
	if connFound, err = common.GetResource(
		ctx, cl, nsnBuilder.ConnectionNsName(), AppConnTypeName, connSecret,
	); err != nil {
		return err
	}

	// then delete the *current* kafka conn info secret, if present
	if connFound {
		if err = cl.Delete(ctx, connSecret); err != nil && !k8serr.IsNotFound(err) {
			return err
		}
	}

	// write the backupConnInfo as the new connInfo secret
	if _, err = writeKafkaConnInfo(ctx, cl, wandbOwner, nsnBuilder, backupConnInfo); err != nil {
		return err
	}

	return nil
}

// restoreKafkaNodePoolClusterId will setup the KafkaNodePool status with clusterId to the one stored in kafka-connection-info
// * The connection info secret must be present
// * The clusterId must not be blank; otherwise it will error
// * The PVCs must still exist from the previous cluster
// returns with any error
func restoreKafkaNodePoolClusterId(
	ctx context.Context,
	cl client.Client,
	nsnBuilder *NsNameBuilder,
	desired *strimziv1.KafkaNodePool,
) error {
	log := ctrl.LoggerFrom(ctx)

	// Get connInfo (may have been recently restored from backup)
	connInfo, err := readKafkaConnInfo(ctx, cl, nsnBuilder)
	if err != nil {
		return err
	}
	if connInfo == nil {
		return nil
	}

	// if connInfo has no clusterID, this is an error
	if connInfo.ClusterId == "" {
		err = errors.New("kafka restore will fail with blank clusterId")
		log.Error(err, "kafka restore error")
		return nil
	}

	// if there are no PVCs from the previous kafka installation, it is an error
	var pvc = &corev1.PersistentVolumeClaim{}
	found, err := common.GetResource(
		ctx, cl, nsnBuilder.PvcNsName(0, 0), "PersistentVolumeClaim", pvc,
	)
	if err != nil {
		return err
	}
	if !found {
		err = errors.New("kafka restore will fail with missing PVCs")
		log.Error(err, "Kafka restore error", "name", nsnBuilder.PvcName(0, 0))
		return nil
	}

	log.Info("restoring Kafka connection info", "clusterId", connInfo.ClusterId)
	desired.Status.ClusterId = connInfo.ClusterId
	if err = cl.Status().Update(ctx, desired); err != nil {
		return err
	}
	return nil
}
