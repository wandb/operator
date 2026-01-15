package strimzi

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	strimziv1 "github.com/wandb/operator/pkg/vendored/strimzi-kafka/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// restoreKafkaConnInfo will setup the Kafka status will some configuration details for new Kafka NodePools when:
// * a connection info secret is present from the previous cluster
// * a PVC still exists from the previous cluster
func restoreKafkaConnInfo(
	ctx context.Context,
	cl client.Client,
	nsnBuilder *NsNameBuilder,
	desired *strimziv1.KafkaNodePool,
	actual *strimziv1.KafkaNodePool,
) error {
	log := logx.GetSlog(ctx)

	var connInfo *kafkaConnInfo
	var err error
	var found bool

	// if there is existing connection info from the previous cluster
	connInfo, err = readKafkaConnInfo(ctx, cl, nsnBuilder)
	if err != nil {
		return err
	}
	if connInfo == nil {
		log.Info("restore: abort with no connInfo")
		return nil
	}

	// if it has a clusterID
	if connInfo.ClusterId == "" {
		log.Info("restore: abort with blank ClusterId")
		return nil
	}
	log.Debug("restore: valid connection info found")

	// if there is a PVC from the previous cluster
	var pvc = &corev1.PersistentVolumeClaim{}
	if found, err = common.GetResource(
		ctx, cl, nsnBuilder.PvcNsName(0, 0), "PersistentVolumeClaim", pvc,
	); err != nil {
		return err
	}
	if !found {
		log.Info("restore: abort with no PVC", "name", nsnBuilder.PvcName(0, 0))
		return nil
	}
	log.Debug("restore: PVC found")

	log.Info("restore: set clusterId", "clusterId", connInfo.ClusterId)
	desired.Status.ClusterId = connInfo.ClusterId
	if err = cl.Status().Update(ctx, desired); err != nil {
		return err
	}

	return nil
}
