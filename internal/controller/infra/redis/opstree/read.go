package opstree

import (
	"context"
	"fmt"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) (*translator.RedisStatus, error) {
	var standaloneActual = &redisv1beta2.Redis{}
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}
	var status = &translator.RedisStatus{}
	var err error
	var found bool

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if found, err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.StandaloneNsName(), StandaloneType, standaloneActual,
	); err != nil {
		return nil, err
	}
	if !found {
		standaloneActual = nil
	}
	if found, err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.SentinelNsName(), SentinelType, sentinelActual,
	); err != nil {
		return nil, err
	}
	if !found {
		sentinelActual = nil
	}
	if found, err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.ReplicationNsName(), ReplicationType, replicationActual,
	); err != nil {
		return nil, err
	}
	if !found {
		replicationActual = nil
	}

	if standaloneActual != nil {
		///////////////////////////////////
		// set connection details
		connInfo := readStandaloneConnectionDetails(standaloneActual)

		var connection *translator.InfraConnection
		if connection, err = writeRedisConnInfo(
			ctx, client, wandbOwner, nsNameBldr, connInfo,
		); err != nil {
			return nil, err
		}

		if connection != nil {
			status.Connection = *connection
		}

		///////////////////////////////////
		// add conditions

		///////////////////////////////////
		// set top-level summary

		var runningPods map[string]bool
		if runningPods, err = standalonePodsRunningStatus(ctx, client, standaloneActual); err != nil {
			return nil, err
		}
		computeStandaloneStatusSummary(ctx, runningPods, status)
	} else if sentinelActual != nil && replicationActual != nil {
		connInfo := readSentinelConnectionDetails(sentinelActual)

		var connection *translator.InfraConnection
		if connection, err = writeRedisConnInfo(
			ctx, client, wandbOwner, nsNameBldr, connInfo,
		); err != nil {
			return nil, err
		}

		if connection != nil {
			status.Connection = *connection
		}

		///////////////////////////////////
		// add conditions

		///////////////////////////////////
		// set top-level summary
		var sentinelRunningPods, replicationRunningPods map[string]bool
		if sentinelRunningPods, err = sentinelPodsRunningStatus(ctx, client, sentinelActual); err != nil {
			return nil, err
		}
		if replicationRunningPods, err = replicationPodsRunningStatus(ctx, client, replicationActual); err != nil {
			return nil, err
		}
		computeSentinelStatusSummary(ctx, sentinelRunningPods, replicationRunningPods, status)
	} else {
		status.State = "NotInstalled"
		status.Ready = false
	}

	return status, nil
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

func computeSentinelStatusSummary(
	ctx context.Context, sentinelPodsRunning, replicationPodsRunning map[string]bool, status *translator.RedisStatus,
) {
	log := ctrl.LoggerFrom(ctx)
	var runningCount, podCount int

	// Sentinel calculation
	podCount = 0
	runningCount = 0
	for _, isRunning := range sentinelPodsRunning {
		podCount++
		if isRunning {
			runningCount++
		}
	}
	log.Info(fmt.Sprintf("%d or %d Redis Sentinel Pods are running", runningCount, podCount))

	if podCount == 0 || podCount != runningCount {
		status.State = "NotReady"
		status.Ready = false
		return
	}

	// Replica calculation
	podCount = 0
	runningCount = 0
	for _, isRunning := range replicationPodsRunning {
		podCount++
		if isRunning {
			runningCount++
		}
	}
	log.Info(fmt.Sprintf("%d or %d Redis Replica Pods are running", runningCount, podCount))

	if podCount == 0 || podCount != runningCount {
		status.State = "NotReady"
		status.Ready = false
		return
	}

	status.State = "Ready"
	status.Ready = true
}

func computeStandaloneStatusSummary(
	ctx context.Context, podsRunning map[string]bool, status *translator.RedisStatus,
) {
	log := ctrl.LoggerFrom(ctx)
	var runningCount, podCount int

	for _, isRunning := range podsRunning {
		podCount++
		if isRunning {
			runningCount++
		}
	}
	log.Info(fmt.Sprintf("%d or %d Redis Standalone Pods are running", runningCount, podCount))

	if podCount == 0 || podCount != runningCount {
		status.State = "NotReady"
		status.Ready = false
		return
	}

	status.State = "Ready"
	status.Ready = true
}
