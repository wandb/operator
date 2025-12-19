package v2

import (
	"context"
	"crypto/sha256"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/clickhouse/altinity"
	"github.com/wandb/operator/internal/controller/translator"
	chiv2 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func ToWBClickHouseStatus(ctx context.Context, status translator.ClickHouseStatus) apiv2.WBClickHouseStatus {
	return apiv2.WBClickHouseStatus{
		Ready:          status.Ready,
		State:          status.State,
		Conditions:     status.Conditions,
		LastReconciled: metav1.Now(),
		Connection: apiv2.WBInfraConnection{
			URL: status.Connection.URL,
		},
	}
}

// ToClickHouseVendorSpec converts a WBClickHouseSpec to a ClickHouseInstallation CR.
// This function translates the high-level ClickHouse spec into the vendor-specific
// ClickHouseInstallation format used by the Altinity operator.
func ToClickHouseVendorSpec(
	ctx context.Context,
	spec apiv2.WBClickHouseSpec,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*chiv2.ClickHouseInstallation, error) {
	log := ctrl.LoggerFrom(ctx)

	if !spec.Enabled {
		return nil, nil
	}

	nsnBuilder := altinity.CreateNsNameBuilder(types.NamespacedName{
		Namespace: spec.Namespace, Name: spec.Name,
	})

	// Parse storage quantity
	storageQuantity := resource.MustParse(spec.StorageSize)

	// Create user settings with password
	passwordSha256 := fmt.Sprintf("%x", sha256.Sum256([]byte(altinity.ClickHousePassword)))
	settings := chiv2.NewSettings()
	settings.Set(
		fmt.Sprintf("%s/password_sha256_hex", altinity.ClickHouseUser),
		chiv2.NewSettingScalar(passwordSha256),
	)
	settings.Set(
		fmt.Sprintf("%s/networks/ip", altinity.ClickHouseUser),
		chiv2.NewSettingScalar("::/0"),
	)
	settings.Set(
		fmt.Sprintf("%s/allow_databases/database", altinity.ClickHouseUser),
		chiv2.NewSettingVector([]string{altinity.ClickHouseDatabase, "db_management"}),
	)

	// Build ClickHouseInstallation spec
	chi := &chiv2.ClickHouseInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.InstallationName(),
			Namespace: nsnBuilder.Namespace(),
			Labels: map[string]string{
				"app": altinity.CHIName,
			},
		},
		Spec: chiv2.ChiSpec{
			Configuration: &chiv2.Configuration{
				Clusters: []*chiv2.Cluster{
					{
						Name: nsnBuilder.ClusterName(),
						Layout: &chiv2.ChiClusterLayout{
							ShardsCount:   altinity.ShardsCount,
							ReplicasCount: int(spec.Replicas),
						},
					},
				},
				Users: settings,
			},
			Defaults: &chiv2.Defaults{
				Templates: &chiv2.TemplatesList{
					DataVolumeClaimTemplate: nsnBuilder.VolumeTemplateName(),
				},
			},
			Templates: &chiv2.Templates{
				VolumeClaimTemplates: []chiv2.VolumeClaimTemplate{
					{
						Name: nsnBuilder.VolumeTemplateName(),
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

	// Add pod template with resources if specified
	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
		chi.Spec.Templates.PodTemplates = []chiv2.PodTemplate{
			{
				//Name: "default-pod",
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "clickhouse",
							Resources: corev1.ResourceRequirements{
								Requests: spec.Config.Resources.Requests,
								Limits:   spec.Config.Resources.Limits,
							},
						},
					},
				},
			},
		}
		//chi.Spec.Defaults.Templates.PodTemplate = "default-pod"
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, chi, scheme); err != nil {
		log.Error(err, "failed to set owner reference on CHI CR")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return chi, nil
}
