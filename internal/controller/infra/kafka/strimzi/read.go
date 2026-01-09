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

func readConnectionDetails(nsnBuilder *NsNameBuilder, actualKafka *strimziv1.Kafka) *kafkaConnInfo {
	kafkaHost := fmt.Sprintf("%s-%s.%s.svc.cluster.local", nsnBuilder.KafkaName(), "kafka-bootstrap", nsnBuilder.Namespace())
	kafkaPort := strconv.Itoa(PlainListenerPort)
	kafkaClusterId := ""
	if actualKafka != nil {
		kafkaClusterId = actualKafka.Status.ClusterId
	}

	return &kafkaConnInfo{
		Host:      kafkaHost,
		Port:      kafkaPort,
		ClusterId: kafkaClusterId,
	}
}

func ReadState(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) ([]metav1.Condition, *translator.InfraConnection) {
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	var actualKafka = &strimziv1.Kafka{}
	found, err := ctrlcommon.GetResource(
		ctx, cl, nsnBuilder.KafkaNsName(), KafkaResourceType, actualKafka,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   KafkaCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: ctrlcommon.ApiErrorReason,
			},
		}, nil
	}
	if !found {
		actualKafka = nil
	}

	var actualNodePool = &strimziv1.KafkaNodePool{}
	if found, err = ctrlcommon.GetResource(
		ctx, cl, nsnBuilder.NodePoolNsName(), NodePoolResourceType, actualNodePool,
	); err != nil {
		return []metav1.Condition{
			{
				Type:   NodePoolCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: ctrlcommon.ApiErrorReason,
			},
		}, nil
	}
	if !found {
		actualNodePool = nil
	}

	conditions := make([]metav1.Condition, 0)
	var connection *translator.InfraConnection
	if actualKafka != nil {

		///////////////////////////////////
		// set connection details

		connInfo := readConnectionDetails(nsnBuilder, actualKafka)

		connection, err = writeKafkaConnInfo(
			ctx, cl, wandbOwner, nsnBuilder, connInfo,
		)
		if err != nil {
			return []metav1.Condition{
				{
					Type:   KafkaConnectionInfoType,
					Status: metav1.ConditionUnknown,
					Reason: ctrlcommon.ApiErrorReason,
				},
			}, nil
		}
		if connection == nil {
			conditions = append(conditions, metav1.Condition{
				Type:   KafkaConnectionInfoType,
				Status: metav1.ConditionFalse,
				Reason: ctrlcommon.NoResourceReason,
			})
		} else {
			conditions = append(conditions, metav1.Condition{
				Type:   KafkaConnectionInfoType,
				Status: metav1.ConditionTrue,
				Reason: ctrlcommon.ResourceExistsReason,
			})
		}

		///////////////////////////////////
		// add conditions

		conditions = append(conditions, computeKafkaReportedReadyCondition(ctx, actualKafka)...)

	}

	return conditions, connection
}

func computeKafkaReportedReadyCondition(_ context.Context, kafkaCR *strimziv1.Kafka) []metav1.Condition {
	if kafkaCR == nil {
		return []metav1.Condition{}
	}
	status := metav1.ConditionUnknown
	reason := ctrlcommon.UnknownReason

	// Check for ready condition (first one wins)
	for _, cond := range kafkaCR.Status.Conditions {
		if strings.EqualFold(cond.Type, "ready") {
			status = cond.Status
			reason = ctrlcommon.ReportedStatusReason
			break
		}
	}

	return []metav1.Condition{
		{
			Type:   KafkaReportedReadyType,
			Status: status,
			Reason: reason,
		},
	}
}
