package reconciler

import (
	"context"
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

		partitions := int32(1)
		if topic.PartitionCount > 0 {
			partitions = int32(topic.PartitionCount)
		}

		if err := createTopicIdempotent(dialCtx, admin, topicName, partitions, replicationFactor); err != nil {
			log.Error("failed to create kafka topic", logx.ErrAttr(err), "topic", topicName)
			return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
		}
		log.Debug("ensured kafka topic", "topic", topicName, "partitions", partitions)
	}

	return ctrl.Result{}, nil
}

func createTopicIdempotent(ctx context.Context, admin *kadm.Client, topicName string, partitions int32, replicationFactor int16) error {
	resp, err := admin.CreateTopics(ctx, partitions, replicationFactor, nil, topicName)
	if err != nil {
		return err
	}
	for _, ct := range resp {
		if ct.Err != nil && ct.Err != kerr.TopicAlreadyExists {
			return fmt.Errorf("create topic %q: %w", ct.Topic, ct.Err)
		}
	}
	return nil
}

// resolveKafkaBootstrap reads the managed Kafka connection secret to obtain the
// in-cluster broker host:port used by the admin client.
func resolveKafkaBootstrap(ctx context.Context, cl client.Client, wandb *apiv2.WeightsAndBiases) (string, error) {
	conn := wandb.Status.KafkaStatus.Connection
	secretName := conn.Host.Name
	if secretName == "" {
		return "", fmt.Errorf("kafka connection secret not set in status")
	}

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
