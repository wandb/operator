package percona

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/translator/common"
	"github.com/wandb/operator/internal/defaults"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// buildDesiredPXC creates a PerconaXtraDBCluster CR based on the provided config.
// Handles both dev (standalone) and small (HA with ProxySQL) configurations.
func buildDesiredPXC(
	ctx context.Context,
	mysqlConfig common.MySQLConfig,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*pxcv1.PerconaXtraDBCluster, *common.Results) {
	log := ctrl.LoggerFrom(ctx)
	results := common.InitResults()

	// Parse storage quantity
	storageQuantity, err := resource.ParseQuantity(mysqlConfig.StorageSize)
	if err != nil {
		log.Error(err, "invalid storage size", "storageSize", mysqlConfig.StorageSize)
		results.AddErrors(common.NewMySQLError(common.MySQLErrFailedToCreateCode, fmt.Sprintf("invalid storage size: %v", err)))
		return nil, results
	}

	// Build PXC spec
	pxc := &pxcv1.PerconaXtraDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PXCName,
			Namespace: mysqlConfig.Namespace,
			Labels: map[string]string{
				"app": PXCName,
			},
		},
		Spec: pxcv1.PerconaXtraDBClusterSpec{
			CRVersion: defaults.CRVersion,
			Unsafe: pxcv1.UnsafeFlags{
				PXCSize:   mysqlConfig.AllowUnsafePXCSize,
				TLS:       !mysqlConfig.TLSEnabled, // Unsafe TLS flag is inverse of TLS enabled
				ProxySize: mysqlConfig.AllowUnsafeProxySize,
			},
			PXC: &pxcv1.PXCSpec{
				PodSpec: &pxcv1.PodSpec{
					Size:  int32(mysqlConfig.Replicas),
					Image: mysqlConfig.PXCImage,
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
				Enabled: &mysqlConfig.TLSEnabled,
			},
		},
	}

	// Add resources if specified
	if len(mysqlConfig.Resources.Requests) > 0 || len(mysqlConfig.Resources.Limits) > 0 {
		pxc.Spec.PXC.PodSpec.Resources = corev1.ResourceRequirements{
			Requests: mysqlConfig.Resources.Requests,
			Limits:   mysqlConfig.Resources.Limits,
		}
	}

	// Configure ProxySQL for HA mode
	if mysqlConfig.ProxySQLEnabled {
		pxc.Spec.ProxySQL = &pxcv1.ProxySQLSpec{
			PodSpec: pxcv1.PodSpec{
				Enabled: true,
				Size:    int32(mysqlConfig.ProxySQLReplicas),
				Image:   mysqlConfig.ProxySQLImage,
				VolumeSpec: &pxcv1.VolumeSpec{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}
	} else {
		// Explicitly disable HAProxy for dev mode (we don't use HAProxy)
		pxc.Spec.HAProxy = &pxcv1.HAProxySpec{
			PodSpec: pxcv1.PodSpec{
				Enabled: false,
			},
		}
	}

	// Configure LogCollector
	if mysqlConfig.LogCollectorEnabled {
		pxc.Spec.LogCollector = &pxcv1.LogCollectorSpec{
			Enabled: true,
			Image:   mysqlConfig.LogCollectorImage,
		}
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, pxc, scheme); err != nil {
		log.Error(err, "failed to set owner reference on PXC CR")
		results.AddErrors(common.NewMySQLError(common.MySQLErrFailedToCreateCode, fmt.Sprintf("failed to set owner reference: %v", err)))
		return nil, results
	}

	return pxc, results
}
