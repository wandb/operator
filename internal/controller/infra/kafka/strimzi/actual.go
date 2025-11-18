package strimzi

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/model"
	v1beta3 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type strimziKafka struct {
	kafka    *v1beta3.Kafka
	nodePool *v1beta3.KafkaNodePool
	config   model.KafkaConfig
	client   client.Client
	owner    metav1.Object
	scheme   *runtime.Scheme
}

// Initialize fetches existing Kafka and KafkaNodePool CRs
func Initialize(
	ctx context.Context,
	client client.Client,
	kafkaConfig model.KafkaConfig,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*strimziKafka, error) {
	log := ctrl.LoggerFrom(ctx)

	var kafka = &v1beta3.Kafka{}
	var nodePool = &v1beta3.KafkaNodePool{}
	var result = strimziKafka{
		config:   kafkaConfig,
		client:   client,
		owner:    owner,
		scheme:   scheme,
		kafka:    nil,
		nodePool: nil,
	}

	// Try to get Kafka CR
	kafkaKey := types.NamespacedName{
		Name:      KafkaName,
		Namespace: kafkaConfig.Namespace,
	}
	err := client.Get(ctx, kafkaKey, kafka)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error getting actual Kafka CR")
			return nil, err
		}
	} else {
		result.kafka = kafka
	}

	// Try to get KafkaNodePool CR
	nodePoolKey := types.NamespacedName{
		Name:      NodePoolName,
		Namespace: kafkaConfig.Namespace,
	}
	err = client.Get(ctx, nodePoolKey, nodePool)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error getting actual KafkaNodePool CR")
			return nil, err
		}
	} else {
		result.nodePool = nodePool
	}

	return &result, nil
}

// Upsert creates or updates Kafka and KafkaNodePool CRs based on whether they exist
func (a *strimziKafka) Upsert(ctx context.Context, kafkaConfig model.KafkaConfig) *model.Results {
	results := model.InitResults()
	var nextResults *model.Results

	// Build desired Kafka CR
	desiredKafka, nextResults := buildDesiredKafka(ctx, kafkaConfig, a.owner, a.scheme)
	results.Merge(nextResults)
	if results.HasCriticalError() {
		return results
	}

	// Build desired NodePool CR
	desiredNodePool, nextResults := buildDesiredNodePool(ctx, kafkaConfig, a.owner, a.scheme)
	results.Merge(nextResults)
	if results.HasCriticalError() {
		return results
	}

	// Handle Kafka CR
	if a.kafka == nil {
		nextResults = a.createKafka(ctx, desiredKafka)
		results.Merge(nextResults)
	} else {
		nextResults = a.updateKafka(ctx, desiredKafka)
		results.Merge(nextResults)
	}

	// Handle NodePool CR
	if a.nodePool == nil {
		nextResults = a.createNodePool(ctx, desiredNodePool)
		results.Merge(nextResults)
	} else {
		nextResults = a.updateNodePool(ctx, desiredNodePool)
		results.Merge(nextResults)
	}

	return results
}

// Delete removes Kafka and KafkaNodePool CRs
func (a *strimziKafka) Delete(ctx context.Context) *model.Results {
	log := ctrl.LoggerFrom(ctx)
	results := model.InitResults()

	// Delete NodePool first (should be deleted before Kafka for clean teardown)
	if a.nodePool != nil {
		if err := a.client.Delete(ctx, a.nodePool); err != nil {
			log.Error(err, "Failed to delete KafkaNodePool CR")
			results.AddErrors(model.NewKafkaError(
				model.KafkaErrFailedToDeleteCode,
				fmt.Sprintf("failed to delete KafkaNodePool: %v", err),
			))
			return results
		}
		results.AddStatuses(model.NewKafkaStatusDetail(model.KafkaNodePoolDeletedCode, NodePoolName))
	}

	// Delete Kafka CR
	if a.kafka != nil {
		if err := a.client.Delete(ctx, a.kafka); err != nil {
			log.Error(err, "Failed to delete Kafka CR")
			results.AddErrors(model.NewKafkaError(
				model.KafkaErrFailedToDeleteCode,
				fmt.Sprintf("failed to delete Kafka: %v", err),
			))
			return results
		}
		results.AddStatuses(model.NewKafkaStatusDetail(model.KafkaDeletedCode, KafkaName))
	}

	return results
}
