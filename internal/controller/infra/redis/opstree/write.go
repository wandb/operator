package opstree

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/wandb/operator/internal/controller/common"
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

const (
	StandaloneType  = "RedisStandalone"
	SentinelType    = "RedisSentinel"
	ReplicationType = "RedisReplication"
	AppConnTypeName = "RedisAppConn"

	// pvcTemplatePrefix is the volumeClaimTemplate name the opstree operator
	// uses when creating StatefulSets, resulting in PVCs named
	// "redis-data-{crName}-{ordinal}".
	pvcTemplatePrefix = "redis-data"
)

func WriteState(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	standaloneDesired *redisv1beta2.Redis,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
	replicationDesired *redisreplicationv1beta2.RedisReplication,
	wandbOwner client.Object,
	onDeleteRule translator.OnDeleteRule,
	wandbLabels map[string]string,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.Redis)
	results := make([]metav1.Condition, 0)

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	results = append(results, writeStandaloneState(ctx, cl, specNamespacedName, nsnBuilder, standaloneDesired, wandbOwner, onDeleteRule)...)
	results = append(results, writeSentinelState(ctx, cl, specNamespacedName, nsnBuilder, sentinelDesired, wandbOwner, onDeleteRule)...)
	results = append(results, writeReplicationState(ctx, cl, nsnBuilder, replicationDesired)...)

	if len(wandbLabels) > 0 {
		var pvcPrefixes []string
		var podPrefixes []string
		if standaloneDesired != nil {
			pvcPrefixes = append(pvcPrefixes, fmt.Sprintf("%s-%s-", pvcTemplatePrefix, nsnBuilder.StandaloneName()))
			podPrefixes = append(podPrefixes, fmt.Sprintf("%s-", nsnBuilder.StandaloneName()))
		}
		if replicationDesired != nil {
			pvcPrefixes = append(pvcPrefixes, fmt.Sprintf("%s-%s-", pvcTemplatePrefix, nsnBuilder.ReplicationName()))
			podPrefixes = append(podPrefixes, fmt.Sprintf("%s-", nsnBuilder.ReplicationName()))
		}
		if err := ensurePVCLabels(ctx, cl, specNamespacedName.Namespace, pvcPrefixes, wandbLabels); err != nil {
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

// ensurePVCLabels patches PVCs whose names match any of the given prefixes
// that are missing the wandb labels. The opstree operator creates PVCs via
// StatefulSet volumeClaimTemplates named "redis-data", so PVCs are named
// "redis-data-{crName}-{ordinal}".
func ensurePVCLabels(
	ctx context.Context,
	cl client.Client,
	namespace string,
	namePrefixes []string,
	labels map[string]string,
) error {
	log := logx.GetSlog(ctx)

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(ctx, pvcList, &client.ListOptions{Namespace: namespace}); err != nil {
		return err
	}

	for _, pvc := range pvcList.Items {
		if !matchesAnyPrefix(pvc.Name, namePrefixes) {
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
	specNamespacedName types.NamespacedName,
	nsnBuilder *NsNameBuilder,
	standaloneDesired *redisv1beta2.Redis,
	wandbOwner client.Object,
	onDeleteRule translator.OnDeleteRule,
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

	shouldRemove := found && standaloneDesired == nil
	if shouldRemove {
		if onDeleteRule.Policy == translator.Detach {
			if err := DetachFinalizer(ctx, cl, specNamespacedName, wandbOwner); err != nil {
				result = append(result, metav1.Condition{
					Type:   RedisStandaloneCustomResourceType,
					Status: metav1.ConditionFalse,
					Reason: common.PendingDeleteReason,
				})
			}
		}
	}

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
	case common.UpdateAction:
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
	specNamespacedName types.NamespacedName,
	nsnBuilder *NsNameBuilder,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
	wandbOwner client.Object,
	onDeleteRule translator.OnDeleteRule,
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

	shouldRemove := found && sentinelDesired == nil
	if shouldRemove {
		if onDeleteRule.Policy == translator.Detach {
			if err := DetachFinalizer(ctx, cl, specNamespacedName, wandbOwner); err != nil {
				result = append(result, metav1.Condition{
					Type:   RedisSentinelCustomResourceType,
					Status: metav1.ConditionFalse,
					Reason: common.PendingDeleteReason,
				})
			}
		}
	}

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
	case common.UpdateAction:
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
	case common.UpdateAction:
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
