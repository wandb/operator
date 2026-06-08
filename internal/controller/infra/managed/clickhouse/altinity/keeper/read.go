package keeper

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	chkv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse-keeper.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReadState reports whether the Keeper ensemble is ready (all Keeper pods
// running). ClickHouse depends on Keeper for ReplicatedMergeTree coordination,
// so the returned KeeperReportedReadyType condition gates ClickHouse readiness.
func ReadState(
	ctx context.Context,
	cl client.Client,
	keeperNsName types.NamespacedName,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.ClickHouse)
	var actual = &chkv1.ClickHouseKeeperInstallation{}

	found, err := common.GetResource(ctx, cl, keeperNsName, ResourceTypeName, actual)
	if err != nil {
		return []metav1.Condition{{
			Type:   KeeperReportedReadyType,
			Status: metav1.ConditionUnknown,
			Reason: common.ApiErrorReason,
		}}
	}
	if !found {
		return []metav1.Condition{{
			Type:   KeeperReportedReadyType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		}}
	}

	podsRunning, err := keeperPodsRunningStatus(ctx, cl, keeperNsName.Namespace, actual)
	if err != nil {
		return []metav1.Condition{{
			Type:   KeeperReportedReadyType,
			Status: metav1.ConditionUnknown,
			Reason: common.ApiErrorReason,
		}}
	}

	return computeKeeperReadyCondition(ctx, podsRunning)
}

func keeperPodsRunningStatus(
	ctx context.Context, cl client.Client, namespace string, chk *chkv1.ClickHouseKeeperInstallation,
) (map[string]bool, error) {
	result := make(map[string]bool)
	if chk == nil || chk.Status == nil {
		return result, nil
	}
	for _, podName := range chk.Status.Pods {
		var pod = &corev1.Pod{}
		nsName := types.NamespacedName{Namespace: namespace, Name: podName}
		found, err := common.GetResource(ctx, cl, nsName, "KeeperPod", pod)
		if err != nil {
			return result, err
		}
		result[podName] = found && common.PodReady(pod)
	}
	return result, nil
}

func computeKeeperReadyCondition(ctx context.Context, podsRunning map[string]bool) []metav1.Condition {
	log := logx.GetSlog(ctx)

	var runningCount, podCount int
	for _, isRunning := range podsRunning {
		podCount++
		if isRunning {
			runningCount++
		}
	}
	log.Info("Keeper pods status", "running", runningCount, "total", podCount)

	status := metav1.ConditionUnknown
	reason := common.UnknownReason
	message := ""
	switch {
	case podCount > 0 && podCount == runningCount:
		status = metav1.ConditionTrue
		reason = common.ResourceExistsReason
	case podCount > 0:
		status = metav1.ConditionFalse
		reason = common.NoResourceReason
		message = fmt.Sprintf("%d of %d keeper pods running", runningCount, podCount)
	}

	return []metav1.Condition{{
		Type:    KeeperReportedReadyType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}}
}
