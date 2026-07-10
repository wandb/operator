package seaweedfs

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	ObjectStoreModuleName = "seaweedfs"

	// TODO: remove this hardcoded default once all supported manifest versions
	// supply bucket.<instance>.images.seaweedfs.
	defaultSeaweedImage = "chrislusf/seaweedfs:4.35"
)

func SeaweedImage(img manifest.ImageRef, globalImageRegistry string) string {
	if out := img.GetImage(globalImageRegistry); out != "" {
		return out
	}
	// Fallback for older manifests that don't supply the image.
	return defaultSeaweedImage
}

const (
	seaweedWritableTmpVolumeName = "seaweedfs-tmp"
	seaweedWritableTmpMountPath  = "/tmp"
	seaweedFilerDataMountPath    = "/data/filerldb2"

	// Filer holds only the leveldb2 path index, so it needs far less disk than the data volumes.
	seaweedFilerStorageSize = "1Gi"
)

const (
	seaweedMasterMetricsPort int32 = 9091
	seaweedVolumeMetricsPort int32 = 9092
	seaweedFilerMetricsPort  int32 = 9093

	// Volume count auto-sizes as disk / this limit, so keep it small enough that even a modest disk yields writable volumes.
	seaweedVolumeSizeLimitMB int32 = 1024
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
	mfst manifest.Manifest,
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

	replication := seaweedReplication(infraSpec.Copies, infraSpec.Replicas)

	// VolumeSizeLimitMB caps per-volume rollover, not total capacity (the volume PVC governs that).
	volumeSizeLimitMB := seaweedVolumeSizeLimitMB

	// Merge the storage request (PVC size) with any configured cpu/memory so neither drops the other.
	volumeRequests := corev1.ResourceList{corev1.ResourceStorage: storageQuantity}
	for name, qty := range infraSpec.Config.Resources.Requests {
		volumeRequests[name] = qty
	}

	filerStorageQuantity := resource.MustParse(seaweedFilerStorageSize)

	labels := BuildWandbObjectStoreLabels(wandb)
	labels["app"] = SeaweedName(specName)

	seaweedCR := &seaweedv1.Seaweed{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SeaweedName(specName),
			Namespace: infraSpec.Namespace,
			Labels:    labels,
		},
		Spec: seaweedv1.SeaweedSpec{
			Image: SeaweedImage(mfst.Bucket["default"].Images["seaweedfs"], wandb.Spec.Global.ImageRegistry),
			TLS: &seaweedv1.TLSSpec{
				Enabled: infraSpec.SeaweedObjectStoreSpec.TlsEnabled,
			},
			Master: &seaweedv1.MasterSpec{
				Replicas:           1,
				DefaultReplication: &replication,
				VolumeSizeLimitMB:  &volumeSizeLimitMB,
				MetricsPort:        ptr.To(seaweedMasterMetricsPort),
				ComponentSpec: seaweedv1.ComponentSpec{
					Volumes:      seaweedWritableVolumes(),
					VolumeMounts: seaweedWritableVolumeMounts(),
				},
			},
			Volume: &seaweedv1.VolumeSpec{
				Replicas: infraSpec.Replicas,
				VolumeServerConfig: seaweedv1.VolumeServerConfig{
					MetricsPort: ptr.To(seaweedVolumeMetricsPort),
					// 0 lets each server fill its whole disk (disk / volumeSizeLimitMB volumes) instead of a low fixed cap.
					MaxVolumeCounts: ptr.To(int32(0)),
					ComponentSpec: seaweedv1.ComponentSpec{
						Volumes:      seaweedWritableVolumes(),
						VolumeMounts: seaweedWritableVolumeMounts(),
					},
					// Operator sizes the data PVC from Requests[storage] — a persistent disk, not ephemeral.
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: volumeRequests,
						Limits:   infraSpec.Config.Resources.Limits,
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
				Port:       ptr.To(int32(80)),
				DomainName: nil,
			},
			Filer: &seaweedv1.FilerSpec{
				Replicas:    1,
				MetricsPort: ptr.To(seaweedFilerMetricsPort),
				Config:      ptr.To("[leveldb2]\nenabled = true\ndir = \"" + seaweedFilerDataMountPath + "\""),
				ComponentSpec: seaweedv1.ComponentSpec{
					Volumes:      seaweedWritableVolumes(),
					VolumeMounts: seaweedWritableVolumeMounts(),
				},
				Persistence: &seaweedv1.PersistenceSpec{
					Enabled:   true,
					MountPath: ptr.To(seaweedFilerDataMountPath),
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: filerStorageQuantity,
						},
					},
				},
			},
			Affinity:    wandb.GetAffinity(infraSpec.ManagedInfraSpec),
			Tolerations: *wandb.GetTolerations(infraSpec.ManagedInfraSpec),
		},
	}

	if err := ctrl.SetControllerReference(wandb, seaweedCR, scheme); err != nil {
		log.Error("failed to set owner reference on Seaweed CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return seaweedCR, nil
}

// seaweedReplication builds the SeaweedFS replication code from the neutral copy
// count, clamped to the data-node count so we never request more copies than servers.
func seaweedReplication(copies, replicas int32) string {
	// Unset copies keeps the legacy behavior: one extra copy once there is more than one server.
	if copies <= 0 {
		if replicas > 1 {
			return "001"
		}
		return "000"
	}
	// Never request more copies than there are other servers to hold them.
	maxCopies := replicas - 1
	if maxCopies < 0 {
		maxCopies = 0
	}
	if copies > maxCopies {
		copies = maxCopies
	}
	return fmt.Sprintf("00%d", copies)
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
