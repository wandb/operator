package opstree

import (
	"context"
	"fmt"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	redisv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redissentinel/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) ([]metav1.Condition, *translator.InfraConnection) {
	ctx, _ = logx.WithSlog(ctx, logx.Redis)
	var standaloneActual = &redisv1beta2.Redis{}
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := ctrlcommon.GetResource(
		ctx, client, nsnBuilder.StandaloneNsName(), StandaloneType, standaloneActual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   RedisStandaloneCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: ctrlcommon.ApiErrorReason,
			},
		}, nil
	}
	if !found {
		standaloneActual = nil
	}

	found, err = ctrlcommon.GetResource(
		ctx, client, nsnBuilder.SentinelNsName(), SentinelType, sentinelActual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   RedisSentinelCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: ctrlcommon.ApiErrorReason,
			},
		}, nil
	}
	if !found {
		sentinelActual = nil
	}

	found, err = ctrlcommon.GetResource(
		ctx, client, nsnBuilder.ReplicationNsName(), ReplicationType, replicationActual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   RedisReplicationCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: ctrlcommon.ApiErrorReason,
			},
		}, nil
	}
	if !found {
		replicationActual = nil
	}

	conditions := make([]metav1.Condition, 0)
	var connection *translator.InfraConnection

	if standaloneActual != nil {
		connInfo := readStandaloneConnectionDetails(standaloneActual)

		connection, err = writeRedisConnInfo(
			ctx, client, wandbOwner, nsnBuilder, connInfo,
		)
		if err != nil {
			return []metav1.Condition{
				{
					Type:   RedisConnectionInfoType,
					Status: metav1.ConditionUnknown,
					Reason: ctrlcommon.ApiErrorReason,
				},
			}, nil
		}
		if connection == nil {
			conditions = append(conditions, metav1.Condition{
				Type:   RedisConnectionInfoType,
				Status: metav1.ConditionFalse,
				Reason: ctrlcommon.NoResourceReason,
			})
		} else {
			conditions = append(conditions, metav1.Condition{
				Type:   RedisConnectionInfoType,
				Status: metav1.ConditionTrue,
				Reason: ctrlcommon.ResourceExistsReason,
			})
		}

		standalonePodsRunning, err := standalonePodsRunningStatus(ctx, client, standaloneActual)
		if err != nil {
			return []metav1.Condition{
				{
					Type:   RedisReportedReadyType,
					Status: metav1.ConditionUnknown,
					Reason: ctrlcommon.ApiErrorReason,
				},
			}, nil
		}
		conditions = append(conditions, computeStandaloneReportedReadyCondition(ctx, standalonePodsRunning)...)

	} else if sentinelActual != nil && replicationActual != nil {
		connInfo := readSentinelConnectionDetails(sentinelActual)

		connection, err = writeRedisConnInfo(
			ctx, client, wandbOwner, nsnBuilder, connInfo,
		)
		if err != nil {
			return []metav1.Condition{
				{
					Type:   RedisConnectionInfoType,
					Status: metav1.ConditionUnknown,
					Reason: ctrlcommon.ApiErrorReason,
				},
			}, nil
		}
		if connection == nil {
			conditions = append(conditions, metav1.Condition{
				Type:   RedisConnectionInfoType,
				Status: metav1.ConditionFalse,
				Reason: ctrlcommon.NoResourceReason,
			})
		} else {
			conditions = append(conditions, metav1.Condition{
				Type:   RedisConnectionInfoType,
				Status: metav1.ConditionTrue,
				Reason: ctrlcommon.ResourceExistsReason,
			})
		}

		sentinelPodsRunning, err := sentinelPodsRunningStatus(ctx, client, sentinelActual)
		if err != nil {
			return []metav1.Condition{
				{
					Type:   RedisReportedReadyType,
					Status: metav1.ConditionUnknown,
					Reason: ctrlcommon.ApiErrorReason,
				},
			}, nil
		}
		replicationPodsRunning, err := replicationPodsRunningStatus(ctx, client, replicationActual)
		if err != nil {
			return []metav1.Condition{
				{
					Type:   RedisReportedReadyType,
					Status: metav1.ConditionUnknown,
					Reason: ctrlcommon.ApiErrorReason,
				},
			}, nil
		}
		conditions = append(conditions, computeSentinelReportedReadyCondition(ctx, sentinelPodsRunning, replicationPodsRunning)...)
	}

	return conditions, connection
}

func computeStandaloneReportedReadyCondition(
	ctx context.Context, podsRunning map[string]bool,
) []metav1.Condition {
	log := logx.GetSlog(ctx)
	var runningCount, podCount int

	for _, isRunning := range podsRunning {
		podCount++
		if isRunning {
			runningCount++
		}
	}
	log.Info("Redis Standalone pods status", "running", runningCount, "total", podCount)

	status := metav1.ConditionUnknown
	reason := ctrlcommon.UnknownReason
	message := ""

	if podCount > 0 && podCount == runningCount {
		status = metav1.ConditionTrue
		reason = ctrlcommon.ResourceExistsReason
	} else if podCount > 0 {
		status = metav1.ConditionFalse
		reason = ctrlcommon.ResourceExistsReason
		log.Info("Redis Standalone pods not all running", "running", runningCount, "total", podCount)
	}

	return []metav1.Condition{
		{
			Type:    RedisReportedReadyType,
			Status:  status,
			Reason:  reason,
			Message: message,
		},
	}
}

