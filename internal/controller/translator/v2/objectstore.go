package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/managed/objectstore/seaweedfs"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

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

	seaweedCR := &seaweedv1.Seaweed{
		ObjectMeta: metav1.ObjectMeta{
			Name:      seaweedfs.SeaweedName(specName),
			Namespace: infraSpec.Namespace,
			Labels: map[string]string{
				"app": seaweedfs.SeaweedName(specName),
			},
		},
		Spec: seaweedv1.SeaweedSpec{
			Image: translator.SeaweedImage,
			Master: &seaweedv1.MasterSpec{
				Replicas:           1,
				DefaultReplication: &replication,
				VolumeSizeLimitMB:  &volumeSizeLimitMB,
			},
			Volume: &seaweedv1.VolumeSpec{
				Replicas: infraSpec.Replicas,
			},
			Filer: &seaweedv1.FilerSpec{
				Replicas: 1,
			},
			S3: &seaweedv1.S3GatewaySpec{
				Replicas: 1,
				Port:     ptr.To(int32(9000)),
				ConfigSecret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: seaweedfs.ConfigName(specName),
					},
					Key: "config.json",
				},
				IAM: true,
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
) (seaweedfs.SeaweedS3Config, error) {
	return seaweedfs.SeaweedS3Config{
		AccessKey: spec.Config.AccessKey,
	}, nil
}

func BuildWandbObjectStoreLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return BuildWandbLabels(wandb, translator.ObjectStoreModuleName)
}

func ToObjectStoreOnDeleteRule(wandb *apiv2.WeightsAndBiases, retentionPolicy apiv2.RetentionPolicy) translator.OnDeleteRule {
	return ToOnDeleteRule(wandb, retentionPolicy, translator.ObjectStoreModuleName)
}
