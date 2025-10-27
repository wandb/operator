package wandb_v2

import (
	"context"
	"errors"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *WeightsAndBiasesV2Reconciler) handleRedisHA(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) CtrlState {
	var err error
	var desiredRedis wandbRedisWrapper
	var actualRedis wandbRedisWrapper
	var reconciliation wandbRedisDoReconcile
	log := ctrl.LoggerFrom(ctx)
	namespacedName := redisNamespacedName(req)

	if !wandb.Spec.Redis.Enabled {
		log.Info("Redis not enabled, skipping")
		return CtrlContinue()
	}

	log.Info("Handling Redis HA")

	if actualRedis, err = getActualRedis(ctx, r, namespacedName); err != nil {
		log.Error(err, "Failed to get actual Redis HA")
		return CtrlError(err)
	}

	if desiredRedis, err = getDesiredRedisHA(ctx, wandb, namespacedName, actualRedis); err != nil {
		log.Error(err, "Failed to compute desired Redis HA state")
		return CtrlError(err)
	}

	if reconciliation, err = computeRedisReconcileDrift(ctx, wandb, desiredRedis, actualRedis); err != nil {
		log.Error(err, "Failed to compute Redis HA reconciliation drift")
		return CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return CtrlContinue()
}

func getDesiredRedisHA(
	_ context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName, actual wandbRedisWrapper,
) (
	wandbRedisWrapper, error,
) {
	result := wandbRedisWrapper{
		installed:       false,
		obj:             nil,
		secretInstalled: false,
		secret:          nil,
	}

	if !wandb.Spec.Redis.Enabled {
		return result, nil
	}

	result.installed = true

	storageSize := wandb.Spec.Redis.StorageSize
	if storageSize == "" {
		storageSize = "1Gi"
	}

	storageQuantity, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return result, errors.New("invalid storage size: " + storageSize)
	}

	redis := &redisv1beta2.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
		},
		Spec: redisv1beta2.RedisSpec{
			KubernetesConfig: common.KubernetesConfig{
				Image:           "quay.io/opstree/redis:v7.0.15",
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
			Storage: &common.Storage{
				VolumeClaimTemplate: corev1.PersistentVolumeClaim{
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
	}

	result.obj = redis

	if actual.IsReady() {
		namespace := namespacedName.Namespace
		redisHost := "wandb-redis." + namespace + ".svc.cluster.local"
		redisPort := "6379"

		connectionSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wandb-redis-connection",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"REDIS_HOST": redisHost,
				"REDIS_PORT": redisPort,
			},
		}

		result.secret = connectionSecret
		result.secretInstalled = true
	}

	return result, nil
}
