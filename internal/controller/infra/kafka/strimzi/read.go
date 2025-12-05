package strimzi

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator/common"
	v1beta3 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
) ([]common.KafkaCondition, error) {
	var err error
	var results []common.KafkaCondition
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

	///////////
	// Extract connection info from Kafka CR
	// Connection format: wandb-kafka.{namespace}.svc.cluster.local:9092
	kafkaHost := fmt.Sprintf("%s.%s.svc.cluster.local", nsNameBldr.KafkaName(), nsNameBldr.Namespace())
	kafkaPort := strconv.Itoa(PlainListenerPort)

	connInfo := common.KafkaConnInfo{
		Host: kafkaHost,
		Port: kafkaPort,
	}
	results = append(results, common.NewKafkaConnCondition(connInfo))
	///////////

	return results, nil
}
