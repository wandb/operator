package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysql/percona"
	"github.com/wandb/operator/internal/controller/translator"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func ToWBMysqlStatus(ctx context.Context, status translator.MysqlStatus) apiv2.WBMySQLStatus {
	return apiv2.WBMySQLStatus{
		Ready:          status.Ready,
		State:          status.State,
		Conditions:     status.Conditions,
		LastReconciled: metav1.Now(),
		Connection: apiv2.WBInfraConnection{
			URL: status.Connection.URL,
		},
	}
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

	if !spec.Enabled {
		return nil, nil
	}

	specName := spec.Name
	nsNameBldr := percona.CreateNsNameBuilder(types.NamespacedName{
		Name:      specName,
		Namespace: spec.Namespace,
	})

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
	pxcImage := translator.DevPXCImage
	if spec.Replicas > 1 {
		pxcImage = translator.ProdPXCImage
	}

	// Build PXC spec
	pxc := &pxcv1.PerconaXtraDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsNameBldr.ClusterName(),
			Namespace: spec.Namespace,
			Labels: map[string]string{
				"app": nsNameBldr.ClusterName(),
			},
		},
		Spec: pxcv1.PerconaXtraDBClusterSpec{
			CRVersion: translator.PXCCRVersion,
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
	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
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
				Image:   translator.ProxySQLImage,
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
			Image:   translator.LogCollectorImg,
		}
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, pxc, scheme); err != nil {
		log.Error(err, "failed to set owner reference on PXC CR")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return pxc, nil
}
