package v2

import (
	"context"
	"crypto/sha256"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/clickhouse/altinity"
	"github.com/wandb/operator/internal/controller/translator/common"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/defaults"
	chiv2 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// BuildClickHouseConfig will create a new WBClickHouseSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildClickHouseConfig(actual apiv2.WBClickHouseSpec, defaultConfig common.ClickHouseConfig) (common.ClickHouseConfig, error) {
	clickhouseConfig := TranslateClickHouseSpec(actual)

	clickhouseConfig.StorageSize = utils.CoalesceQuantity(clickhouseConfig.StorageSize, defaultConfig.StorageSize)
	clickhouseConfig.Namespace = utils.Coalesce(clickhouseConfig.Namespace, defaultConfig.Namespace)
	clickhouseConfig.Version = utils.Coalesce(clickhouseConfig.Version, defaultConfig.Version)
	clickhouseConfig.Resources = utils.Resources(clickhouseConfig.Resources, defaultConfig.Resources)

	clickhouseConfig.Enabled = actual.Enabled
	clickhouseConfig.Replicas = actual.Replicas

	return clickhouseConfig, nil
}

func TranslateClickHouseSpec(spec apiv2.WBClickHouseSpec) common.ClickHouseConfig {
	config := common.ClickHouseConfig{
		Enabled:     spec.Enabled,
		Namespace:   spec.Namespace,
		StorageSize: spec.StorageSize,
		Replicas:    spec.Replicas,
		Version:     spec.Version,
	}
	if spec.Config != nil {
		config.Resources = spec.Config.Resources
	}

	return config
}

func ExtractClickHouseStatus(ctx context.Context, results *common.Results) apiv2.WBClickHouseStatus {
	return TranslateClickHouseStatus(
		ctx,
		common.ExtractClickHouseStatus(ctx, results),
	)
}

func TranslateClickHouseStatus(ctx context.Context, m common.ClickHouseStatus) apiv2.WBClickHouseStatus {
	var result apiv2.WBClickHouseStatus
	var details []apiv2.WBStatusDetail

	for _, err := range m.Errors {
		details = append(details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.Code(),
			Message: err.Reason(),
		})
	}

	for _, detail := range m.Details {
		state := translateClickHouseStatusCode(detail.Code())
		details = append(details, apiv2.WBStatusDetail{
			State:   state,
			Code:    detail.Code(),
			Message: detail.Message(),
		})
	}

	result.Connection = apiv2.WBClickHouseConnection{
		ClickHouseHost: m.Connection.Host,
		ClickHousePort: m.Connection.Port,
		ClickHouseUser: m.Connection.User,
	}

	result.Ready = m.Ready
	result.Details = details
	result.State = computeOverallState(details, m.Ready)
	result.LastReconciled = metav1.Now()

	return result
}

func translateClickHouseStatusCode(code string) apiv2.WBStateType {
	switch code {
	case string(common.ClickHouseCreatedCode):
		return apiv2.WBStateUpdating
	case string(common.ClickHouseUpdatedCode):
		return apiv2.WBStateUpdating
	case string(common.ClickHouseDeletedCode):
		return apiv2.WBStateDeleting
	case string(common.ClickHouseConnectionCode):
		return apiv2.WBStateReady
	default:
		return apiv2.WBStateUnknown
	}
}

func (i *InfraConfigBuilder) AddClickHouseConfig(actual apiv2.WBClickHouseSpec) *InfraConfigBuilder {
	var err error
	var size common.Size
	var defaultConfig common.ClickHouseConfig
	var mergedConfig common.ClickHouseConfig

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = defaults.BuildClickHouseDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	mergedConfig, err = BuildClickHouseConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedClickHouse = mergedConfig
	return i
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

	// Parse storage quantity
	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

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

	// Build ClickHouseInstallation spec
	chi := &chiv2.ClickHouseInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      altinity.CHIName,
			Namespace: spec.Namespace,
			Labels: map[string]string{
				"app": altinity.CHIName,
			},
		},
		Spec: chiv2.ChiSpec{
			Configuration: &chiv2.Configuration{
				Clusters: []*chiv2.Cluster{
					{
						Name: altinity.ClusterName,
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
					DataVolumeClaimTemplate: altinity.VolumeTemplateName,
				},
			},
			Templates: &chiv2.Templates{
				VolumeClaimTemplates: []chiv2.VolumeClaimTemplate{
					{
						Name: altinity.VolumeTemplateName,
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
	if spec.Config != nil && (len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0) {
		chi.Spec.Templates.PodTemplates = []chiv2.PodTemplate{
			{
				Name: "default-pod",
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
		chi.Spec.Defaults.Templates.PodTemplate = "default-pod"
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, chi, scheme); err != nil {
		log.Error(err, "failed to set owner reference on CHI CR")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return chi, nil
}
