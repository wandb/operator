package seaweedfs

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	ObjectStoreModuleName = "seaweedfs"
	SeaweedImage          = "chrislusf/seaweedfs:latest"
)

const (
	seaweedWritableTmpVolumeName = "seaweedfs-tmp"
	seaweedWritableTmpMountPath  = "/tmp"
	seaweedFilerDataMountPath    = "/data/filerldb2"
)

func seaweedWritableVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: seaweedWritableTmpVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
}

func seaweedWritableVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: seaweedWritableTmpVolumeName, MountPath: seaweedWritableTmpMountPath},
	}
}

func ToObjectStoreVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*seaweedv1.Seaweed, error) {
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

	replication := "000"
	if infraSpec.Replicas > 1 {
		replication = "001"
	}

	volumeSizeLimitMB := int32(storageQuantity.Value() / (1024 * 1024))

	labels := BuildWandbObjectStoreLabels(wandb)
	labels["app"] = SeaweedName(specName)

	seaweedCR := &seaweedv1.Seaweed{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SeaweedName(specName),
			Namespace: infraSpec.Namespace,
			Labels:    labels,
		},
		Spec: seaweedv1.SeaweedSpec{
			Image: SeaweedImage,
			TLS: &seaweedv1.TLSSpec{
				Enabled: infraSpec.SeaweedObjectStoreSpec.TlsEnabled,
			},
			Master: &seaweedv1.MasterSpec{
				Replicas:           1,
				DefaultReplication: &replication,
				VolumeSizeLimitMB:  &volumeSizeLimitMB,
				ComponentSpec: seaweedv1.ComponentSpec{
					Volumes:      seaweedWritableVolumes(),
					VolumeMounts: seaweedWritableVolumeMounts(),
				},
			},
			Volume: &seaweedv1.VolumeSpec{
				Replicas: infraSpec.Replicas,
				VolumeServerConfig: seaweedv1.VolumeServerConfig{
					ComponentSpec: seaweedv1.ComponentSpec{
						Volumes:      seaweedWritableVolumes(),
						VolumeMounts: seaweedWritableVolumeMounts(),
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: storageQuantity,
						},
					},
				},
			},
			S3: &seaweedv1.S3GatewaySpec{
				ComponentSpec: seaweedv1.ComponentSpec{
					Affinity:    wandb.GetAffinity(infraSpec.ManagedInfraSpec),
					Tolerations: *wandb.GetTolerations(infraSpec.ManagedInfraSpec),
				},
				ResourceRequirements: corev1.ResourceRequirements{},
				Replicas:             1,
				ConfigSecret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ConfigName(specName),
					},
					Key: "config.json",
				},
				Port:       new(int32(80)),
				DomainName: nil,
			},
			Filer: &seaweedv1.FilerSpec{
				Replicas: 1,
				Config:   ptr.To("[leveldb2]\nenabled = true\ndir = \"" + seaweedFilerDataMountPath + "\""),
				ComponentSpec: seaweedv1.ComponentSpec{
					Volumes:      seaweedWritableVolumes(),
					VolumeMounts: seaweedWritableVolumeMounts(),
				},
				Persistence: &seaweedv1.PersistenceSpec{
					Enabled:   true,
					MountPath: ptr.To(seaweedFilerDataMountPath),
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: storageQuantity,
						},
					},
				},
			},
			Affinity:    wandb.GetAffinity(infraSpec.ManagedInfraSpec),
			Tolerations: *wandb.GetTolerations(infraSpec.ManagedInfraSpec),
		},
	}

	if len(infraSpec.Config.Resources.Requests) > 0 || len(infraSpec.Config.Resources.Limits) > 0 {
		seaweedCR.Spec.Volume.ResourceRequirements = corev1.ResourceRequirements{
			Requests: infraSpec.Config.Resources.Requests,
			Limits:   infraSpec.Config.Resources.Limits,
		}
	}

	if err := ctrl.SetControllerReference(wandb, seaweedCR, scheme); err != nil {
		log.Error("failed to set owner reference on Seaweed CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return seaweedCR, nil
}

func ToObjectStoreEnvConfig(
	ctx context.Context,
	spec apiv2.ManagedObjectStoreSpec,
) (SeaweedS3Config, error) {
	return SeaweedS3Config{
		AccessKey: spec.Config.AccessKey,
	}, nil
}

func BuildWandbObjectStoreLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return common.BuildWandbLabels(wandb, ObjectStoreModuleName)
}

func ToObjectStoreOnDeleteRule(wandb *apiv2.WeightsAndBiases, retentionPolicy apiv2.RetentionPolicy) common.OnDeleteRule {
	return common.ToOnDeleteRule(wandb, retentionPolicy, ObjectStoreModuleName)
}
