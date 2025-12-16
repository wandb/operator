package strimzi

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	strimziv1 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readConnectionDetails(nsNameBldr *NsNameBuilder) *kafkaConnInfo {
	kafkaHost := fmt.Sprintf("%s-%s.%s.svc.cluster.local", nsNameBldr.KafkaName(), "kafka-bootstrap", nsNameBldr.Namespace())
	kafkaPort := strconv.Itoa(PlainListenerPort)

	return &kafkaConnInfo{
		Host: kafkaHost,
		Port: kafkaPort,
	}
}

func ReadState(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) (*translator.KafkaStatus, error) {
	var err error
	var found bool
	var status = &translator.KafkaStatus{}
	var actualKafka = &strimziv1.Kafka{}
	var actualNodePool = &strimziv1.KafkaNodePool{}

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if found, err = ctrlcommon.GetResource(
		ctx, cl, nsNameBldr.KafkaNsName(), KafkaResourceType, actualKafka,
	); err != nil {
		return nil, err
	}
	if !found {
		actualKafka = nil
	}

	if found, err = ctrlcommon.GetResource(
		ctx, cl, nsNameBldr.NodePoolNsName(), NodePoolResourceType, actualNodePool,
	); err != nil {
		return nil, err
	}
	if !found {
		actualNodePool = nil
	}

	if actualKafka != nil {

		///////////////////////////////////
		// set connection details

		connInfo := readConnectionDetails(nsNameBldr)

		var connection *translator.InfraConnection
		if connection, err = writeKafkaConnInfo(
			ctx, cl, wandbOwner, nsNameBldr, connInfo,
		); err != nil {
			return nil, err
		}

		if connection != nil {
			status.Connection = *connection
		}

		///////////////////////////////////
		// add conditions

	}

	///////////////////////////////////
	// set top-level summary
	computeStatusSummary(ctx, actualKafka, status)

	return status, nil
}

func computeStatusSummary(ctx context.Context, kafkaCR *strimziv1.Kafka, status *translator.KafkaStatus) {
	if kafkaCR == nil {
		status.State = "Not Installed"
		status.Ready = false
	} else {
		// Check for ready condition (first one wins)
		for _, cond := range kafkaCR.Status.Conditions {
			if strings.EqualFold(cond.Type, "ready") {
				status.Ready = cond.Status == metav1.ConditionTrue
				if cond.Status == metav1.ConditionTrue && cond.Reason != "" {
					status.State = cond.Reason
				}
				break
			}
		}

		// Use KafkaMetadataState if available and state not set from condition
		if status.State == "" && kafkaCR.Status.KafkaMetadataState != "" {
			status.State = kafkaCR.Status.KafkaMetadataState
		}

		// Set default state if still empty
		if status.State == "" {
			if status.Ready {
				status.State = "Ready"
			} else {
				status.State = "Not Ready"
			}
		}
	}
}
