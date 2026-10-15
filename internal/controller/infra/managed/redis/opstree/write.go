package opstree

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	redisv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redissentinel/v1beta2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	StandaloneType  = "RedisStandalone"
	SentinelType    = "RedisSentinel"
	ReplicationType = "RedisReplication"
	AppConnTypeName = "RedisAppConn"
)

func WriteState(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	standaloneDesired *redisv1beta2.Redis,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
	replicationDesired *redisreplicationv1beta2.RedisReplication,
	wandbLabels map[string]string,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.Redis)
	results := make([]metav1.Condition, 0)

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	results = append(results, writeStandaloneState(ctx, cl, nsnBuilder, standaloneDesired)...)
	results = append(results, writeSentinelState(ctx, cl, nsnBuilder, sentinelDesired)...)
	results = append(results, writeReplicationState(ctx, cl, nsnBuilder, replicationDesired)...)

	if len(wandbLabels) > 0 {
		var statefulSetNames []string
		var podPrefixes []string
		if standaloneDesired != nil {
			statefulSetNames = append(statefulSetNames, nsnBuilder.StandaloneName())
			podPrefixes = append(podPrefixes, fmt.Sprintf("%s-", nsnBuilder.StandaloneName()))
		}
		if replicationDesired != nil {
			statefulSetNames = append(statefulSetNames, nsnBuilder.ReplicationName())
			podPrefixes = append(podPrefixes, fmt.Sprintf("%s-", nsnBuilder.ReplicationName()))
		}
		if err := ensurePVCLabels(ctx, cl, specNamespacedName.Namespace, statefulSetNames, wandbLabels); err != nil {
			results = append(results, metav1.Condition{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			})
		}
		if err := ensurePodLabels(ctx, cl, specNamespacedName.Namespace, podPrefixes, wandbLabels); err != nil {
			results = append(results, metav1.Condition{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			})
		}
	}

	return results
}

// ensurePVCLabels derives PVC names from the actual StatefulSet templates.
// Upstream defaults the template name to the CR name and can also configure it;
// assuming a fixed template name leaves those PVCs without ownership labels.
func ensurePVCLabels(
	ctx context.Context,
	cl client.Client,
	namespace string,
	statefulSetNames []string,
	labels map[string]string,
) error {
	log := logx.GetSlog(ctx)
	namePrefixes, err := redisClaimPrefixes(ctx, cl, namespace, statefulSetNames)
	if err != nil {
		return err
	}
	if len(namePrefixes) == 0 {
		return nil
	}

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(ctx, pvcList, &client.ListOptions{Namespace: namespace}); err != nil {
		return err
	}

	for _, pvc := range pvcList.Items {
		if !matchesPVCName(pvc.Name, namePrefixes) {
			continue
		}
		if common.HasAllLabelKeys(pvc.Labels, labels) {
			continue
		}
		patch := client.MergeFrom(pvc.DeepCopy())
		if pvc.Labels == nil {
			pvc.Labels = make(map[string]string)
		}
		maps.Copy(pvc.Labels, labels)
		if err := cl.Patch(ctx, &pvc, patch); err != nil {
			log.Error("failed to patch PVC labels", logx.ErrAttr(err), "pvc", pvc.Name)
			return err
		}
		log.Debug("patched wandb labels onto PVC", "pvc", pvc.Name)
	}
	return nil
}

// redisClaimPrefixes uses actual templates so custom claim names are preserved.
func redisClaimPrefixes(ctx context.Context, cl client.Client, namespace string, statefulSetNames []string) ([]string, error) {
	var namePrefixes []string
	for _, name := range statefulSetNames {
		sts := &appsv1.StatefulSet{}
		if err := cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, sts); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		for _, template := range sts.Spec.VolumeClaimTemplates {
			namePrefixes = append(namePrefixes, fmt.Sprintf("%s-%s-", template.Name, sts.Name))
		}
	}
	return namePrefixes, nil
}

