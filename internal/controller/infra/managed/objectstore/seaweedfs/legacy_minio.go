package seaweedfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var minioTenantGVK = schema.GroupVersionKind{
	Group:   "minio.min.io",
	Version: "v2",
	Kind:    "Tenant",
}

// CleanupLegacyMinio detects and cleans up a MinIO Tenant CR left over from
// before the SeaweedFS migration. It uses an unstructured client so we don't
// need to vendor the MinIO operator types.
//
// If the MinIO CRD is not installed, this is a no-op.
// If a Tenant exists owned by this WeightsAndBiases CR, it is either detached
// (owner reference removed) or purged (deleted along with associated PVCs and
// Secrets) based on the purge parameter.
func CleanupLegacyMinio(
	ctx context.Context,
	cl client.Client,
	wandbName string,
	wandbNamespace string,
	wandbUID types.UID,
	purge bool,
	onDeleteSelector labels.Selector,
) error {
	ctx, log := logx.WithSlog(ctx, logx.ObjectStore)

	tenantName := fmt.Sprintf("%s-minio", wandbName)
	tenantNsName := types.NamespacedName{Name: tenantName, Namespace: wandbNamespace}

	tenant := &unstructured.Unstructured{}
	tenant.SetGroupVersionKind(minioTenantGVK)

	err := cl.Get(ctx, tenantNsName, tenant)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		if isNoMatchError(err) {
			return nil
		}
		return err
	}

	owned := false
	for _, ref := range tenant.GetOwnerReferences() {
		if ref.UID == wandbUID {
			owned = true
			break
		}
	}
	if !owned {
		log.Info("legacy MinIO Tenant exists but is not owned by this WeightsAndBiases CR, skipping",
			"tenant", tenantName)
		return nil
	}

	if purge {
		log.Info("purging legacy MinIO Tenant CR", "tenant", tenantName)
		if err := cl.Delete(ctx, tenant); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete legacy MinIO Tenant: %w", err)
		}
		if onDeleteSelector != nil {
			if err := purgeLegacyMinioResources(ctx, cl, wandbNamespace, onDeleteSelector); err != nil {
				return err
			}
		}
	} else {
		log.Info("detaching legacy MinIO Tenant CR", "tenant", tenantName)
		refs := tenant.GetOwnerReferences()
		filtered := make([]interface{}, 0, len(refs))
		for _, ref := range refs {
			if ref.UID != wandbUID {
				filtered = append(filtered, map[string]interface{}{
					"apiVersion":         ref.APIVersion,
					"kind":               ref.Kind,
					"name":               ref.Name,
					"uid":                string(ref.UID),
					"controller":         ref.Controller,
					"blockOwnerDeletion": ref.BlockOwnerDeletion,
				})
			}
		}
		if err := unstructured.SetNestedSlice(tenant.Object, filtered, "metadata", "ownerReferences"); err != nil {
			return fmt.Errorf("failed to update owner references: %w", err)
		}
		if err := cl.Update(ctx, tenant); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to detach legacy MinIO Tenant: %w", err)
		}
	}

	connSecretName := fmt.Sprintf("%s-objectstore-connection", wandbName)
	connSecret := &corev1.Secret{}
	if err := cl.Get(ctx, types.NamespacedName{Name: connSecretName, Namespace: wandbNamespace}, connSecret); err == nil {
		if purge {
			if err := cl.Delete(ctx, connSecret); err != nil && !errors.IsNotFound(err) {
				return err
			}
			log.Info("deleted legacy MinIO connection secret", "secret", connSecretName)
		} else {
			common.RemoveOwnerReference(connSecret, wandbUID)
			if err := cl.Update(ctx, connSecret); err != nil && !errors.IsNotFound(err) {
				return err
			}
			log.Info("detached legacy MinIO connection secret", "secret", connSecretName)
		}
	}

	log.Info("legacy MinIO cleanup complete", "tenant", tenantName, "purged", purge)
	return nil
}

func purgeLegacyMinioResources(
	ctx context.Context,
	cl client.Client,
	namespace string,
	selector labels.Selector,
) error {
	log := logx.GetSlog(ctx)
	listOptions := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: selector,
	}

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(ctx, pvcList, listOptions); err != nil {
		return err
	}
	for _, pvc := range pvcList.Items {
		if err := cl.Delete(ctx, &pvc); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	if len(pvcList.Items) > 0 {
		log.Info("purged legacy MinIO PVCs", "count", len(pvcList.Items))
	}

	secretList := &corev1.SecretList{}
	if err := cl.List(ctx, secretList, listOptions); err != nil {
		return err
	}
	for _, secret := range secretList.Items {
		if err := cl.Delete(ctx, &secret); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	if len(secretList.Items) > 0 {
		log.Info("purged legacy MinIO Secrets", "count", len(secretList.Items))
	}

	return nil
}

func isNoMatchError(err error) bool {
	return strings.Contains(err.Error(), "no matches for") ||
		strings.Contains(err.Error(), "the server could not find the requested resource")
}
