package keeper

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	chkv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse-keeper.altinity.com/v1"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ToKeeperVendorSpec builds the ClickHouseKeeperInstallation CR that coordinates
// ReplicatedMergeTree replication for the managed ClickHouse installation.
func ToKeeperVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*chkv1.ClickHouseKeeperInstallation, error) {
	_, log := logx.WithSlog(ctx, logx.ClickHouse)
	spec := wandb.Spec.ClickHouse.ManagedClickHouse
	if spec == nil {
		return nil, nil
	}

	// Keeper sizing comes from the server manifest (clickhouseKeeper) or CR; there
	// are no operator-side defaults, so fail loudly if the storage size is missing.
	storageQuantity, err := resource.ParseQuantity(spec.Keeper.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid keeper storageSize %q (expected from the server manifest's clickhouseKeeper sizing): %w", spec.Keeper.StorageSize, err)
	}

	labels := common.BuildWandbLabels(wandb, KeeperModuleName)

	podSpec := corev1.PodSpec{
		SecurityContext: keeperPodSecurityContext(),
		Affinity:        wandb.GetAffinity(spec.ManagedInfraSpec),
		Tolerations:     *wandb.GetTolerations(spec.ManagedInfraSpec),
		Containers: []corev1.Container{
			{
				Name:            keeperContainerName,
				Image:           KeeperImage,
				SecurityContext: keeperContainerSecurityContext(),
			},
		},
	}
	if len(spec.Keeper.Config.Resources.Requests) > 0 || len(spec.Keeper.Config.Resources.Limits) > 0 {
		podSpec.Containers[0].Resources = corev1.ResourceRequirements{
			Requests: spec.Keeper.Config.Resources.Requests,
			Limits:   spec.Keeper.Config.Resources.Limits,
		}
	}

	chk := &chkv1.ClickHouseKeeperInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      InstallationName(spec.Name),
			Namespace: spec.Namespace,
			Labels:    labels,
		},
		Spec: chkv1.ChkSpec{
			Configuration: &chkv1.Configuration{
				Clusters: []*chkv1.Cluster{
					{
						Name: ClusterName,
						Layout: &chkv1.ChkClusterLayout{
							ReplicasCount: int(spec.Keeper.Replicas),
						},
					},
				},
			},
			Defaults: &chiv1.Defaults{
				Templates: &chiv1.TemplatesList{
					PodTemplate:             podTemplateName,
					DataVolumeClaimTemplate: volumeTemplateName,
				},
			},
			Templates: &chiv1.Templates{
				PodTemplates: []chiv1.PodTemplate{
					{
						Name:       podTemplateName,
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec:       podSpec,
					},
				},
				VolumeClaimTemplates: []chiv1.VolumeClaimTemplate{
					{
						Name:       volumeTemplateName,
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceStorage: storageQuantity},
							},
						},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(wandb, chk, scheme); err != nil {
		log.Error("failed to set owner reference on CHK CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return chk, nil
}

// BuildWandbKeeperLabels returns the standard W&B labels for Keeper resources.
func BuildWandbKeeperLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return common.BuildWandbLabels(wandb, KeeperModuleName)
}

func keeperPodSecurityContext() *corev1.PodSecurityContext {
	if utils.IsOpenShift() {
		return &corev1.PodSecurityContext{
			RunAsNonRoot:   ptr.To(true),
			SeccompProfile: runtimeDefaultSeccompProfile(),
		}
	}
	return &corev1.PodSecurityContext{
		RunAsUser:      ptr.To(keeperRunAsUser),
		RunAsGroup:     ptr.To(keeperRunAsGroup),
		RunAsNonRoot:   ptr.To(true),
		FSGroup:        ptr.To(keeperFSGroup),
		SeccompProfile: runtimeDefaultSeccompProfile(),
	}
}

func keeperContainerSecurityContext() *corev1.SecurityContext {
	sc := &corev1.SecurityContext{
		RunAsNonRoot:             ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		SeccompProfile:           runtimeDefaultSeccompProfile(),
	}
	if !utils.IsOpenShift() {
		sc.RunAsUser = ptr.To(keeperRunAsUser)
		sc.RunAsGroup = ptr.To(keeperRunAsGroup)
	}
	return sc
}

func runtimeDefaultSeccompProfile() *corev1.SeccompProfile {
	return &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
}
