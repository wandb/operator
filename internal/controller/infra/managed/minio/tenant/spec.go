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

const (
	minioRunAsUser  int64 = 1000
	minioRunAsGroup int64 = 1000
	minioFSGroup    int64 = 1000

	minioWritableTmpVolumeName = "minio-tmp"
	minioWritableTmpMountPath  = "/tmp"

	minioCapabilityAll corev1.Capability = "ALL"
)

func minioPodSecurityContext() *corev1.PodSecurityContext {
	if utils.IsOpenShift() {
		return &corev1.PodSecurityContext{
			RunAsNonRoot:   ptr.Bool(true),
			SeccompProfile: minioRuntimeDefaultSeccompProfile(),
		}
	}

	return &corev1.PodSecurityContext{
		RunAsUser:      ptr.Int64(minioRunAsUser),
		RunAsGroup:     ptr.Int64(minioRunAsGroup),
		RunAsNonRoot:   ptr.Bool(true),
		FSGroup:        ptr.Int64(minioFSGroup),
		SeccompProfile: minioRuntimeDefaultSeccompProfile(),
	}
}

func minioContainerSecurityContext() *corev1.SecurityContext {
	securityContext := &corev1.SecurityContext{
		RunAsNonRoot:             ptr.Bool(true),
		AllowPrivilegeEscalation: ptr.Bool(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{minioCapabilityAll},
		},
		SeccompProfile: minioRuntimeDefaultSeccompProfile(),
	}
	if !utils.IsOpenShift() {
		securityContext.RunAsUser = ptr.Int64(minioRunAsUser)
		securityContext.RunAsGroup = ptr.Int64(minioRunAsGroup)
	}
	return securityContext
}

func minioWritableVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: minioWritableTmpVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
}

func minioWritableVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: minioWritableTmpVolumeName, MountPath: minioWritableTmpMountPath},
	}
}

func minioRuntimeDefaultSeccompProfile() *corev1.SeccompProfile {
	return &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
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
			Image:     MinioImage,
			Mountpath: miniov2.MinIOVolumeMountPath,
			Subpath:   miniov2.MinIOVolumeSubPath,
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
			AdditionalVolumes:      minioWritableVolumes(),
			AdditionalVolumeMounts: minioWritableVolumeMounts(),
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
