package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/minio/tenant"
	"github.com/wandb/operator/internal/controller/translator/common"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/defaults"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// BuildMinioConfig will create a new common.MinioConfig with defaultConfig applied if not
// present in actual. It should *never* be saved into the CR!
func BuildMinioConfig(actual apiv2.WBMinioSpec, defaultConfig common.MinioConfig) (common.MinioConfig, error) {
	minioConfig := TranslateMinioSpec(actual)

	minioConfig.StorageSize = utils.CoalesceQuantity(minioConfig.StorageSize, defaultConfig.StorageSize)
	minioConfig.Namespace = utils.Coalesce(minioConfig.Namespace, defaultConfig.Namespace)
	minioConfig.Resources = utils.Resources(minioConfig.Resources, defaultConfig.Resources)

	minioConfig.Enabled = actual.Enabled

	return minioConfig, nil
}

func TranslateMinioSpec(spec apiv2.WBMinioSpec) common.MinioConfig {
	config := common.MinioConfig{
		Enabled:     spec.Enabled,
		Namespace:   spec.Namespace,
		StorageSize: spec.StorageSize,
	}
	if spec.Config != nil {
		config.Resources = spec.Config.Resources
	}

	return config
}

func ExtractMinioStatus(ctx context.Context, results *common.Results) apiv2.WBMinioStatus {
	return TranslateMinioStatus(
		ctx,
		common.ExtractMinioStatus(ctx, results),
	)
}

func TranslateMinioStatus(ctx context.Context, m common.MinioStatus) apiv2.WBMinioStatus {
	var result apiv2.WBMinioStatus
	var details []apiv2.WBStatusDetail

	for _, err := range m.Errors {
		details = append(details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.Code(),
			Message: err.Reason(),
		})
	}

	for _, detail := range m.Details {
		state := translateMinioStatusCode(detail.Code())
		details = append(details, apiv2.WBStatusDetail{
			State:   state,
			Code:    detail.Code(),
			Message: detail.Message(),
		})
	}

	result.Connection = apiv2.WBMinioConnection{
		MinioHost:      m.Connection.Host,
		MinioPort:      m.Connection.Port,
		MinioAccessKey: m.Connection.AccessKey,
	}

	result.Ready = m.Ready
	result.Details = details
	result.State = computeOverallState(details, m.Ready)
	result.LastReconciled = metav1.Now()

	return result
}

func translateMinioStatusCode(code string) apiv2.WBStateType {
	switch code {
	case string(common.MinioCreatedCode):
		return apiv2.WBStateUpdating
	case string(common.MinioUpdatedCode):
		return apiv2.WBStateUpdating
	case string(common.MinioDeletedCode):
		return apiv2.WBStateDeleting
	case string(common.MinioConnectionCode):
		return apiv2.WBStateReady
	default:
		return apiv2.WBStateUnknown
	}
}

func (i *InfraConfigBuilder) AddMinioConfig(actual apiv2.WBMinioSpec) *InfraConfigBuilder {
	var err error
	var size common.Size
	var defaultConfig common.MinioConfig
	var mergedConfig common.MinioConfig

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = defaults.BuildMinioDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	mergedConfig, err = BuildMinioConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedMinio = mergedConfig
	return i
}

// ToMinioVendorSpec converts a WBMinioSpec to a Minio Tenant CR.
// This function translates the high-level Minio spec into the vendor-specific
// Tenant format used by the Minio operator.
func ToMinioVendorSpec(
	ctx context.Context,
	spec apiv2.WBMinioSpec,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*miniov2.Tenant, error) {
	log := ctrl.LoggerFrom(ctx)

	// Parse storage quantity
	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

	// Determine volumes per server based on replica count
	volumesPerServer := common.DevVolumesPerServer
	if spec.Replicas > 1 {
		volumesPerServer = common.ProdVolumesPerServer
	}

	// Build Tenant spec
	minioTenant := &miniov2.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenant.TenantName,
			Namespace: spec.Namespace,
			Labels: map[string]string{
				"app": tenant.TenantName,
			},
		},
		Spec: miniov2.TenantSpec{
			Image: common.MinioImage,
			Configuration: &corev1.LocalObjectReference{
				Name: tenant.TenantName + "-config",
			},
			Pools: []miniov2.Pool{
				{
					Name:             tenant.PoolName,
					Servers:          spec.Replicas,
					VolumesPerServer: volumesPerServer,
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: storageQuantity,
								},
							},
						},
					},
				},
			},
		},
	}

	// Add resources if specified
	if spec.Config != nil && (len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0) {
		minioTenant.Spec.Pools[0].Resources = corev1.ResourceRequirements{
			Requests: spec.Config.Resources.Requests,
			Limits:   spec.Config.Resources.Limits,
		}
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, minioTenant, scheme); err != nil {
		log.Error(err, "failed to set owner reference on Tenant CR")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return minioTenant, nil
}
