package strimzi

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/vendored/strimzi-kafka/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readConnectionDetails(nsnBuilder *NsNameBuilder, actualKafka *v1.Kafka) *kafkaConnInfo {
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
	k8sClient client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
	onDeleteRule ctrlcommon.OnDeleteRule,
) ([]metav1.Condition, *apiv2.KafkaConnection) {
	ctx, log := logx.WithSlog(ctx, logx.Kafka)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	var actualKafka = &v1.Kafka{}
	found, err := ctrlcommon.GetResource(
		ctx, k8sClient, nsnBuilder.KafkaNsName(), KafkaResourceType, actualKafka,
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

	conditions := make([]metav1.Condition, 0)

	if !found {
		log.Info("Kafka CR not found")
		actualKafka = nil
		if onDeleteRule.Policy == ctrlcommon.Purge {
			log.Debug(
				"Attempting to purge associated kafka resources after deletion",
				"kafkaName", nsnBuilder.KafkaName(),
			)
			if err := purgeAssociatedResources(ctx, k8sClient, specNamespacedName.Namespace, onDeleteRule.Selector); err != nil {
				conditions = append(conditions, metav1.Condition{
					Type:   KafkaCustomResourceType,
					Status: metav1.ConditionUnknown,
					Reason: ctrlcommon.ApiErrorReason,
				})
			} else {
				conditions = append(conditions, metav1.Condition{
					Type:   KafkaCustomResourceType,
					Status: metav1.ConditionFalse,
					Reason: ctrlcommon.PendingDeleteReason,
				})
			}
		}
	}

	var actualNodePool = &v1.KafkaNodePool{}
	if found, err = ctrlcommon.GetResource(
		ctx, k8sClient, nsnBuilder.NodePoolNsName(), NodePoolResourceType, actualNodePool,
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
		log.Info("Kafka NodePool CR not found")
		// actualNodePool is not used elsewhere, so we don't need to set it to nil
	}

	var connection *apiv2.KafkaConnection
	if actualKafka != nil {

		///////////////////////////////////
		// set connection details

		connInfo := readConnectionDetails(nsnBuilder, actualKafka)

		connection, err = writeKafkaConnInfo(
			ctx, k8sClient, wandbOwner, nsnBuilder, connInfo,
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

func computeKafkaReportedReadyCondition(_ context.Context, kafkaCR *v1.Kafka) []metav1.Condition {
	if kafkaCR == nil {
		return []metav1.Condition{}
	}
	status := metav1.ConditionUnknown
	reason := ctrlcommon.UnknownReason

	// Check for Ready and NotReady conditions (first one wins)
	for _, cond := range kafkaCR.Status.Conditions {
		if cond.Status != metav1.ConditionUnknown {
			if strings.EqualFold(cond.Type, "Ready") {
				status = cond.Status
				reason = ctrlcommon.ReportedStatusReason
				break
			}
			if strings.EqualFold(cond.Type, "NotReady") {
				if cond.Status == metav1.ConditionTrue {
					status = metav1.ConditionFalse
				} else {
					status = metav1.ConditionTrue
				}
				reason = ctrlcommon.ReportedStatusReason
				break
			}
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
