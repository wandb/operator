package strimzi

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
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
) (*translator.KafkaStatus, error) {
	var err error
	var status = &translator.KafkaStatus{}
	var actualKafka = &v1beta3.Kafka{}
	var actualNodePool = &v1beta3.KafkaNodePool{}

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.KafkaNsName(), KafkaResourceType, actualKafka,
	); err != nil {
		return nil, err
	}

	if err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.NodePoolNsName(), NodePoolResourceType, actualNodePool,
	); err != nil {
		return nil, err
	}

	if actualKafka == nil || actualNodePool == nil {
		return nil, nil
	}

	connInfo := readConnectionDetails(nsNameBldr)

	var connection *translator.InfraConnection
	if connection, err = writeKafkaConnInfo(
		ctx, client, wandbOwner, nsNameBldr, connInfo,
	); err != nil {
		return nil, err
	}

	if connection != nil {
		status.Connection = *connection
	}

	return status, nil
}
