package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/minio/tenant"
	"github.com/wandb/operator/internal/controller/translator/common"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func ExtractMinioStatus(ctx context.Context, conditions []common.MinioCondition) apiv2.WBMinioStatus {
	return TranslateMinioStatus(
		ctx,
		common.ExtractMinioStatus(ctx, conditions),
	)
}

func TranslateMinioStatus(ctx context.Context, m common.MinioStatus) apiv2.WBMinioStatus {
	var result apiv2.WBMinioStatus
	var conditions []apiv2.WBStatusCondition

	for _, condition := range m.Conditions {
		state := translateMinioStatusCode(condition.Code())
		conditions = append(conditions, apiv2.WBStatusCondition{
			State:   state,
			Code:    condition.Code(),
			Message: condition.Message(),
		})
	}

	result.Connection = apiv2.WBMinioConnection{
		MinioHost:      m.Connection.Host,
		MinioPort:      m.Connection.Port,
		MinioAccessKey: m.Connection.AccessKey,
	}

	result.Ready = m.Ready
	result.Conditions = conditions
	result.State = computeOverallState(conditions, m.Ready)
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

	if !spec.Enabled {
		return nil, nil
	}

	specName := spec.Name

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
			Name:      tenant.TenantName(specName),
			Namespace: spec.Namespace,
			Labels: map[string]string{
				"app": tenant.TenantName(specName),
			},
		},
		Spec: miniov2.TenantSpec{
			Image: common.MinioImage,
			Configuration: &corev1.LocalObjectReference{
				Name: tenant.ConfigName(specName),
			},
			Pools: []miniov2.Pool{
				{
					Name:             tenant.PoolName(specName),
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
	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
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

func ToMinioConfigSecret(
	ctx context.Context,
	spec apiv2.WBMinioSpec,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*corev1.Secret, error) {
	log := ctrl.LoggerFrom(ctx)

	specName := spec.Name

	configSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenant.ConfigName(specName),
			Namespace: spec.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"config.env": `export MINIO_ROOT_USER="admin"
export MINIO_ROOT_PASSWORD="admin123456"
export MINIO_BROWSER="on"`,
		},
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, configSecret, scheme); err != nil {
		log.Error(err, "failed to set owner reference on Minio config secret")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return configSecret, nil
}
