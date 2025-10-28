package wandb_v2

import (
	"context"
	"errors"

	miniov2 "github.com/wandb/operator/api/minio-operator-vendored/minio.min.io/v2"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *WeightsAndBiasesV2Reconciler) handleMinioHA(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) CtrlState {
	var err error
	var desiredMinio wandbMinioWrapper
	var actualMinio wandbMinioWrapper
	var reconciliation wandbMinioDoReconcile
	log := ctrl.LoggerFrom(ctx)
	namespacedName := minioNamespacedName(req)

	if !wandb.Spec.ObjStorage.Enabled {
		log.Info("ObjStorage not enabled, skipping")
		return CtrlContinue()
	}

	log.Info("Handling MinIO HA")

	if actualMinio, err = getActualMinio(ctx, r, namespacedName); err != nil {
		log.Error(err, "Failed to get actual MinIO HA resources")
		return CtrlError(err)
	}

	if ctrlState := actualMinio.maybeHandleDeletion(ctx, wandb, actualMinio, r); ctrlState.shouldExit(HandlerScope) {
		return ctrlState
	}

	if desiredMinio, err = getDesiredMinioHA(ctx, wandb, namespacedName, actualMinio); err != nil {
		log.Error(err, "Failed to get desired MinIO HA configuration")
		return CtrlError(err)
	}

	if reconciliation, err = computeMinioReconcileDrift(ctx, wandb, desiredMinio, actualMinio); err != nil {
		log.Error(err, "Failed to compute MinIO HA reconcile drift")
		return CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return CtrlContinue()
}

func getDesiredMinioHA(
	_ context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName, actual wandbMinioWrapper,
) (
	wandbMinioWrapper, error,
) {
	result := wandbMinioWrapper{
		installed:       false,
		obj:             nil,
		secretInstalled: false,
		secret:          nil,
	}

	if !wandb.Spec.ObjStorage.Enabled {
		return result, nil
	}

	result.installed = true

	storageSize := wandb.Spec.ObjStorage.StorageSize
	if storageSize == "" {
		storageSize = "10Gi"
	}

	storageQuantity, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return result, errors.New("invalid storage size: " + storageSize)
	}

	replicas := int32(3)

	configSecretName := namespacedName.Name + "-config"
	tenant := &miniov2.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels: map[string]string{
				"app": "wandb-minio",
			},
		},
		Spec: miniov2.TenantSpec{
			Image: "quay.io/minio/minio:latest",
			Configuration: &corev1.LocalObjectReference{
				Name: configSecretName,
			},
			Pools: []miniov2.Pool{
				{
					Name:             "pool-0",
					Servers:          replicas,
					VolumesPerServer: 4,
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
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

	wandbBackupSpec := wandb.Spec.ObjStorage.Backup
	if wandbBackupSpec.Enabled {
		if wandbBackupSpec.StorageType != apiv2.WBBackupStorageTypeFilesystem {
			return result, errors.New("only filesystem backup storage type is supported for MinIO")
		}
	}

	result.obj = tenant

	if actual.IsReady() {
		namespace := namespacedName.Namespace
		minioEndpoint := "https://minio." + namespace + ".svc.cluster.local:443"
		minioConfigSecretName := "wandb-minio-config"
		minioConfigSecretKey := "config.env"
		minioConfigMountPath := "/etc/minio/config.env"

		connectionSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wandb-minio-connection",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"MINIO_ENDPOINT":           minioEndpoint,
				"MINIO_CONFIG_SECRET_NAME": minioConfigSecretName,
				"MINIO_CONFIG_SECRET_KEY":  minioConfigSecretKey,
				"MINIO_CONFIG_MOUNT_PATH":  minioConfigMountPath,
			},
		}

		result.secret = connectionSecret
		result.secretInstalled = true
	}

	return result, nil
}
