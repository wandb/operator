package strimzi

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	transcommon "github.com/wandb/operator/internal/controller/translator"
	v1beta3 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readConnectionDetails(nsNameBldr *NsNameBuilder) *kafkaConnInfo {
	kafkaHost := fmt.Sprintf("%s.%s.svc.cluster.local", nsNameBldr.KafkaName(), nsNameBldr.Namespace())
	kafkaPort := strconv.Itoa(PlainListenerPort)

	return &kafkaConnInfo{
		Host: kafkaHost,
		Port: kafkaPort,
	}
}

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) ([]transcommon.KafkaCondition, error) {
	var err error
	var results []transcommon.KafkaCondition
	var actualKafka = &v1beta3.Kafka{}
	var actualNodePool = &v1beta3.KafkaNodePool{}

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.KafkaNsName(), KafkaResourceType, actualKafka,
	); err != nil {
		return results, err
	}

	if err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.NodePoolNsName(), NodePoolResourceType, actualNodePool,
	); err != nil {
		return results, err
	}

	if actualKafka == nil || actualNodePool == nil {
		return results, nil
	}

	connInfo := readConnectionDetails(nsNameBldr)

	var connection *transcommon.KafkaConnection
	if connection, err = writeKafkaConnInfo(
		ctx, client, wandbOwner, nsNameBldr, connInfo,
	); err != nil {
		return results, err
	}

	results = append(results, transcommon.NewKafkaConnCondition(*connection))

	return results, nil
}
