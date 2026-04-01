package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/kafka/strimzi"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/vendored/strimzi-kafka/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func kafkaWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	if wandb.Spec.Kafka.ManagedKafka != nil {
		return managedKafkaWriteState(ctx, client, wandb)
	}
	if wandb.Spec.Kafka.ExternalKafka != nil {
		return externalKafkaWriteState(ctx, client, wandb)
	}
	return nil
}

func kafkaReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.KafkaConnection) {
	if wandb.Spec.Kafka.ManagedKafka != nil {
		return managedKafkaReadState(ctx, client, wandb, newConditions)
	}
	if wandb.Spec.Kafka.ExternalKafka != nil {
		return externalKafkaReadState(ctx, client, wandb, newConditions)
	}
	return newConditions, nil
}

func kafkaInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.KafkaConnection,
) (ctrl.Result, error) {
	if wandb.Spec.Kafka.ManagedKafka != nil {
		return managedKafkaInferStatus(ctx, client, recorder, wandb, newConditions, newInfraConn)
	}
	if wandb.Spec.Kafka.ExternalKafka != nil {
		return externalKafkaInferStatus(ctx, client, wandb, newConditions, newInfraConn)
	}
	return ctrl.Result{}, nil
}

func kafkaPurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	if spec := wandb.Spec.Kafka.ManagedKafka; spec != nil {
		specNamespacedName := managedKafkaSpecNamespacedName(spec)
		onDeleteRule := translatorv2.ToKafkaOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
		return strimzi.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if wandb.Spec.Kafka.ExternalKafka != nil {
		return deleteWandbConnectionSecret(ctx, client, types.NamespacedName{
			Namespace: wandb.Namespace,
			Name:      kafkaConnectionName,
		})
	}
	return nil
}

func kafkaDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	spec := wandb.Spec.Kafka.ManagedKafka
	if spec == nil {
		return nil
	}
	specNamespacedName := managedKafkaSpecNamespacedName(spec)
	return strimzi.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

// managed

func managedKafkaWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	spec := wandb.Spec.Kafka.ManagedKafka

	log := ctrl.LoggerFrom(ctx)
	var desiredKafka *v1.Kafka
	desiredKafka, err := translatorv2.ToKafkaVendorSpec(
		ctx,
		wandb,
		client.Scheme(),
	)
	if err != nil {
		log.Error(err, "failed to translate kafka spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	var desiredNodePool *v1.KafkaNodePool
	desiredNodePool, err = translatorv2.ToKafkaNodePoolVendorSpec(
		ctx,
		wandb,
		client.Scheme(),
	)
	if err != nil {
		log.Error(err, "failed to translate kafka node pool spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	specNamespacedName := managedKafkaSpecNamespacedName(spec)

	if conditions := strimzi.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID(), spec.Replicas); conditions != nil {
		return conditions
	}

	results := make([]metav1.Condition, 0)
	results = append(results, strimzi.WriteState(ctx, client, specNamespacedName, desiredKafka, desiredNodePool)...)

	return results
}

func managedKafkaReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.KafkaConnection) {
	spec := wandb.Spec.Kafka.ManagedKafka

	specNamespacedName := managedKafkaSpecNamespacedName(spec)
	onDeleteRule := translatorv2.ToKafkaOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
	readConditions, newInfraConn := strimzi.ReadState(ctx, client, specNamespacedName, wandb, onDeleteRule)
	newConditions = append(newConditions, readConditions...)
	return newConditions, newInfraConn
}

func managedKafkaInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.KafkaConnection,
) (ctrl.Result, error) {
	oldConditions := wandb.Status.KafkaStatus.Conditions
	oldInfraConn := translatorv2.ToTranslatorKafkaConnection(wandb.Status.KafkaStatus.Connection)

	enabled := true
	updatedStatus, events, ctrlResult := strimzi.ComputeStatus(
		ctx,
		enabled,
		oldConditions,
		newConditions,
		utils.Coalesce(newInfraConn, &oldInfraConn),
		wandb.Generation,
	)
	for _, e := range events {
		recorder.Event(wandb, e.Type, e.Reason, e.Message)
	}
	wandb.Status.KafkaStatus = translatorv2.ToWbKafkaInfraStatus(updatedStatus)
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

// external

func externalKafkaWriteState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	spec := wandb.Spec.Kafka.ExternalKafka
	logger := ctrl.LoggerFrom(ctx)

	fields := map[string]corev1.SecretKeySelector{
		"url":            spec.URL,
		"BrokerEndpoint": spec.BrokerEndpoint,
		"Host":           spec.Host,
		"Port":           spec.Port,
	}

	data := map[string]string{}
	for key, sel := range fields {
		val, err := resolveSecretKey(ctx, c, wandb.Namespace, sel)
		if err != nil {
			logger.Error(err, "failed to resolve external kafka field", "key", key)
			return []metav1.Condition{{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			}}
		}
		if val != "" {
			data[key] = val
		}
	}

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: kafkaConnectionName}
	return writeExternalConnectionSecret(ctx, c, wandb, nsName, data)
}

func externalKafkaReadState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.KafkaConnection) {
	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: kafkaConnectionName}
	secret := &corev1.Secret{}
	found, err := common.GetResource(ctx, c, nsName, "Secret", secret)
	if err != nil {
		return append(newConditions, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		}), nil
	}
	if !found {
		return newConditions, nil
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return newConditions, &translator.KafkaConnection{
		URL:            corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "url", Optional: ptr.To(false)},
		BrokerEndpoint: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "BrokerEndpoint", Optional: ptr.To(false)},
		Host:           corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port:           corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
	}
}

func externalKafkaInferStatus(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.KafkaConnection,
) (ctrl.Result, error) {
	oldInfraConn := translatorv2.ToTranslatorKafkaConnection(wandb.Status.KafkaStatus.Connection)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	state := common.HealthyState
	ready := true
	if newInfraConn == nil {
		state = common.ErrorState
		ready = false
	}

	updatedConditions := common.ComputeConditionUpdates(
		wandb.Status.KafkaStatus.Conditions,
		newConditions,
		wandb.Generation,
		translator.DefaultConditionExpiry,
	)

	wandb.Status.KafkaStatus = translatorv2.ToWbKafkaInfraStatus(translator.KafkaStatus{
		InfraStatus: translator.InfraStatus{
			Ready:      ready,
			State:      state,
			Conditions: updatedConditions,
		},
		Connection: *conn,
	})
	return ctrl.Result{}, c.Status().Update(ctx, wandb)
}

// helpers

func managedKafkaSpecNamespacedName(spec *apiv2.ManagedKafkaSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
