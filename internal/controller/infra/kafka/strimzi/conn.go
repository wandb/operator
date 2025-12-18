package strimzi

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	strimziv1 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type kafkaConnInfo struct {
	Host      string
	Port      string
	Username  string
	Password  string
	ClusterId string
}

func (c *kafkaConnInfo) toURL() string {
	return fmt.Sprintf("kafka://%s:%s", c.Host, c.Port)
}

func readKafkaConnInfo(
	ctx context.Context,
	cl client.Client,
	nsnBuilder *NsNameBuilder,
) (*kafkaConnInfo, error) {
	var err error
	var found bool
	var actual = &corev1.Secret{}

	nsName := nsnBuilder.ConnectionNsName()

	if found, err = common.GetResource(
		ctx, cl, nsName, AppConnTypeName, actual,
	); err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("kafka connection secret %s not found", nsName)
	}

	return &kafkaConnInfo{
		Host:      string(actual.Data["Host"]),
		Port:      string(actual.Data["Port"]),
		ClusterId: string(actual.Data["ClusterID"]),
	}, nil
}

func writeKafkaConnInfo(
	ctx context.Context,
	cl client.Client,
	owner client.Object,
	nsnBuilder *NsNameBuilder,
	connInfo *kafkaConnInfo,
) (
	*translator.InfraConnection, error,
) {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var found bool
	var gvk schema.GroupVersionKind
	var actual = &corev1.Secret{}

	nsName := nsnBuilder.ConnectionNsName()
	urlKey := "url"

	if found, err = common.GetResource(
		ctx, cl, nsName, AppConnTypeName, actual,
	); err != nil {
		return nil, err
	}
	if !found {
		actual = nil
	}

	// prefer existing strimzi CR clusterId to wandb Secret clusterId
	// but take a non-blank value of a blank one
	nextClusterId := ""
	wandbSecretClusterId := ""
	if actual != nil {
		wandbSecretClusterId = string(actual.Data["ClusterID"])
	}
	strimziCrClusterId := connInfo.ClusterId
	if wandbSecretClusterId == strimziCrClusterId {
		nextClusterId = strimziCrClusterId // no change
	} else if wandbSecretClusterId != "" && strimziCrClusterId != "" {
		log.Info("Kafka clusterId replace wandb secret with strimzi CR status",
			"wandbSecretClusterId", wandbSecretClusterId, "strimziCrClusterId", strimziCrClusterId)
		nextClusterId = strimziCrClusterId
	} else if wandbSecretClusterId != "" {
		log.Info("Kafka clusterId use existing wandb secret", "wandbSecretClusterId", wandbSecretClusterId)
		nextClusterId = wandbSecretClusterId
	} else if strimziCrClusterId != "" {
		log.Info("Kafka clusterId use existing strimzi CR status", "strimziCrClusterId", strimziCrClusterId)
		nextClusterId = strimziCrClusterId
	}

	if gvk, err = cl.GroupVersionKindFor(owner); err != nil {
		log.Error(err, fmt.Sprintf("Error getting GVK for %s", owner.GetName()))
		return nil, fmt.Errorf("could not get GVK for owner: %w", err)
	}
	ref := metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		Controller:         ptr.To(false),
		BlockOwnerDeletion: ptr.To(false),
	}

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            nsName.Name,
			Namespace:       nsName.Namespace,
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			urlKey:      connInfo.toURL(),
			"Host":      connInfo.Host,
			"Port":      connInfo.Port,
			"ClusterID": nextClusterId,
		},
	}

	if err = common.CrudResource(ctx, cl, desired, actual); err != nil {
		return nil, err
	}

	return &translator.InfraConnection{
		URL: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: nsName.Name,
			},
			Key:      urlKey,
			Optional: ptr.To(false),
		},
	}, nil
}

// restoreKafkaConnInfo will setup the Kafka status will some configuration details when:
// * it is a newly created Kafka cluster
// * a connection info secret is present from the previous cluster
// * a PVC still exists from the previous cluster
func restoreKafkaConnInfo(
	ctx context.Context,
	cl client.Client,
	nsnBuilder *NsNameBuilder,
	desired *strimziv1.Kafka,
	actual *strimziv1.Kafka,
) error {
	log := ctrl.LoggerFrom(ctx)

	var connInfo *kafkaConnInfo
	var err error
	var found bool

	// if is a newly created cluster
	if actual != nil || desired == nil {
		return nil
	}

	// if there is existing connection info from the previous cluster
	connInfo, err = readKafkaConnInfo(ctx, cl, nsnBuilder)
	if err != nil {
		return err
	}
	if connInfo == nil {
		return nil
	}

	// if it has a clusterID
	if connInfo.ClusterId == "" {
		return nil
	}

	// if there is a PVC from the previous cluster
	var pvc = &corev1.PersistentVolumeClaim{}
	if found, err = common.GetResource(
		ctx, cl, nsnBuilder.PvcNsName(0, 0), "PersistentVolumeClaim", pvc,
	); err != nil {
		return err
	}
	if !found {
		return nil
	}

	log.Info("restoring Kafka connection info", "clusterId", connInfo.ClusterId)
	desired.Status.ClusterId = connInfo.ClusterId
	if err = cl.Status().Update(ctx, desired); err != nil {
		return err
	}

	return nil
}