func matchesPVCName(name string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if ordinal, ok := strings.CutPrefix(name, prefix); ok {
			n, err := strconv.Atoi(ordinal)
			if err == nil && n >= 0 && strconv.Itoa(n) == ordinal {
				return true
			}
		}
	}
	return false
}

func ensurePodLabels(
	ctx context.Context,
	cl client.Client,
	namespace string,
	namePrefixes []string,
	labels map[string]string,
) error {
	log := logx.GetSlog(ctx)

	podList := &corev1.PodList{}
	if err := cl.List(ctx, podList, &client.ListOptions{Namespace: namespace}); err != nil {
		return err
	}

	for _, pod := range podList.Items {
		if !matchesAnyPrefix(pod.Name, namePrefixes) {
			continue
		}
		if common.HasAllLabelKeys(pod.Labels, labels) {
			continue
		}
		patch := client.MergeFrom(pod.DeepCopy())
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		maps.Copy(pod.Labels, labels)
		if err := cl.Patch(ctx, &pod, patch); err != nil {
			log.Error("failed to patch Pod labels", logx.ErrAttr(err), "pod", pod.Name)
			return err
		}
		log.Debug("patched wandb labels onto Pod", "pod", pod.Name)
	}
	return nil
}

func matchesAnyPrefix(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func writeStandaloneState(
	ctx context.Context,
	cl client.Client,
	nsnBuilder *NsNameBuilder,
	standaloneDesired *redisv1beta2.Redis,
) []metav1.Condition {
	var standaloneActual = &redisv1beta2.Redis{}

	found, err := common.GetResource(
		ctx, cl, nsnBuilder.StandaloneNsName(), StandaloneType, standaloneActual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   RedisStandaloneCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}
	}
	if !found {
		standaloneActual = nil
	}

	result := make([]metav1.Condition, 0)

	action, err := common.CrudResource(ctx, cl, standaloneDesired, standaloneActual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	}

	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   RedisStandaloneCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   RedisStandaloneCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction, common.UnchangedAction:
		result = append(result, metav1.Condition{
			Type:   RedisStandaloneCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   RedisStandaloneCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return result
}

func writeSentinelState(
	ctx context.Context,
	cl client.Client,
	nsnBuilder *NsNameBuilder,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
) []metav1.Condition {
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}

	found, err := common.GetResource(
		ctx, cl, nsnBuilder.SentinelNsName(), SentinelType, sentinelActual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   RedisSentinelCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}
	}
	if !found {
		sentinelActual = nil
	}

	result := make([]metav1.Condition, 0)

	action, err := common.CrudResource(ctx, cl, sentinelDesired, sentinelActual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	}

	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   RedisSentinelCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   RedisSentinelCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction, common.UnchangedAction:
		result = append(result, metav1.Condition{
			Type:   RedisSentinelCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   RedisSentinelCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return result
}

func writeReplicationState(
	ctx context.Context,
	cl client.Client,
	nsnBuilder *NsNameBuilder,
	replicationDesired *redisreplicationv1beta2.RedisReplication,
) []metav1.Condition {
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}

	found, err := common.GetResource(
		ctx, cl, nsnBuilder.ReplicationNsName(), ReplicationType, replicationActual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   RedisReplicationCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}
	}
	if !found {
		replicationActual = nil
	}

	result := make([]metav1.Condition, 0)

	action, err := common.CrudResource(ctx, cl, replicationDesired, replicationActual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	}

	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   RedisReplicationCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   RedisReplicationCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction, common.UnchangedAction:
		result = append(result, metav1.Condition{
			Type:   RedisReplicationCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   RedisReplicationCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return result
}

type redisConnInfo struct {
	Host           string
	Port           string
	SentinelHost   string
	SentinelPort   string
	SentinelMaster string
}

func (c *redisConnInfo) toURL() string {
	if c.SentinelHost != "" {
		return fmt.Sprintf("redis://%s:%s?master=%s", c.SentinelHost, c.SentinelPort, c.SentinelMaster)
	}
	return fmt.Sprintf("redis://%s:%s", c.Host, c.Port)
}
