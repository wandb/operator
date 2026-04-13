package strimzi

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PurgeFinalizer deletes PVCs belonging to Kafka when the retention policy is
// Purge. Strimzi normally handles this via KafkaNodePool DeleteClaim, but we
// also clean up here as a safety net.
func PurgeFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	onDeleteRule translator.OnDeleteRule,
) error {
	ctx, _ = logx.WithSlog(ctx, logx.Kafka)
	if onDeleteRule.Policy != translator.Purge {
		return nil
	}
	return purgeAssociatedResources(ctx, cl, specNamespacedName.Namespace, onDeleteRule.Selector)
}

func purgeAssociatedResources(
	ctx context.Context,
	cl client.Client,
	namespace string,
	onDeleteSelector labels.Selector,
) error {
	listOptions := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: onDeleteSelector,
	}

	// Deployments
	deploymentList := &appsv1.DeploymentList{}
	if err := deleteMatchingResources(ctx, cl, listOptions, deploymentList, "Deployments"); err != nil {
		return err
	}

	// StrimziPodSets
	if err := deleteMatchingStrimziPodSets(ctx, cl, listOptions); err != nil {
		return err
	}

	// Pods
	podList := &corev1.PodList{}
	if err := deleteMatchingResources(ctx, cl, listOptions, podList, "Pods"); err != nil {
		return err
	}

	// PVCs
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := deleteMatchingResources(ctx, cl, listOptions, pvcList, "PVCs"); err != nil {
		return err
	}

	return nil
}

func deleteMatchingResources(
	ctx context.Context,
	cl client.Client,
	listOptions *client.ListOptions,
	list client.ObjectList,
	resourceType string,
) error {
	log := logx.GetSlog(ctx)

	if err := cl.List(ctx, list, listOptions); err != nil {
		return err
	}

	var objects []client.Object
	if err := apimeta.EachListItem(list, func(obj runtime.Object) error {
		cobj, ok := obj.(client.Object)
		if !ok {
			return fmt.Errorf("list item for %s does not implement client.Object", resourceType)
		}
		objects = append(objects, cobj)
		return nil
	}); err != nil {
		return err
	}

	if len(objects) > 0 {
		log.Info(
			"Purging associated "+resourceType,
			"count", len(objects), "selector", listOptions.LabelSelector.String(),
		)
	} else {
		log.Debug(
			"No associated "+resourceType+" found to purge",
			"selector", listOptions.LabelSelector.String(),
		)
	}

	for _, obj := range objects {
		if err := cl.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func deleteMatchingStrimziPodSets(
	ctx context.Context,
	cl client.Client,
	listOptions *client.ListOptions,
) error {
	log := logx.GetSlog(ctx)
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "core.strimzi.io",
		Version: "v1",
		Kind:    "StrimziPodSetList",
	})

	if err := cl.List(ctx, list, listOptions); err != nil {
		if apimeta.IsNoMatchError(err) {
			return nil
		}
		return err
	}

	var objects []client.Object
	if err := apimeta.EachListItem(list, func(obj runtime.Object) error {
		cobj, ok := obj.(client.Object)
		if !ok {
			return fmt.Errorf("list item for StrimziPodSets does not implement client.Object")
		}
		objects = append(objects, cobj)
		return nil
	}); err != nil {
		return err
	}

	if len(objects) > 0 {
		log.Info(
			"Purging associated StrimziPodSets",
			"count", len(objects), "selector", listOptions.LabelSelector.String(),
		)
	} else {
		log.Debug(
			"No associated StrimziPodSets found to purge",
			"selector", listOptions.LabelSelector.String(),
		)
	}

	for _, obj := range objects {
		if err := cl.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}
