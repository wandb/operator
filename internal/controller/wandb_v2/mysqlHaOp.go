package wandb_v2

import (
	"context"
	"errors"

	pxcv1 "github.com/wandb/operator/api/percona-operator-vendored/pxc/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *WeightsAndBiasesV2Reconciler) handlePerconaMysqlHA(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) CtrlState {
	var err error
	var desiredPercona wandbPerconaMysqlWrapper
	var actualPercona wandbPerconaMysqlWrapper
	var reconciliation wandbPerconaMysqlDoReconcile
	var namespacedName = perconaMysqlNamespacedName(req)
	log := ctrllog.FromContext(ctx)

	if actualPercona, err = actualPerconaMysql(ctx, r, namespacedName); err != nil {
		log.Error(err, "Failed to get actual Percona MySQL HA")
		return CtrlError(err)
	}

	if ctrlState := actualPercona.maybeHandleDeletion(ctx, wandb, actualPercona, r); ctrlState.shouldExit(HandlerScope) {
		return ctrlState
	}

	if desiredPercona, err = desiredPerconaMysqlHA(ctx, wandb, namespacedName, r, actualPercona); err != nil {
		log.Error(err, "Failed to compute desired Percona MySQL HA state")
		return CtrlError(err)
	}

	if reconciliation, err = computePerconaReconcileDrift(ctx, wandb, desiredPercona, actualPercona); err != nil {
		log.Error(err, "Failed to compute Percona MySQL HA reconciliation drift")
		return CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return CtrlContinue()
}

func desiredPerconaMysqlHA(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName, reconciler *WeightsAndBiasesV2Reconciler, actual wandbPerconaMysqlWrapper,
) (
	wandbPerconaMysqlWrapper, error,
) {
	result := wandbPerconaMysqlWrapper{
		installed:       false,
		obj:             nil,
		secretInstalled: false,
		secret:          nil,
	}

	if !wandb.Spec.Database.Enabled {
		return result, nil
	}

	result.installed = true

	storageSize := wandb.Spec.Database.StorageSize
	if storageSize == "" {
		storageSize = "20Gi"
	}

	storageQuantity, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return result, errors.New("invalid storage size: " + storageSize)
	}
	tlsEnabled := false
	wandbBackupSpec := wandb.Spec.Database.Backup

	pxc := &pxcv1.PerconaXtraDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
		},
		Spec: pxcv1.PerconaXtraDBClusterSpec{
			CRVersion: "1.18.0",
			Unsafe: pxcv1.UnsafeFlags{
				PXCSize:   false,
				TLS:       true,
				ProxySize: false,
			},
			PXC: &pxcv1.PXCSpec{
				PodSpec: &pxcv1.PodSpec{
					Size:  3,
					Image: "percona/percona-xtradb-cluster:8.0.43-34.1",
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
			HAProxy: &pxcv1.HAProxySpec{
				PodSpec: pxcv1.PodSpec{
					Enabled: true,
					Size:    3,
					Image:   "percona/percona-xtradb-cluster-operator:1.18.0-haproxy",
				},
			},
			LogCollector: &pxcv1.LogCollectorSpec{
				Enabled: true,
				Image:   "percona/pmm-client:2",
			},
		},
	}

	if wandbBackupSpec.Enabled {
		storageName := "default-backup"
		if wandbBackupSpec.StorageName != "" {
			storageName = wandbBackupSpec.StorageName
		}
		if wandbBackupSpec.StorageType != apiv2.WBBackupStorageTypeFilesystem {
			return result, errors.New("only filesystem backup storage type is supported for now for Percona XtraDB Cluster")
		}
		pxc.Spec.Backup = &pxcv1.PXCScheduledBackup{
			Image: "percona/percona-xtradb-cluster-operator:1.18.0-pxc8.0-backup",
			Storages: map[string]*pxcv1.BackupStorageSpec{
				storageName: {
					Type: pxcv1.BackupStorageFilesystem,
					Volume: &pxcv1.VolumeSpec{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
							AccessModes: wandbBackupSpec.Filesystem.AccessModes,
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: wandbBackupSpec.Filesystem.StorageSize,
								},
							},
						},
					},
				},
			},
		}
	}

	result.obj = pxc

	if actual.IsReady() {
		namespace := namespacedName.Namespace

		sourceSecretName := types.NamespacedName{
			Name:      namespacedName.Name + "-secrets",
			Namespace: namespace,
		}
		sourceSecret := &corev1.Secret{}
		if err := reconciler.Get(ctx, sourceSecretName, sourceSecret); err != nil {
			if machErrors.IsNotFound(err) {
				return result, nil
			}
			return result, err
		}

		mysqlPassword, ok := sourceSecret.Data["root"]
		if !ok {
			return result, errors.New("root key not found in " + sourceSecretName.Name)
		}

		mysqlHost := "wandb-mysql-haproxy." + namespace + ".svc.cluster.local"
		mysqlPort := "3306"
		mysqlUser := "root"

		connectionSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wandb-mysql-connection",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"MYSQL_HOST":     mysqlHost,
				"MYSQL_PORT":     mysqlPort,
				"MYSQL_USER":     mysqlUser,
				"MYSQL_PASSWORD": string(mysqlPassword),
			},
		}

		result.secret = connectionSecret
		result.secretInstalled = true
	}

	return result, nil
}
