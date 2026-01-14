package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/minio/tenant"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	miniov2 "github.com/wandb/operator/pkg/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// createMinioTelemetryEnv creates environment variables for MinIO telemetry if enabled.
// Disables authentication for Prometheus metrics endpoint in dev environments.
// Returns nil if telemetry is disabled.
func createMinioTelemetryEnv(telemetry apiv2.Telemetry) []corev1.EnvVar {
	if !telemetry.Enabled {
		return nil
	}

	return []corev1.EnvVar{
		{
			Name:  "MINIO_PROMETHEUS_AUTH_TYPE",
			Value: "public",
		},
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
	ctx, log := logx.IntoContext(ctx, logx.Minio)

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
	volumesPerServer := translator.DevVolumesPerServer
	if spec.Replicas > 1 {
		volumesPerServer = translator.ProdVolumesPerServer
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
			Image: translator.MinioImage,
			Configuration: &corev1.LocalObjectReference{
				Name: tenant.ConfigName(specName),
			},
			Pools: []miniov2.Pool{
				{
					Name:             tenant.PoolName(specName),
					Affinity:         spec.Affinity,
					Tolerations:      *spec.Tolerations,
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
			Buckets: []miniov2.Bucket{
				{
					Name: "bucket",
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

	// Add telemetry environment variables if enabled
	minioTenant.Spec.Env = createMinioTelemetryEnv(spec.Telemetry)

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, minioTenant, scheme); err != nil {
		log.Error(err, "failed to set owner reference on Tenant CR")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return minioTenant, nil
}

func ToMinioEnvConfig(
	ctx context.Context,
	spec apiv2.WBMinioSpec,
) (tenant.MinioEnvConfig, error) {
	return tenant.MinioEnvConfig{
		RootUser:            spec.Config.RootUser,
		MinioBrowserSetting: spec.Config.MinioBrowserSetting,
	}, nil
}
