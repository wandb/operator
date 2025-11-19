package tenant

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/translator/common"
	"github.com/wandb/operator/internal/defaults"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// buildDesiredTenant creates a Minio Tenant CR based on the provided config.
// Handles both dev (1 server, 1 volume) and small (3 servers, 4 volumes) configurations.
func buildDesiredTenant(
	ctx context.Context,
	minioConfig common.MinioConfig,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*miniov2.Tenant, *common.Results) {
	log := ctrl.LoggerFrom(ctx)
	results := common.InitResults()

	// Parse storage quantity
	storageQuantity, err := resource.ParseQuantity(minioConfig.StorageSize)
	if err != nil {
		log.Error(err, "invalid storage size", "storageSize", minioConfig.StorageSize)
		results.AddErrors(common.NewMinioError(common.MinioErrFailedToCreateCode, fmt.Sprintf("invalid storage size: %v", err)))
		return nil, results
	}

	// Build Tenant spec
	tenant := &miniov2.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TenantName,
			Namespace: minioConfig.Namespace,
			Labels: map[string]string{
				"app": TenantName,
			},
		},
		Spec: miniov2.TenantSpec{
			Image: defaults.MinioImage,
			Configuration: &corev1.LocalObjectReference{
				Name: TenantName + "-config",
			},
			Pools: []miniov2.Pool{
				{
					Name:             PoolName,
					Servers:          minioConfig.Servers,
					VolumesPerServer: minioConfig.VolumesPerServer,
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
	if len(minioConfig.Resources.Requests) > 0 || len(minioConfig.Resources.Limits) > 0 {
		tenant.Spec.Pools[0].Resources = corev1.ResourceRequirements{
			Requests: minioConfig.Resources.Requests,
			Limits:   minioConfig.Resources.Limits,
		}
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, tenant, scheme); err != nil {
		log.Error(err, "failed to set owner reference on Tenant CR")
		results.AddErrors(common.NewMinioError(common.MinioErrFailedToCreateCode, fmt.Sprintf("failed to set owner reference: %v", err)))
		return nil, results
	}

	return tenant, results
}

// buildDesiredSecret creates the Minio configuration secret required by the Tenant CR
func buildDesiredSecret(
	ctx context.Context,
	minioConfig common.MinioConfig,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*corev1.Secret, *common.Results) {
	log := ctrl.LoggerFrom(ctx)
	results := common.InitResults()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TenantName + "-config",
			Namespace: minioConfig.Namespace,
			Labels: map[string]string{
				"app": TenantName,
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"config.env": `export MINIO_ROOT_USER="admin"
export MINIO_ROOT_PASSWORD="admin123456"
export MINIO_BROWSER="on"`,
		},
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, secret, scheme); err != nil {
		log.Error(err, "failed to set owner reference on config secret")
		results.AddErrors(common.NewMinioError(common.MinioErrFailedToCreateCode, fmt.Sprintf("failed to set owner reference on secret: %v", err)))
		return nil, results
	}

	return secret, results
}
