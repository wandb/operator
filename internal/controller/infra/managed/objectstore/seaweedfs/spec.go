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

	// Filer holds only the leveldb2 path index; its size scales with object count,
	// not total data. 20Gi is a safe default for large counts; override with
	// FilerStorageSize when a deployment needs more.
	seaweedFilerStorageSize = "20Gi"
)

const (
	seaweedMasterMetricsPort int32 = 9091
	seaweedVolumeMetricsPort int32 = 9092
	seaweedFilerMetricsPort  int32 = 9093
	seaweedVolumeSizeLimitMB int64 = 1024
)

func volumeLayout(storageQuantity resource.Quantity) (int32, int32) {
	storageMB := storageQuantity.Value() / (1024 * 1024)
	volumeSizeMB := min(seaweedVolumeSizeLimitMB, storageMB/2)
	if volumeSizeMB < 1 {
		volumeSizeMB = 1
	}

	maxVolumeCount := storageMB/volumeSizeMB - 1
	if maxVolumeCount < 1 {
		maxVolumeCount = 1
	}

	return int32(volumeSizeMB), int32(maxVolumeCount)
}

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
	infraSpec *apiv2.ManagedObjectStoreSpec,
	scheme *runtime.Scheme,
	mfst manifest.Manifest,
) (*seaweedv1.Seaweed, error) {
	_, log := logx.WithSlog(ctx, logx.ObjectStore)
	if infraSpec == nil {
		return nil, nil
	}

	specName := infraSpec.Name

	storageQuantity, err := resource.ParseQuantity(infraSpec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", infraSpec.StorageSize, err)
	}

	replication := seaweedReplication(infraSpec.Copies, infraSpec.Replicas)

	volumeSizeLimitMB, maxVolumeCount := volumeLayout(storageQuantity)

	// Merge the storage request (PVC size) with any configured cpu/memory so neither drops the other.
	volumeRequests := corev1.ResourceList{corev1.ResourceStorage: storageQuantity}
	for name, qty := range infraSpec.Config.Resources.Requests {
		volumeRequests[name] = qty
	}

	filerStorageSize := seaweedFilerStorageSize
	if infraSpec.SeaweedObjectStoreSpec.FilerStorageSize != "" {
		filerStorageSize = infraSpec.SeaweedObjectStoreSpec.FilerStorageSize
	}
	filerStorageQuantity, err := resource.ParseQuantity(filerStorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid filer storage size %q: %w", filerStorageSize, err)
	}

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
					ExtraArgs:    []string{"-ip.bind=0.0.0.0"},
				},
			},
			Volume: &seaweedv1.VolumeSpec{
				Replicas: infraSpec.Replicas,
				VolumeServerConfig: seaweedv1.VolumeServerConfig{
					MetricsPort:     ptr.To(seaweedVolumeMetricsPort),
					MaxVolumeCounts: ptr.To(maxVolumeCount),
					ComponentSpec: seaweedv1.ComponentSpec{
						Volumes:      seaweedWritableVolumes(),
						VolumeMounts: seaweedWritableVolumeMounts(),
						ExtraArgs:    []string{"-ip.bind=0.0.0.0"},
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
					Env: []corev1.EnvVar{{
						// W&B presigns S3 URLs against the in-cluster endpoint and
						// rewrites the host for external clients without re-signing;
						// pin signature verification to that endpoint so presigned
						// requests arriving through an ingress proxy (whose
						// Host/X-Forwarded-Host is the external hostname) validate.
						Name:  "S3_EXTERNAL_URL",
						Value: s3ExternalURL(specName, infraSpec.Namespace, infraSpec.SeaweedObjectStoreSpec.TlsEnabled),
					}},
				},
				ResourceRequirements: corev1.ResourceRequirements{},
				Replicas:             1,
				ConfigSecret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ConfigName(specName),
					},
					Key: "config.json",
				},
				DomainName: nil,
			},
			Filer: &seaweedv1.FilerSpec{
				Replicas:    1,
				MetricsPort: ptr.To(seaweedFilerMetricsPort),
				Config:      ptr.To("[leveldb2]\nenabled = true\ndir = \"" + seaweedFilerDataMountPath + "\""),
				ComponentSpec: seaweedv1.ComponentSpec{
					Volumes:      seaweedWritableVolumes(),
					VolumeMounts: seaweedWritableVolumeMounts(),
					ExtraArgs:    []string{"-ip.bind=0.0.0.0"},
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
