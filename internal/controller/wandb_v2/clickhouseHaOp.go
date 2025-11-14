package wandb_v2

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	chiv1 "github.com/wandb/operator/api/altinity-clickhouse-vendored/clickhouse.altinity.com/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/ctrlqueue"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *WeightsAndBiasesV2Reconciler) handleClickHouseHA(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) ctrlqueue.CtrlState {
	var err error
	var desiredClickHouse wandbClickHouseWrapper
	var actualClickHouse wandbClickHouseWrapper
	var reconciliation wandbClickHouseDoReconcile
	log := ctrl.LoggerFrom(ctx)
	namespacedName := clickhouseNamespacedName(wandb)

	if !wandb.Spec.ClickHouse.Enabled {
		log.Info("ClickHouse not enabled, skipping")
		return ctrlqueue.CtrlContinue()
	}

	log.Info("Handling ClickHouse HA")

	if actualClickHouse, err = getActualClickHouse(ctx, r, namespacedName); err != nil {
		log.Error(err, "Failed to get actual ClickHouse HA resources")
		return ctrlqueue.CtrlError(err)
	}

	if ctrlState := actualClickHouse.maybeHandleDeletion(ctx, wandb, actualClickHouse, r); ctrlState.ShouldExit(ctrlqueue.PackageScope) {
		return ctrlState
	}

	if desiredClickHouse, err = getDesiredClickHouseHA(ctx, wandb, namespacedName, actualClickHouse); err != nil {
		log.Error(err, "Failed to get desired ClickHouse HA configuration")
		return ctrlqueue.CtrlError(err)
	}

	if reconciliation, err = computeClickHouseReconcileDrift(ctx, wandb, desiredClickHouse, actualClickHouse); err != nil {
		log.Error(err, "Failed to compute ClickHouse HA reconcile drift")
		return ctrlqueue.CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return ctrlqueue.CtrlContinue()
}

func getDesiredClickHouseHA(
	_ context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName, actual wandbClickHouseWrapper,
) (
	wandbClickHouseWrapper, error,
) {
	result := wandbClickHouseWrapper{
		installed:       false,
		obj:             nil,
		secretInstalled: false,
		secret:          nil,
	}

	if !wandb.Spec.ClickHouse.Enabled {
		return result, nil
	}

	result.installed = true

	storageSize := wandb.Spec.ClickHouse.StorageSize
	if storageSize == "" {
		storageSize = "10Gi"
	}

	storageQuantity, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return result, errors.New("invalid storage size: " + storageSize)
	}

	replicas := int32(3)

	canaryUsername := "test_user"
	canaryPassword := "test_password"
	passwordSha256 := fmt.Sprintf("%x", sha256.Sum256([]byte(canaryPassword)))
	settings := chiv1.NewSettings()
	settings.Set(
		fmt.Sprintf("%s/password_sha256_hex", canaryUsername),
		chiv1.NewSettingScalar(passwordSha256),
	)
	settings.Set(
		fmt.Sprintf("%s/networks/ip", canaryUsername),
		chiv1.NewSettingScalar("::/0"),
	)

	chi := &chiv1.ClickHouseInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels: map[string]string{
				"app": "wandb-clickhouse",
			},
		},
		Spec: chiv1.ChiSpec{
			Configuration: &chiv1.Configuration{
				Clusters: []*chiv1.Cluster{
					{
						Name: "cluster",
						Layout: &chiv1.ChiClusterLayout{
							ShardsCount:   1,
							ReplicasCount: int(replicas),
						},
					},
				},
				Users: settings,
			},
			Defaults: &chiv1.Defaults{
				Templates: &chiv1.TemplatesList{
					DataVolumeClaimTemplate: "default-volume",
				},
			},
			Templates: &chiv1.Templates{
				VolumeClaimTemplates: []chiv1.VolumeClaimTemplate{
					{
						Name: "default-volume",
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

	wandbBackupSpec := wandb.Spec.ClickHouse.Backup
	if wandbBackupSpec.Enabled {
		if wandbBackupSpec.StorageType != apiv2.WBBackupStorageTypeFilesystem {
			return result, errors.New("only filesystem backup storage type is supported for ClickHouse")
		}
	}

	result.obj = chi

	if actual.IsReady() {
		namespace := namespacedName.Namespace
		clickhouseHost := "clickhouse-wandb-clickhouse." + namespace + ".svc.cluster.local"
		clickhousePort := "9000"
		clickhouseHTTPPort := "8123"

		connectionSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wandb-clickhouse-connection",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"CLICKHOUSE_CANARY_USERNAME": canaryUsername,
				"CLICKHOUSE_CANARY_PASSWORD": canaryPassword,
				"CLICKHOUSE_HOST":            clickhouseHost,
				"CLICKHOUSE_PORT":            clickhousePort,
				"CLICKHOUSE_HTTP_PORT":       clickhouseHTTPPort,
			},
		}

		result.secret = connectionSecret
		result.secretInstalled = true
	}

	return result, nil
}