func computeSentinelReportedReadyCondition(
	ctx context.Context, sentinelPodsRunning, replicationPodsRunning map[string]bool,
) []metav1.Condition {
	log := logx.GetSlog(ctx)

	var sentinelRunningCount, sentinelPodCount int
	for _, isRunning := range sentinelPodsRunning {
		sentinelPodCount++
		if isRunning {
			sentinelRunningCount++
		}
	}
	log.Info("Redis Sentinel pods status", "running", sentinelRunningCount, "total", sentinelPodCount)

	var replicationRunningCount, replicationPodCount int
	for _, isRunning := range replicationPodsRunning {
		replicationPodCount++
		if isRunning {
			replicationRunningCount++
		}
	}
	log.Info("Redis Replication pods status", "running", replicationRunningCount, "total", replicationPodCount)

	status := metav1.ConditionUnknown
	reason := ctrlcommon.UnknownReason
	message := ""

	allPodsRunning := sentinelPodCount > 0 && sentinelPodCount == sentinelRunningCount &&
		replicationPodCount > 0 && replicationPodCount == replicationRunningCount
	atLeastOneEach := sentinelRunningCount > 0 && replicationRunningCount > 0
	eitherZero := sentinelRunningCount == 0 || replicationRunningCount == 0

	if allPodsRunning {
		status = metav1.ConditionTrue
		reason = ctrlcommon.ResourceExistsReason
	} else if eitherZero {
		status = metav1.ConditionFalse
		reason = ctrlcommon.ResourceExistsReason
		message = fmt.Sprintf("sentinel: %d/%d running, replication: %d/%d running", sentinelRunningCount, sentinelPodCount, replicationRunningCount, replicationPodCount)
	} else if atLeastOneEach {
		status = metav1.ConditionFalse
		reason = "degraded"
		message = fmt.Sprintf("sentinel: %d/%d running, replication: %d/%d running", sentinelRunningCount, sentinelPodCount, replicationRunningCount, replicationPodCount)
	}

	return []metav1.Condition{
		{
			Type:    RedisReportedReadyType,
			Status:  status,
			Reason:  reason,
			Message: message,
		},
	}
}

func sentinelPodsRunningStatus(
	ctx context.Context, client client.Client, sentinel *redissentinelv1beta2.RedisSentinel,
) (
	map[string]bool, error,
) {
	var result = make(map[string]bool)
	var found bool
	var err error

	if sentinel == nil {
		return result, nil
	}

	podCount := sentinel.Spec.Size
	if podCount == nil || *podCount == 0 {
		return result, nil
	}

	namespace := sentinel.Namespace
	specName := sentinel.Name

	for i := 0; i < int(*podCount); i++ {
		podName := fmt.Sprintf("%s-sentinel-%d", specName, i)
		var pod = &corev1.Pod{}
		nsName := types.NamespacedName{Namespace: namespace, Name: podName}
		if found, err = ctrlcommon.GetResource(
			ctx, client, nsName, "RedisSentinelPod", pod,
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

func replicationPodsRunningStatus(
	ctx context.Context, client client.Client, replication *redisreplicationv1beta2.RedisReplication,
) (
	map[string]bool, error,
) {
	var result = make(map[string]bool)
	var found bool
	var err error

	if replication == nil {
		return result, nil
	}

	podCount := replication.Spec.Size
	if podCount == nil || *podCount == 0 {
		return result, nil
	}

	namespace := replication.Namespace
	specName := replication.Name

	for i := 0; i < int(*podCount); i++ {
		podName := fmt.Sprintf("%s-%d", specName, i)
		var pod = &corev1.Pod{}
		nsName := types.NamespacedName{Namespace: namespace, Name: podName}
		if found, err = ctrlcommon.GetResource(
			ctx, client, nsName, "RedisReplicaPod", pod,
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

func standalonePodsRunningStatus(
	ctx context.Context, client client.Client, standalone *redisv1beta2.Redis,
) (
	map[string]bool, error,
) {
	var result = make(map[string]bool)
	var found bool
	var err error

	if standalone == nil {
		return result, nil
	}

	podCount := 1

	namespace := standalone.Namespace
	specName := standalone.Name

	for i := 0; i < podCount; i++ {
		podName := fmt.Sprintf("%s-%d", specName, i)
		var pod = &corev1.Pod{}
		nsName := types.NamespacedName{Namespace: namespace, Name: podName}
		if found, err = ctrlcommon.GetResource(
			ctx, client, nsName, "RedisPod", pod,
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
