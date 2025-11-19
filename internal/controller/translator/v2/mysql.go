package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysql/percona"
	"github.com/wandb/operator/internal/controller/translator/common"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/defaults"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// BuildMySQLConfig will create a new common.MySQLConfig with defaultConfig applied if not
// present in actual. It should *never* be saved into the CR!
func BuildMySQLConfig(actual apiv2.WBMySQLSpec, defaultConfig common.MySQLConfig) (common.MySQLConfig, error) {
	mysqlConfig := TranslateMySQLSpec(actual)

	mysqlConfig.StorageSize = utils.CoalesceQuantity(mysqlConfig.StorageSize, defaultConfig.StorageSize)
	mysqlConfig.Namespace = utils.Coalesce(mysqlConfig.Namespace, defaultConfig.Namespace)
	mysqlConfig.Resources = utils.Resources(mysqlConfig.Resources, defaultConfig.Resources)

	mysqlConfig.Enabled = actual.Enabled
	mysqlConfig.Replicas = actual.Replicas

	return mysqlConfig, nil
}

func TranslateMySQLSpec(spec apiv2.WBMySQLSpec) common.MySQLConfig {
	config := common.MySQLConfig{
		Enabled:     spec.Enabled,
		Namespace:   spec.Namespace,
		StorageSize: spec.StorageSize,
		Replicas:    spec.Replicas,
	}
	if spec.Config != nil {
		config.Resources = spec.Config.Resources
	}

	return config
}

func ExtractMySQLStatus(ctx context.Context, results *common.Results) apiv2.WBMySQLStatus {
	return TranslateMySQLStatus(
		ctx,
		common.ExtractMySQLStatus(ctx, results),
	)
}

func TranslateMySQLStatus(ctx context.Context, m common.MySQLStatus) apiv2.WBMySQLStatus {
	var result apiv2.WBMySQLStatus
	var details []apiv2.WBStatusDetail

	for _, err := range m.Errors {
		details = append(details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.Code(),
			Message: err.Reason(),
		})
	}

	for _, detail := range m.Details {
		state := translateMySQLStatusCode(detail.Code())
		details = append(details, apiv2.WBStatusDetail{
			State:   state,
			Code:    detail.Code(),
			Message: detail.Message(),
		})
	}

	result.Connection = apiv2.WBMySQLConnection{
		MySQLHost: m.Connection.Host,
		MySQLPort: m.Connection.Port,
		MySQLUser: m.Connection.User,
	}

	result.Ready = m.Ready
	result.Details = details
	result.State = computeOverallState(details, m.Ready)
	result.LastReconciled = metav1.Now()

	return result
}

func translateMySQLStatusCode(code string) apiv2.WBStateType {
	switch code {
	case string(common.MySQLCreatedCode):
		return apiv2.WBStateUpdating
	case string(common.MySQLUpdatedCode):
		return apiv2.WBStateUpdating
	case string(common.MySQLDeletedCode):
		return apiv2.WBStateDeleting
	case string(common.MySQLConnectionCode):
		return apiv2.WBStateReady
	default:
		return apiv2.WBStateUnknown
	}
}

func (i *InfraConfigBuilder) AddMySQLConfig(actual apiv2.WBMySQLSpec) *InfraConfigBuilder {
	var err error
	var size common.Size
	var defaultConfig common.MySQLConfig
	var mergedConfig common.MySQLConfig

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = defaults.BuildMySQLDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	mergedConfig, err = BuildMySQLConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedMySQL = mergedConfig
	return i
}

// ToMySQLVendorSpec converts a WBMySQLSpec to a PerconaXtraDBCluster CR.
// This function translates the high-level MySQL spec into the vendor-specific
// PerconaXtraDBCluster format used by the Percona operator.
func ToMySQLVendorSpec(
	ctx context.Context,
	spec apiv2.WBMySQLSpec,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*pxcv1.PerconaXtraDBCluster, error) {
	log := ctrl.LoggerFrom(ctx)

	// Parse storage quantity
	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

	// Determine configuration based on replica count
	proxySQLEnabled := spec.Replicas > 1
	tlsEnabled := spec.Replicas > 1
	allowUnsafePXCSize := spec.Replicas == 1
	allowUnsafeProxySize := spec.Replicas == 1

	// Select PXC image based on mode (dev vs prod)
	pxcImage := common.DevPXCImage
	if spec.Replicas > 1 {
		pxcImage = common.ProdPXCImage
	}

	// Build PXC spec
	pxc := &pxcv1.PerconaXtraDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      percona.PXCName,
			Namespace: spec.Namespace,
			Labels: map[string]string{
				"app": percona.PXCName,
			},
		},
		Spec: pxcv1.PerconaXtraDBClusterSpec{
			CRVersion: common.PXCCRVersion,
			Unsafe: pxcv1.UnsafeFlags{
				PXCSize:   allowUnsafePXCSize,
				TLS:       !tlsEnabled,
				ProxySize: allowUnsafeProxySize,
			},
			PXC: &pxcv1.PXCSpec{
				PodSpec: &pxcv1.PodSpec{
					Size:  spec.Replicas,
					Image: pxcImage,
					VolumeSpec: &pxcv1.VolumeSpec{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: storageQuantity,
								},
							},
						},
					},
				},
			},
			TLS: &pxcv1.TLSSpec{
				Enabled: &tlsEnabled,
			},
		},
	}

	// Add resources if specified
	if spec.Config != nil && (len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0) {
		pxc.Spec.PXC.PodSpec.Resources = corev1.ResourceRequirements{
			Requests: spec.Config.Resources.Requests,
			Limits:   spec.Config.Resources.Limits,
		}
	}

	// Configure ProxySQL for HA mode (replicas > 1)
	if proxySQLEnabled {
		proxySQLReplicas := int32(3)
		if spec.Replicas < 3 {
			proxySQLReplicas = spec.Replicas
		}
		pxc.Spec.ProxySQL = &pxcv1.ProxySQLSpec{
			PodSpec: pxcv1.PodSpec{
				Enabled: true,
				Size:    proxySQLReplicas,
				Image:   common.ProxySQLImage,
				VolumeSpec: &pxcv1.VolumeSpec{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}
	} else {
		// Explicitly disable HAProxy for dev mode
		pxc.Spec.HAProxy = &pxcv1.HAProxySpec{
			PodSpec: pxcv1.PodSpec{
				Enabled: false,
			},
		}
	}

	// Configure LogCollector for dev mode
	if spec.Replicas == 1 {
		pxc.Spec.LogCollector = &pxcv1.LogCollectorSpec{
			Enabled: true,
			Image:   common.LogCollectorImg,
		}
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, pxc, scheme); err != nil {
		log.Error(err, "failed to set owner reference on PXC CR")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return pxc, nil
}
