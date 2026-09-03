package reconciler

import (
	"context"
	"fmt"
	"sort"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type seaweedFSVolumeTarget struct {
	component string
	name      string
	namespace string
	desired   string
}

type volumeExpansionBlocker struct {
	component    string
	pvc          types.NamespacedName
	storageClass string
	current      resource.Quantity
	desired      resource.Quantity
}

type volumeExpansionPending struct {
	component string
	pvc       types.NamespacedName
	current   resource.Quantity
	desired   resource.Quantity
}

func (b volumeExpansionBlocker) String() string {
	class := b.storageClass
	if class == "" {
		class = "<none>"
	}
	return fmt.Sprintf(
		"%s requires PVC %s/%s to grow from %s to %s, but StorageClass %q does not allow volume expansion",
		b.component,
		b.pvc.Namespace,
		b.pvc.Name,
		b.current.String(),
		b.desired.String(),
		class,
	)
}

func reconcileSeaweedFSVolumeExpansion(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
) ([]volumeExpansionBlocker, []volumeExpansionPending, error) {
	pvcsByNamespace := map[string][]corev1.PersistentVolumeClaim{}
	storageClasses := map[string]*storagev1.StorageClass{}
	var blockers []volumeExpansionBlocker
	var pending []volumeExpansionPending

	for _, target := range seaweedFSVolumeTargets(wandb) {
		if target.desired == "" || target.namespace == "" || target.name == "" {
			continue
		}

		pvcs, ok := pvcsByNamespace[target.namespace]
		if !ok {
			pvcList := &corev1.PersistentVolumeClaimList{}
			if err := c.List(ctx, pvcList, client.InNamespace(target.namespace)); err != nil {
				return nil, nil, fmt.Errorf("list PVCs in namespace %q: %w", target.namespace, err)
			}
			pvcs = pvcList.Items
			pvcsByNamespace[target.namespace] = pvcs
		}

		desired, err := resource.ParseQuantity(target.desired)
		if err != nil {
			return nil, nil, fmt.Errorf("parse desired storage for %s: %w", target.component, err)
		}
		for i := range pvcs {
			pvc := &pvcs[i]
			if pvc.Labels["app.kubernetes.io/instance"] != target.name ||
				pvc.Labels["app.kubernetes.io/component"] != target.component {
				continue
			}

			current := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			if desired.Cmp(current) > 0 {
				storageClassName := ""
				if pvc.Spec.StorageClassName != nil {
					storageClassName = *pvc.Spec.StorageClassName
				}
				expandable, err := storageClassAllowsExpansion(ctx, c, storageClasses, storageClassName)
				if err != nil {
					return nil, nil, fmt.Errorf(
						"check StorageClass for PVC %s/%s: %w",
						pvc.Namespace,
						pvc.Name,
						err,
					)
				}
				if !expandable {
					blockers = append(blockers, volumeExpansionBlocker{
						component:    "objectStore/" + target.component,
						pvc:          client.ObjectKeyFromObject(pvc),
						storageClass: storageClassName,
						current:      current,
						desired:      desired,
					})
					continue
				}

				before := pvc.DeepCopy()
				if pvc.Spec.Resources.Requests == nil {
					pvc.Spec.Resources.Requests = corev1.ResourceList{}
				}
				pvc.Spec.Resources.Requests[corev1.ResourceStorage] = desired
				if err := c.Patch(ctx, pvc, client.MergeFrom(before)); err != nil {
					return nil, nil, fmt.Errorf("expand PVC %s/%s: %w", pvc.Namespace, pvc.Name, err)
				}
			}

			capacity := pvc.Status.Capacity[corev1.ResourceStorage]
			if desired.Cmp(capacity) > 0 {
				pending = append(pending, volumeExpansionPending{
					component: "objectStore/" + target.component,
					pvc:       client.ObjectKeyFromObject(pvc),
					current:   capacity,
					desired:   desired,
				})
			}
		}
	}

	sort.Slice(blockers, func(i, j int) bool {
		return blockers[i].String() < blockers[j].String()
	})
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].pvc.String() < pending[j].pvc.String()
	})
	return blockers, pending, nil
}

func storageClassAllowsExpansion(
	ctx context.Context,
	c client.Client,
	cache map[string]*storagev1.StorageClass,
	name string,
) (bool, error) {
	if name == "" {
		return false, nil
	}
	storageClass, ok := cache[name]
	if !ok {
		storageClass = &storagev1.StorageClass{}
		if err := c.Get(ctx, types.NamespacedName{Name: name}, storageClass); err != nil {
			return false, fmt.Errorf("get StorageClass %q: %w", name, err)
		}
		cache[name] = storageClass
	}
	return storageClass.AllowVolumeExpansion != nil && *storageClass.AllowVolumeExpansion, nil
}

func seaweedFSVolumeTargets(wandb *apiv2.WeightsAndBiases) []seaweedFSVolumeTarget {
	var targets []seaweedFSVolumeTarget
	for _, instance := range wandb.Spec.ObjectStore {
		spec := instance.ManagedObjectStore
		if spec == nil {
			continue
		}
		targets = append(targets,
			seaweedFSVolumeTarget{
				component: "volume",
				name:      spec.Name,
				namespace: spec.Namespace,
				desired:   spec.StorageSize,
			},
			seaweedFSVolumeTarget{
				component: "filer",
				name:      spec.Name,
				namespace: spec.Namespace,
				desired:   spec.SeaweedObjectStoreSpec.FilerStorageSize,
			},
		)
	}
	return targets
}

func volumeExpansionBlockersMessage(blockers []volumeExpansionBlocker) string {
	messages := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		messages = append(messages, blocker.String())
	}
	return strings.Join(messages, "; ")
}

func volumeExpansionPendingMessage(pending []volumeExpansionPending) string {
	messages := make([]string, 0, len(pending))
	for _, item := range pending {
		messages = append(messages, fmt.Sprintf(
			"waiting for %s PVC %s/%s capacity to grow from %s to %s",
			item.component,
			item.pvc.Namespace,
			item.pvc.Name,
			item.current.String(),
			item.desired.String(),
		))
	}
	return strings.Join(messages, "; ")
}
