package strimzi

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	v1beta3 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) (*translator.KafkaStatus, error) {
	var err error
	var found bool
	var status = &translator.KafkaStatus{}
	var actualKafka = &v1beta3.Kafka{}
	var actualNodePool = &v1beta3.KafkaNodePool{}

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

func computeStatusSummary(ctx context.Context, kafkaCR *v1beta3.Kafka, status *translator.KafkaStatus) {
	if kafkaCR == nil {
		status.State = "NotInstalled"
		status.Ready = false
	} else {
		for _, cond := range kafkaCR.Status.Conditions {
			if cond.Status == metav1.ConditionTrue {
				status.Ready = strings.EqualFold(cond.Type, "ready")
				status.State = cond.Reason
			}
		}
		if status.State == "" {
			if status.Ready {
				status.State = "Ready"
			} else {
				status.State = "NotReady"
			}
		}
	}
}
