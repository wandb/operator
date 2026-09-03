package reconciler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/managed/kafka/bufstream"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func kafkaWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	mfst manifest.Manifest,
) []metav1.Condition {
	if wandb.Spec.Kafka.ManagedKafka != nil {
		return managedKafkaWriteState(ctx, client, wandb, mfst)
	}
	return nil
}

func kafkaReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.KafkaConnection) {
	if wandb.Spec.Kafka.ManagedKafka != nil {
		return managedKafkaReadState(ctx, client, wandb, newConditions)
	}
	return newConditions, nil
}

func kafkaInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.KafkaConnection,
) (ctrl.Result, error) {
	if wandb.Spec.Kafka.ManagedKafka != nil {
		return managedKafkaInferStatus(ctx, client, recorder, wandb, newConditions, newInfraConn)
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
		onDeleteRule := bufstream.ToKafkaOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
		return bufstream.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
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
	return bufstream.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

// managed

func managedKafkaWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	mfst manifest.Manifest,
) []metav1.Condition {
	spec := wandb.Spec.Kafka.ManagedKafka
	specNamespacedName := managedKafkaSpecNamespacedName(spec)

	if conditions := bufstream.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID(), spec.Replicas); conditions != nil {
		return conditions
	}

	return bufstream.WriteState(ctx, client, wandb, mfst)
}

func managedKafkaReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.KafkaConnection) {
	spec := wandb.Spec.Kafka.ManagedKafka

	specNamespacedName := managedKafkaSpecNamespacedName(spec)
	onDeleteRule := bufstream.ToKafkaOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
	readConditions, newInfraConn := bufstream.ReadState(ctx, client, specNamespacedName, wandb, onDeleteRule)
	newConditions = append(newConditions, readConditions...)
	return newConditions, newInfraConn
}

func managedKafkaInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.KafkaConnection,
) (ctrl.Result, error) {
	statusBefore := wandb.DeepCopy().Status
	oldConditions := wandb.Status.KafkaStatus.Conditions
	oldInfraConn := wandb.Status.KafkaStatus.Connection

	enabled := true
	updatedStatus, events, ctrlResult := bufstream.ComputeStatus(
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
	wandb.Status.KafkaStatus = updatedStatus
	err := updateWandbStatusIfChanged(ctx, client, wandb, statusBefore)

	return ctrlResult, err
}

// helpers

func managedKafkaSpecNamespacedName(spec *apiv2.ManagedKafkaSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}

// createKafkaTopics provisions the manifest-defined topics directly via the Kafka
// Admin API. Bufstream is Kafka-protocol compatible, so topic creation is
// idempotent: an already-existing topic is treated as success.
func createKafkaTopics(ctx context.Context, cl client.Client, wandb *apiv2.WeightsAndBiases, manifest manifest.Manifest) (ctrl.Result, error) {
	if wandb.Spec.Kafka.ManagedKafka == nil {
		return ctrl.Result{}, nil
	}
	log := logx.GetSlog(ctx)

	bootstrap, err := resolveKafkaBootstrap(ctx, cl, wandb)
	if err != nil {
		log.Error("failed to resolve kafka bootstrap endpoint", logx.ErrAttr(err))
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	kafkaSpec := wandb.Spec.Kafka.ManagedKafka
	replicationFactor := int16(1)
	if kafkaSpec.Config.ReplicationConfig.DefaultReplicationFactor > 0 {
		replicationFactor = int16(kafkaSpec.Config.ReplicationConfig.DefaultReplicationFactor)
	}

	adminClient, err := kgo.NewClient(
		kgo.SeedBrokers(bootstrap),
		kgo.ClientID("wandb-operator"),
	)
	if err != nil {
		log.Error("failed to create kafka client", logx.ErrAttr(err))
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}
	defer adminClient.Close()

	admin := kadm.NewClient(adminClient)

	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, topic := range manifest.Kafka.Topics {
		if len(topic.Features) > 0 && !manifest.FeaturesEnabled(topic.Features) {
			continue
		}

		topicName := topic.Topic
		if topicName == "" {
			topicName = topic.Name
		}
		if topicName == "" {
			continue
		}

		partitions, explicitlyOverridden := desiredKafkaTopicPartitionCount(kafkaSpec, topic)

		if err := reconcileKafkaTopic(dialCtx, admin, topicName, partitions, explicitlyOverridden, replicationFactor); err != nil {
			log.Error("failed to reconcile kafka topic", logx.ErrAttr(err), "topic", topicName)
			return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
		}
		log.Debug("ensured kafka topic partition floor", "topic", topicName, "partitions", partitions)
	}

	return ctrl.Result{}, nil
}

type kafkaTopicAdmin interface {
	CreateTopics(context.Context, int32, int16, map[string]*string, ...string) (kadm.CreateTopicResponses, error)
	ListTopics(context.Context, ...string) (kadm.TopicDetails, error)
	UpdatePartitions(context.Context, int, ...string) (kadm.CreatePartitionsResponses, error)
}

func desiredKafkaTopicPartitionCount(kafkaSpec *apiv2.ManagedKafkaSpec, topic manifest.KafkaTopic) (int32, bool) {
	partitions := int32(1)
	if topic.PartitionCount > 0 {
		partitions = int32(topic.PartitionCount)
	}

	topicName := topic.Topic
	if topicName == "" {
		topicName = topic.Name
	}
	if override, ok := kafkaSpec.Config.TopicPartitionOverrides[topicName]; ok {
		return int32(override), true
	}
	return partitions, false
}

func reconcileKafkaTopic(ctx context.Context, admin kafkaTopicAdmin, topicName string, partitions int32, explicitlyOverridden bool, replicationFactor int16) error {
	resp, err := admin.CreateTopics(ctx, partitions, replicationFactor, nil, topicName)
	if err != nil {
		return err
	}
	ct, ok := resp[topicName]
	if !ok {
		return fmt.Errorf("create topic %q: response did not include topic", topicName)
	}
	if ct.Err == nil {
		return nil
	}
	if !errors.Is(ct.Err, kerr.TopicAlreadyExists) {
		return fmt.Errorf("create topic %q: %w", ct.Topic, ct.Err)
	}

	details, err := admin.ListTopics(ctx, topicName)
	if err != nil {
		return fmt.Errorf("describe existing topic %q: %w", topicName, err)
	}
	detail, ok := details[topicName]
	if !ok {
		return fmt.Errorf("describe existing topic %q: response did not include topic", topicName)
	}
	if detail.Err != nil {
		return fmt.Errorf("describe existing topic %q: %w", topicName, detail.Err)
	}

	existingPartitions := int32(len(detail.Partitions))
	switch {
	case existingPartitions == partitions:
		return nil
	case existingPartitions > partitions:
		if !explicitlyOverridden {
			// Manifest counts are creation defaults. Preserve the old idempotent
			// behavior for topics that an administrator has already enlarged.
			return nil
		}
		return fmt.Errorf(
			"topic %q has %d partitions, exceeding the requested count %d; Kafka cannot decrease a topic's partition count; set spec.kafka.managedKafka.config.topicPartitionOverrides[%q] to at least %d or recreate the topic",
			topicName,
			existingPartitions,
			partitions,
			topicName,
			existingPartitions,
		)
	}

	// kadm.UpdatePartitions sends Kafka's CreatePartitions request with an
	// absolute target, avoiding an accidental overshoot if another reconciler
	// increases the topic between the metadata read and this request.
	updateResp, err := admin.UpdatePartitions(ctx, int(partitions), topicName)
	if err != nil {
		return fmt.Errorf("increase topic %q to %d partitions: %w", topicName, partitions, err)
	}
	updated, ok := updateResp[topicName]
	if !ok {
		return fmt.Errorf("increase topic %q to %d partitions: response did not include topic", topicName, partitions)
	}
	if updated.Err != nil {
		return fmt.Errorf("increase topic %q to %d partitions: %w", topicName, partitions, updated.Err)
	}
	return nil
}

// resolveKafkaBootstrap reads the managed Kafka connection secret to obtain the
// in-cluster broker host:port used by the admin client.
func resolveKafkaBootstrap(ctx context.Context, cl client.Client, wandb *apiv2.WeightsAndBiases) (string, error) {
	conn := wandb.Status.KafkaStatus.Connection
	ref := conn.Host.SecretKeyRef()
	if ref == nil || ref.Name == "" {
		return "", fmt.Errorf("kafka connection secret not set in status")
	}
	secretName := ref.Name

	spec := wandb.Spec.Kafka.ManagedKafka
	secret := &corev1.Secret{}
	found, err := common.GetResource(
		ctx, cl,
		types.NamespacedName{Namespace: spec.Namespace, Name: secretName},
		"Secret", secret,
	)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("kafka connection secret %s not found", secretName)
	}

	host := string(secret.Data["Host"])
	port := string(secret.Data["Port"])
	if host == "" || port == "" {
		return "", fmt.Errorf("kafka connection secret missing host/port")
	}
	return fmt.Sprintf("%s:%s", host, port), nil
}
