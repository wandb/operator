package altinity

import (
	"context"
	"fmt"
	"strconv"

	log "github.com/altinity/clickhouse-operator/pkg/announcer"
	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	chiv1 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readConnectionDetails(specNamespacedName types.NamespacedName) *clickhouseConnInfo {
	clickhouseHost := fmt.Sprintf("%s.%s.svc.cluster.local", ServiceName, specNamespacedName.Namespace)
	clickhousePort := strconv.Itoa(ClickHouseNativePort)

	return &clickhouseConnInfo{
		Host: clickhouseHost,
		Port: clickhousePort,
		User: ClickHouseUser,
	}
}

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) (*translator.ClickHouseStatus, error) {
	var err error
	var found bool
	var status = &translator.ClickHouseStatus{}
	var actual = &chiv1.ClickHouseInstallation{}
	var podsRunning map[string]bool

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if found, err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.InstallationNsName(), ResourceTypeName, actual,
	); err != nil {
		return nil, err
	}
	if !found {
		actual = nil
	}

	if actual != nil {
		if podsRunning, err = chPodsRunningStatus(ctx, client, nsNameBldr.Namespace(), actual); err != nil {
			return nil, err
		}
		///////////////////////////////////
		// set connection details

		connInfo := readConnectionDetails(specNamespacedName)

		var connection *translator.InfraConnection
		if connection, err = writeClickHouseConnInfo(
			ctx, client, wandbOwner, nsNameBldr, connInfo,
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
	computeStatusSummary(ctx, actual, podsRunning, status)

	return status, nil
}

func chPodsRunningStatus(
	ctx context.Context, client client.Client, namespace string, chi *chiv1.ClickHouseInstallation,
) (
	map[string]bool, error,
) {
	var result = make(map[string]bool)
	var found bool
	var err error

	if chi == nil {
		return result, nil
	}
	for _, podName := range chi.Status.Pods {
		var pod = &corev1.Pod{}
		nsName := types.NamespacedName{Namespace: namespace, Name: podName}
		if found, err = ctrlcommon.GetResource(
			ctx, client, nsName, "ClickhousePod", pod,
		); err != nil {
			return result, err
		}
		if found {
			result[podName] = pod.Status.Phase == corev1.PodRunning
		} else {
			result[podName] = false
		}
	}
	return result, nil
}

func computeStatusSummary(
	ctx context.Context, chiCR *chiv1.ClickHouseInstallation, podsRunning map[string]bool, status *translator.ClickHouseStatus,
) {
	var runningCount, podCount int
	for _, isRunning := range podsRunning {
		podCount++
		if isRunning {
			runningCount++
		}
	}

	log.Info(fmt.Sprintf("%d or %d Clickhouse Pods are running", runningCount, podCount))

	if podCount > 0 && podCount == runningCount {
		status.State = "Ready"
		status.Ready = true
	} else {
		status.State = "NotReady"
		status.Ready = false
	}
}
