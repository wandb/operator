package strimzi

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	v1 "github.com/wandb/operator/pkg/vendored/strimzi-kafka/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CheckDetached(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbUID types.UID,
	desiredReplicas int32,
) []metav1.Condition {
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	actual := &v1.KafkaNodePool{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.NodePoolNsName(), NodePoolResourceType, actual)
	if err != nil || !found {
		return nil
	}
	if !common.IsDetached(actual, wandbUID) {
		return nil
	}

	if desiredReplicas > 0 && actual.Spec.Replicas != desiredReplicas {
		return []metav1.Condition{
			{
				Type:    common.ReconciledType,
				Status:  metav1.ConditionFalse,
				Reason:  common.DetachedSpecMismatch,
				Message: fmt.Sprintf("detached Kafka CR spec mismatch: replicas want %d, have %d", desiredReplicas, actual.Spec.Replicas),
			},
		}
	}
	return nil
}

func DetachFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, log := logx.WithSlog(ctx, logx.Kafka)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	if err := detachKafka(ctx, cl, log, nsnBuilder, wandbOwner); err != nil {
		return err
	}
	if err := detachNodePool(ctx, cl, log, nsnBuilder, wandbOwner); err != nil {
		return err
	}
	if err := detachTopics(ctx, cl, log, nsnBuilder, wandbOwner); err != nil {
		return err
	}
	return detachConnectionSecret(ctx, cl, log, nsnBuilder, wandbOwner)
}

func detachKafka(
	ctx context.Context,
	cl client.Client,
	log *slog.Logger,
	nsnBuilder *NsNameBuilder,
	wandbOwner client.Object,
) error {
	var actual = &v1.Kafka{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.KafkaNsName(), KafkaResourceType, actual)
	if err != nil {
		return err
	}
	if !found {
		log.Info("abort detach: Kafka CR not found")
		return nil
	}
	if common.IsDetached(actual, wandbOwner.GetUID()) {
		log.Debug("Kafka CR already detached")
		return nil
	}

	common.RemoveOwnerReference(actual, wandbOwner.GetUID())
	if err = cl.Update(ctx, actual); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error("error detaching Kafka CR", logx.ErrAttr(err))
		return err
	}
	log.Info("detached Kafka CR", "name", actual.Name)
	return nil
}

func detachNodePool(
	ctx context.Context,
	cl client.Client,
	log *slog.Logger,
	nsnBuilder *NsNameBuilder,
	wandbOwner client.Object,
) error {
	var actual = &v1.KafkaNodePool{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.NodePoolNsName(), NodePoolResourceType, actual)
	if err != nil {
		return err
	}
	if !found {
		log.Info("abort detach: KafkaNodePool CR not found")
		return nil
	}
	if common.IsDetached(actual, wandbOwner.GetUID()) {
		log.Debug("KafkaNodePool CR already detached")
		return nil
	}

	common.RemoveOwnerReference(actual, wandbOwner.GetUID())
	if err = cl.Update(ctx, actual); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error("error detaching KafkaNodePool CR", logx.ErrAttr(err))
		return err
	}
	log.Info("detached KafkaNodePool CR", "name", actual.Name)
	return nil
}

func detachTopics(
	ctx context.Context,
	cl client.Client,
	log *slog.Logger,
	nsnBuilder *NsNameBuilder,
	wandbOwner client.Object,
) error {
	topicList := &v1.KafkaTopicList{}
	listOpts := &client.ListOptions{
		Namespace: nsnBuilder.Namespace(),
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"app.kubernetes.io/managed-by": "wandb-operator",
			"app.kubernetes.io/instance":   wandbOwner.GetName(),
		}),
	}
	if err := cl.List(ctx, topicList, listOpts); err != nil {
		return err
	}
	for i := range topicList.Items {
		topic := &topicList.Items[i]
		if common.IsDetached(topic, wandbOwner.GetUID()) {
			continue
		}
		common.RemoveOwnerReference(topic, wandbOwner.GetUID())
		if err := cl.Update(ctx, topic); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			log.Error("error detaching KafkaTopic CR", logx.ErrAttr(err))
			return err
		}
		log.Info("detached KafkaTopic CR", "name", topic.Name)
	}
	return nil
}

func detachConnectionSecret(ctx context.Context, cl client.Client, log *slog.Logger, nsnBuilder *NsNameBuilder, wandbOwner client.Object) error {
	secret := &corev1.Secret{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.ConnectionNsName(), "Secret", secret)
	if err != nil || !found {
		return err
	}
	if common.IsDetached(secret, wandbOwner.GetUID()) {
		return nil
	}
	common.RemoveOwnerReference(secret, wandbOwner.GetUID())
	if err = cl.Update(ctx, secret); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error("error detaching connection secret", logx.ErrAttr(err))
		return err
	}
	log.Info("detached connection secret", "name", nsnBuilder.ConnectionNsName().Name)
	return nil
}
