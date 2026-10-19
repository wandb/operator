package bufstream

import (
	"context"
	"strconv"
	"strings"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PurgeFinalizer deletes the etcd PVCs when the retention policy is Purge. The
// Application CRs, ConfigMap and Secret are owner-referenced by the wandb CR and
// garbage collected automatically, but StatefulSet PVCs are not.
func PurgeFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	onDeleteRule common.OnDeleteRule,
) error {
	ctx, _ = logx.WithSlog(ctx, logx.Kafka)
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
	listOptions := &client.ListOptions{
		Namespace: specNamespacedName.Namespace,
	}

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(ctx, pvcList, listOptions); err != nil {
		return err
	}
	for i := range pvcList.Items {
		pvc := &pvcList.Items[i]
		if !onDeleteSelector.Matches(labels.Set(pvc.Labels)) && !isLegacyEtcdPVC(pvc, specNamespacedName, onDeleteSelector) {
			continue
		}
		log.Info("Purging associated PVC", "pvc", pvc.Name, "selector", onDeleteSelector.String())
		if err := cl.Delete(ctx, pvc); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// Older claim templates omitted the W&B labels. StatefulSet templates are
// immutable, so a template fix alone cannot repair claims on existing clusters.
// Match the exact etcd workload's Kubernetes labels and generated claim name;
// never override an explicit W&B ownership label pointing at another CR.
func isLegacyEtcdPVC(pvc *corev1.PersistentVolumeClaim, nsn types.NamespacedName, selector labels.Selector) bool {
	etcdName := createNsNameBuilder(nsn).EtcdName()
	if pvc.Namespace != nsn.Namespace || pvc.Labels["app.kubernetes.io/name"] != etcdName || pvc.Labels["app.kubernetes.io/instance"] != nsn.Namespace {
		return false
	}
	for _, key := range []string{common.WandbNameLabel, common.WandbNamespaceLabel, common.WandbComponentLabel} {
		if actual, exists := pvc.Labels[key]; exists {
			want, exact := selector.RequiresExactMatch(key)
			if !exact || actual != want {
				return false
			}
		}
	}
	prefix := EtcdDataVolumeName + "-" + etcdName + "-"
	if !strings.HasPrefix(pvc.Name, prefix) {
		return false
	}
	suffix := strings.TrimPrefix(pvc.Name, prefix)
	ordinal, err := strconv.Atoi(suffix)
	return err == nil && ordinal >= 0 && strconv.Itoa(ordinal) == suffix
}
