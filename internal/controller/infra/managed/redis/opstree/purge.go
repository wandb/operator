package opstree

import (
	"context"
	"fmt"
	"strings"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PurgeFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	onDeleteRule common.OnDeleteRule,
) error {
	ctx, _ = logx.WithSlog(ctx, logx.Redis)
	if onDeleteRule.Policy != common.Purge {
		return nil
	}
	return purgeAssociatedResources(ctx, cl, specNamespacedName, onDeleteRule.Selector)
}

func purgeAssociatedResources(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	onDeleteSelector labels.Selector,
) error {
	log := logx.GetSlog(ctx)
	nsn := createNsNameBuilder(specNamespacedName)
	instances := map[string]string{nsn.StandaloneName(): "standalone", nsn.ReplicationName(): "replication"}
	names := []string{nsn.StandaloneName(), nsn.ReplicationName()}
	prefixes, err := redisClaimPrefixes(ctx, cl, specNamespacedName.Namespace, names)
	if err != nil {
		return err
	}
	// GC may have removed the StatefulSets before purge. Cover the current
	// upstream default and the legacy template without waiting for W&B labels.
	for _, name := range names {
		prefixes = append(prefixes, fmt.Sprintf("%s-%s-", name, name), fmt.Sprintf("redis-data-%s-", name))
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(ctx, pvcList, client.InNamespace(specNamespacedName.Namespace)); err != nil {
		return err
	}
	for _, pvc := range pvcList.Items {
		if !redisPurgeClaim(&pvc, prefixes, instances, onDeleteSelector) {
			continue
		}
		log.Info("Purging associated Redis PVC", "pvc", pvc.Name)
		if err := cl.Delete(ctx, &pvc); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// Ownership labels are applied asynchronously. Instance names and template
// names let purge find claims before that reconcile, without touching peers.
func redisPurgeClaim(pvc *corev1.PersistentVolumeClaim, prefixes []string, instances map[string]string, owner labels.Selector) bool {
	owned := false
	if owner != nil {
		requirements, _ := owner.Requirements()
		for _, requirement := range requirements {
			if _, present := pvc.Labels[requirement.Key()]; present && !requirement.Matches(labels.Set(pvc.Labels)) {
				return false
			}
		}
		owned = len(requirements) > 0 && owner.Matches(labels.Set(pvc.Labels))
	}
	if matchesPVCName(pvc.Name, prefixes) {
		return true
	}
	// A custom template name can outlive its StatefulSet. Upstream stamps
	// instance/type labels at claim creation even before W&B labels exist.
	ordinalSeparator := strings.LastIndexByte(pvc.Name, '-')
	if ordinalSeparator < 0 {
		return false
	}
	prefix := pvc.Name[:ordinalSeparator+1]
	if !matchesPVCName(pvc.Name, []string{prefix}) {
		return false
	}
	for name, setupType := range instances {
		suffix := "-" + name + "-"
		if len(prefix) > len(suffix) && strings.HasSuffix(prefix, suffix) &&
			(owned || (pvc.Labels["app"] == name && pvc.Labels["redis_setup_type"] == setupType)) {
			return true
		}
	}
	return false
}
