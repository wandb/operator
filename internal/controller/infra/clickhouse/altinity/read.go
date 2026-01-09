package altinity

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	chiv1 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readConnectionDetails(actual *chiv1.ClickHouseInstallation) *clickhouseConnInfo {
	clickhouseHost := actual.Status.Endpoint
	clickhousePort := strconv.Itoa(ClickHouseHTTPPort)

	return &clickhouseConnInfo{
		Host:     clickhouseHost,
		Port:     clickhousePort,
		User:     ClickHouseUser,
		Password: ClickHousePassword,
		Database: ClickHouseDatabase,
	}
}

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) ([]metav1.Condition, *translator.InfraConnection) {
	var actual = &chiv1.ClickHouseInstallation{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := ctrlcommon.GetResource(
		ctx, client, nsnBuilder.InstallationNsName(), ResourceTypeName, actual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   ClickHouseCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: ctrlcommon.ApiErrorReason,
			},
		}, nil
	}
	if !found {
		actual = nil
	}

	conditions := make([]metav1.Condition, 0)
	var connection *translator.InfraConnection

	if actual != nil {
		podsRunning, err := chPodsRunningStatus(ctx, client, nsnBuilder.Namespace(), actual)
		if err != nil {
			return []metav1.Condition{
				{
					Type:   ClickHouseReportedReadyType,
					Status: metav1.ConditionUnknown,
					Reason: ctrlcommon.ApiErrorReason,
				},
			}, nil
		}

		connInfo := readConnectionDetails(actual)

		connection, err = writeClickHouseConnInfo(
			ctx, client, wandbOwner, nsnBuilder, connInfo,
		)
		if err != nil {
			return []metav1.Condition{
				{
					Type:   ClickHouseConnectionInfoType,
					Status: metav1.ConditionFalse,
					Reason: ctrlcommon.ApiErrorReason,
				},
			}, nil
		}
		if connection == nil {
			conditions = append(conditions, metav1.Condition{
				Type:   ClickHouseConnectionInfoType,
				Status: metav1.ConditionFalse,
				Reason: ctrlcommon.NoResourceReason,
			})
		} else {
			conditions = append(conditions, metav1.Condition{
				Type:   ClickHouseConnectionInfoType,
				Status: metav1.ConditionTrue,
				Reason: ctrlcommon.ResourceExistsReason,
			})
		}

		conditions = append(conditions, computeClickHouseReportedReadyCondition(ctx, actual, podsRunning)...)
	}

	return conditions, connection
}

func chPodsRunningStatus(
	ctx context.Context, client client.Client, namespace string, chi *chiv1.ClickHouseInstallation,
) (
	map[string]bool, error,
) {
	var result = make(map[string]bool)

	if chi == nil {
		return result, nil
	}
	if chi.Status != nil && chi.Status.Pods != nil {
		for _, podName := range chi.Status.Pods {
			var pod = &corev1.Pod{}
			nsName := types.NamespacedName{Namespace: namespace, Name: podName}
			found, err := ctrlcommon.GetResource(
				ctx, client, nsName, "ClickhousePod", pod,
			)
			if err != nil {
				return result, err
			}
			if found {
				result[podName] = pod.Status.Phase == corev1.PodRunning
			} else {
				result[podName] = false
			}
		}
	}
	return result, nil
}

func computeClickHouseReportedReadyCondition(
	ctx context.Context, chi *chiv1.ClickHouseInstallation, podsRunning map[string]bool,
) []metav1.Condition {
	log := ctrl.LoggerFrom(ctx)

	if chi == nil {
		return []metav1.Condition{}
	}

	var runningCount, podCount int
	for _, isRunning := range podsRunning {
		podCount++
		if isRunning {
			runningCount++
		}
	}

	log.Info(fmt.Sprintf("%d of %d ClickHouse Pods are running", runningCount, podCount))

	status := metav1.ConditionUnknown
	reason := ctrlcommon.UnknownReason
	message := ""

	if podCount > 0 && podCount == runningCount {
		status = metav1.ConditionTrue
		reason = ctrlcommon.ResourceExistsReason
	} else if podCount > 0 {
		status = metav1.ConditionFalse
		reason = ctrlcommon.NoResourceReason
		message = fmt.Sprintf("%d of %d pods running", runningCount, podCount)
	}

	return []metav1.Condition{
		{
			Type:    ClickHouseReportedReadyType,
			Status:  status,
			Reason:  reason,
			Message: message,
		},
	}
}
