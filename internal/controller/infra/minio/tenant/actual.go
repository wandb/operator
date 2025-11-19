package tenant

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/translator/common"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type minioTenant struct {
	tenant       *miniov2.Tenant
	configSecret *corev1.Secret
	config       common.MinioConfig
	client       client.Client
	owner        metav1.Object
	scheme       *runtime.Scheme
}

// Initialize fetches existing Minio Tenant CR
func Initialize(
	ctx context.Context,
	client client.Client,
	minioConfig common.MinioConfig,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*minioTenant, error) {
	log := ctrl.LoggerFrom(ctx)

	var tenant = &miniov2.Tenant{}
	var configSecret = &corev1.Secret{}
	var result = minioTenant{
		config:       minioConfig,
		client:       client,
		owner:        owner,
		scheme:       scheme,
		tenant:       nil,
		configSecret: nil,
	}

	// Try to get Tenant CR
	tenantKey := types.NamespacedName{
		Name:      TenantName,
		Namespace: minioConfig.Namespace,
	}
	err := client.Get(ctx, tenantKey, tenant)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error getting actual Tenant CR")
			return nil, err
		}
	} else {
		result.tenant = tenant
	}

	// Try to get config secret
	secretKey := types.NamespacedName{
		Name:      TenantName + "-config",
		Namespace: minioConfig.Namespace,
	}
	err = client.Get(ctx, secretKey, configSecret)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error getting actual config secret")
			return nil, err
		}
	} else {
		result.configSecret = configSecret
	}

	return &result, nil
}

// Upsert creates or updates Minio Tenant CR and config secret based on whether they exist
func (a *minioTenant) Upsert(ctx context.Context, minioConfig common.MinioConfig) *common.Results {
	results := common.InitResults()
	var nextResults *common.Results

	// Build desired config secret (must be created before Tenant)
	desiredSecret, nextResults := buildDesiredSecret(ctx, minioConfig, a.owner, a.scheme)
	results.Merge(nextResults)
	if results.HasCriticalError() {
		return results
	}

	// Handle config secret (must exist before Tenant CR)
	if a.configSecret == nil {
		nextResults = a.createSecret(ctx, desiredSecret)
		results.Merge(nextResults)
		if results.HasCriticalError() {
			return results
		}
	}

	// Build desired Tenant CR
	desiredTenant, nextResults := buildDesiredTenant(ctx, minioConfig, a.owner, a.scheme)
	results.Merge(nextResults)
	if results.HasCriticalError() {
		return results
	}

	// Handle Tenant CR
	if a.tenant == nil {
		nextResults = a.createTenant(ctx, desiredTenant)
		results.Merge(nextResults)
	} else {
		nextResults = a.updateTenant(ctx, desiredTenant, minioConfig)
		results.Merge(nextResults)
	}

	return results
}

// Delete removes Minio Tenant CR and config secret
func (a *minioTenant) Delete(ctx context.Context) *common.Results {
	log := ctrl.LoggerFrom(ctx)
	results := common.InitResults()

	// Delete Tenant CR first
	if a.tenant != nil {
		if err := a.client.Delete(ctx, a.tenant); err != nil {
			log.Error(err, "Failed to delete Tenant CR")
			results.AddErrors(common.NewMinioError(
				common.MinioErrFailedToDeleteCode,
				fmt.Sprintf("failed to delete Tenant: %v", err),
			))
			return results
		}
		results.AddStatuses(common.NewMinioStatusDetail(common.MinioDeletedCode, TenantName))
	}

	// Delete config secret
	if a.configSecret != nil {
		if err := a.client.Delete(ctx, a.configSecret); err != nil {
			log.Error(err, "Failed to delete config secret")
			results.AddErrors(common.NewMinioError(
				common.MinioErrFailedToDeleteCode,
				fmt.Sprintf("failed to delete config secret: %v", err),
			))
			return results
		}
		log.Info("Deleted Minio config secret", "secret", a.configSecret.Name)
	}

	return results
}
