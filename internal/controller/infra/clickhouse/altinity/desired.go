package altinity

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/wandb/operator/internal/model"
	v2 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// buildDesiredCHI creates a ClickHouseInstallation CR based on the provided config.
// Handles both dev (1 replica) and small (3 replicas) configurations.
func buildDesiredCHI(
	ctx context.Context,
	clickhouseConfig model.ClickHouseConfig,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*v2.ClickHouseInstallation, *model.Results) {
	log := ctrl.LoggerFrom(ctx)
	results := model.InitResults()

	// Parse storage quantity
	storageQuantity, err := resource.ParseQuantity(clickhouseConfig.StorageSize)
	if err != nil {
		log.Error(err, "invalid storage size", "storageSize", clickhouseConfig.StorageSize)
		results.AddErrors(model.NewClickHouseError(model.ClickHouseErrFailedToCreateCode, fmt.Sprintf("invalid storage size: %v", err)))
		return nil, results
	}

	// Create user settings with password
	passwordSha256 := fmt.Sprintf("%x", sha256.Sum256([]byte(ClickHousePassword)))
	settings := v2.NewSettings()
	settings.Set(
		fmt.Sprintf("%s/password_sha256_hex", ClickHouseUser),
		v2.NewSettingScalar(passwordSha256),
	)
	settings.Set(
		fmt.Sprintf("%s/networks/ip", ClickHouseUser),
		v2.NewSettingScalar("::/0"),
	)

	// Build ClickHouseInstallation spec
	chi := &v2.ClickHouseInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CHIName,
			Namespace: clickhouseConfig.Namespace,
			Labels: map[string]string{
				"app": CHIName,
			},
		},
		Spec: v2.ChiSpec{
			Configuration: &v2.Configuration{
				Clusters: []*v2.Cluster{
					{
						Name: ClusterName,
						Layout: &v2.ChiClusterLayout{
							ShardsCount:   ShardsCount,
							ReplicasCount: int(clickhouseConfig.Replicas),
						},
					},
				},
				Users: settings,
			},
			Defaults: &v2.Defaults{
				Templates: &v2.TemplatesList{
					DataVolumeClaimTemplate: VolumeTemplateName,
				},
			},
			Templates: &v2.Templates{
				VolumeClaimTemplates: []v2.VolumeClaimTemplate{
					{
						Name: VolumeTemplateName,
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
	if len(clickhouseConfig.Resources.Requests) > 0 || len(clickhouseConfig.Resources.Limits) > 0 {
		chi.Spec.Templates.PodTemplates = []v2.PodTemplate{
			{
				Name: "default-pod",
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "clickhouse",
							Resources: corev1.ResourceRequirements{
								Requests: clickhouseConfig.Resources.Requests,
								Limits:   clickhouseConfig.Resources.Limits,
							},
						},
					},
				},
			},
		}
		chi.Spec.Defaults.Templates.PodTemplate = "default-pod"
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, chi, scheme); err != nil {
		log.Error(err, "failed to set owner reference on CHI CR")
		results.AddErrors(model.NewClickHouseError(model.ClickHouseErrFailedToCreateCode, fmt.Sprintf("failed to set owner reference: %v", err)))
		return nil, results
	}

	return chi, results
}
