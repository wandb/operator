package tenant

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	miniov2 "github.com/wandb/operator/pkg/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	ObjectStoreModuleName = "minio"
	MinioImage            = "quay.io/minio/minio:latest"
	DevVolumesPerServer   = int32(1)
	ProdVolumesPerServer  = int32(4)
)

// minioPodSecurityContext returns the pool-level pod SecurityContext to apply
// to managed MinIO tenants. Returns nil outside of OpenShift so the
// minio-operator's defaults apply.
func minioPodSecurityContext() *corev1.PodSecurityContext {
	if utils.IsOpenShift() {
		return utils.OpenShiftPodSecurityContext()
	}
	return nil
}

// minioContainerSecurityContext returns the pool-level container SecurityContext
// to apply to managed MinIO tenants. Returns nil outside of OpenShift.
func minioContainerSecurityContext() *corev1.SecurityContext {
	if utils.IsOpenShift() {
		return utils.OpenShiftContainerSecurityContext()
	}
	return nil
}

func createObjectStoreTelemetryEnv(telemetry apiv2.Telemetry) []corev1.EnvVar {
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

func ToObjectStoreVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*miniov2.Tenant, error) {
	_, log := logx.WithSlog(ctx, logx.ObjectStore)
	infraSpec := wandb.Spec.ObjectStore.ManagedObjectStore
	if infraSpec == nil {
		return nil, nil
	}

	specName := infraSpec.Name

	storageQuantity, err := resource.ParseQuantity(infraSpec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", infraSpec.StorageSize, err)
	}

	volumesPerServer := DevVolumesPerServer
	if infraSpec.Replicas > 1 {
		volumesPerServer = ProdVolumesPerServer
	}

	minioTenant := &miniov2.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TenantName(specName),
			Namespace: infraSpec.Namespace,
			Labels: map[string]string{
				"app": TenantName(specName),
			},
		},
		Spec: miniov2.TenantSpec{
			Image: MinioImage,
			Configuration: &corev1.LocalObjectReference{
				Name: ConfigName(specName),
			},
			ServiceMetadata: &miniov2.ServiceMetadata{
				MinIOServiceLabels: BuildWandbObjectStoreLabels(wandb),
			},
			PoolsMetadata: &miniov2.PoolsMetadata{
				Labels: BuildWandbObjectStoreLabels(wandb),
			},
			Pools: []miniov2.Pool{
				{
					Name:                     "default",
					Affinity:                 wandb.GetAffinity(infraSpec.ManagedInfraSpec),
					Tolerations:              *wandb.GetTolerations(infraSpec.ManagedInfraSpec),
					Servers:                  infraSpec.Replicas,
					VolumesPerServer:         volumesPerServer,
					SecurityContext:          minioPodSecurityContext(),
					ContainerSecurityContext: minioContainerSecurityContext(),
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Labels: BuildWandbObjectStoreLabels(wandb),
						},
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
			RequestAutoCert: ptr.Bool(false),
		},
	}

	if len(infraSpec.Config.Resources.Requests) > 0 || len(infraSpec.Config.Resources.Limits) > 0 {
		minioTenant.Spec.Pools[0].Resources = corev1.ResourceRequirements{
			Requests: infraSpec.Config.Resources.Requests,
			Limits:   infraSpec.Config.Resources.Limits,
		}
	}

	minioTenant.Spec.Env = createObjectStoreTelemetryEnv(infraSpec.Telemetry)

	if err := ctrl.SetControllerReference(wandb, minioTenant, scheme); err != nil {
		log.Error("failed to set owner reference on Tenant CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return minioTenant, nil
}

func ToObjectStoreEnvConfig(
	ctx context.Context,
	spec apiv2.ManagedObjectStoreSpec,
) (MinioEnvConfig, error) {
	return MinioEnvConfig{
		RootUser:            spec.Config.RootUser,
		MinioBrowserSetting: spec.Config.MinioBrowserSetting,
	}, nil
}

func BuildWandbObjectStoreLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return common.BuildWandbLabels(wandb, ObjectStoreModuleName)
}

func ToObjectStoreOnDeleteRule(wandb *apiv2.WeightsAndBiases, retentionPolicy apiv2.RetentionPolicy) common.OnDeleteRule {
	return common.ToOnDeleteRule(wandb, retentionPolicy, ObjectStoreModuleName)
}
