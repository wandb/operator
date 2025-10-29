package wandb_v2

import (
	"context"
	"errors"

	common "github.com/wandb/operator/api/redis-operator-vendored/common/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redissentinel/v1beta2"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type wandbRedisHAWrapper struct {
	replicationInstalled bool
	replicationObj       *redisreplicationv1beta2.RedisReplication
	sentinelInstalled    bool
	sentinelObj          *redissentinelv1beta2.RedisSentinel
	secretInstalled      bool
	secret               *corev1.Secret
}

func (w *wandbRedisHAWrapper) IsReady() bool {
	if !w.replicationInstalled || w.replicationObj == nil {
		return false
	}
	if !w.sentinelInstalled || w.sentinelObj == nil {
		return false
	}
	return true
}

func (w *wandbRedisHAWrapper) GetStatus() string {
	if !w.replicationInstalled || w.replicationObj == nil {
		return "NotInstalled"
	}
	if !w.sentinelInstalled || w.sentinelObj == nil {
		return "NotInstalled"
	}
	return "ready"
}

func (w *wandbRedisHAWrapper) maybeHandleDeletion(
	_ context.Context, wandb *apiv2.WeightsAndBiases, _ wandbRedisHAWrapper, _ *WeightsAndBiasesV2Reconciler,
) CtrlState {
	if wandb.Spec.Redis.Enabled {
		return CtrlContinue()
	}
	return CtrlContinue()
}

type wandbRedisHADoReconcile interface {
	Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState
}

func (r *WeightsAndBiasesV2Reconciler) handleRedisHA(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) CtrlState {
	var err error
	var desiredRedis wandbRedisHAWrapper
	var actualRedis wandbRedisHAWrapper
	var reconciliation wandbRedisHADoReconcile
	log := ctrl.LoggerFrom(ctx)
	namespacedName := redisNamespacedName(req)

	if !wandb.Spec.Redis.Enabled {
		log.Info("Redis not enabled, skipping")
		return CtrlContinue()
	}

	log.Info("Handling Redis HA")

	if actualRedis, err = getActualRedisHA(ctx, r, namespacedName); err != nil {
		log.Error(err, "Failed to get actual Redis HA")
		return CtrlError(err)
	}

	if ctrlState := actualRedis.maybeHandleDeletion(ctx, wandb, actualRedis, r); ctrlState.shouldExit(HandlerScope) {
		return ctrlState
	}

	if desiredRedis, err = getDesiredRedisHA(ctx, wandb, namespacedName, actualRedis); err != nil {
		log.Error(err, "Failed to compute desired Redis HA state")
		return CtrlError(err)
	}

	if reconciliation, err = computeRedisHAReconcileDrift(ctx, wandb, desiredRedis, actualRedis); err != nil {
		log.Error(err, "Failed to compute Redis HA reconciliation drift")
		return CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return CtrlContinue()
}

func getActualRedisHA(
	ctx context.Context, reconciler *WeightsAndBiasesV2Reconciler, namespacedName types.NamespacedName,
) (
	wandbRedisHAWrapper, error,
) {
	result := wandbRedisHAWrapper{
		replicationInstalled: false,
		replicationObj:       nil,
		sentinelInstalled:    false,
		sentinelObj:          nil,
		secretInstalled:      false,
		secret:               nil,
	}

	replicationObj := &redisreplicationv1beta2.RedisReplication{}
	err := reconciler.Get(ctx, namespacedName, replicationObj)
	if err != nil {
		if !machErrors.IsNotFound(err) {
			return result, err
		}
	} else {
		result.replicationObj = replicationObj
		result.replicationInstalled = true
	}

	sentinelNamespacedName := types.NamespacedName{
		Name:      "wandb-redis",
		Namespace: namespacedName.Namespace,
	}
	sentinelObj := &redissentinelv1beta2.RedisSentinel{}
	err = reconciler.Get(ctx, sentinelNamespacedName, sentinelObj)
	if err != nil {
		if !machErrors.IsNotFound(err) {
			return result, err
		}
	} else {
		result.sentinelObj = sentinelObj
		result.sentinelInstalled = true
	}

	secretNamespacedName := types.NamespacedName{
		Name:      "wandb-redis-connection",
		Namespace: namespacedName.Namespace,
	}
	secret := &corev1.Secret{}
	err = reconciler.Get(ctx, secretNamespacedName, secret)
	if err == nil {
		result.secret = secret
		result.secretInstalled = true
	} else if !machErrors.IsNotFound(err) {
		return result, err
	}

	return result, nil
}

func getDesiredRedisHA(
	_ context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName, actual wandbRedisHAWrapper,
) (
	wandbRedisHAWrapper, error,
) {
	result := wandbRedisHAWrapper{
		replicationInstalled: false,
		replicationObj:       nil,
		sentinelInstalled:    false,
		sentinelObj:          nil,
		secretInstalled:      false,
		secret:               nil,
	}

	if !wandb.Spec.Redis.Enabled {
		return result, nil
	}

	result.replicationInstalled = true
	result.sentinelInstalled = true

	storageSize := wandb.Spec.Redis.StorageSize
	if storageSize == "" {
		storageSize = "1Gi"
	}

	storageQuantity, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return result, errors.New("invalid storage size: " + storageSize)
	}

	clusterSize := int32(3)
	masterName := namespacedName.Name

	redisReplication := &redisreplicationv1beta2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
		},
		Spec: redisreplicationv1beta2.RedisReplicationSpec{
			Size: &clusterSize,
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

	redisSentinel := &redissentinelv1beta2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-redis",
			Namespace: namespacedName.Namespace,
		},
		Spec: redissentinelv1beta2.RedisSentinelSpec{
			Size: &clusterSize,
			KubernetesConfig: common.KubernetesConfig{
				Image:           "quay.io/opstree/redis-sentinel:v7.0.12",
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
			RedisSentinelConfig: &redissentinelv1beta2.RedisSentinelConfig{
				RedisSentinelConfig: common.RedisSentinelConfig{
					RedisReplicationName: namespacedName.Name,
					MasterGroupName:      namespacedName.Name,
				},
			},
		},
	}

	result.replicationObj = redisReplication
	result.sentinelObj = redisSentinel

	if actual.IsReady() {
		namespace := namespacedName.Namespace
		sentinelHost := "wandb-redis-sentinel." + namespace + ".svc.cluster.local"
		sentinelPort := "26379"

		connectionSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wandb-redis-connection",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"REDIS_SENTINEL_HOST": sentinelHost,
				"REDIS_SENTINEL_PORT": sentinelPort,
				"REDIS_MASTER_NAME":   masterName,
			},
		}

		result.secret = connectionSecret
		result.secretInstalled = true
	}

	return result, nil
}

func computeRedisHAReconcileDrift(
	_ context.Context, wandb *apiv2.WeightsAndBiases, desired, actual wandbRedisHAWrapper,
) (
	wandbRedisHADoReconcile, error,
) {
	if !desired.replicationInstalled && actual.replicationInstalled {
		if actual.secretInstalled {
			return &wandbRedisHAConnInfoDelete{
				wandb: wandb,
			}, nil
		}
		if actual.sentinelInstalled {
			return &wandbRedisHASentinelDelete{
				actual: actual,
				wandb:  wandb,
			}, nil
		}
		return &wandbRedisHAReplicationDelete{
			actual: actual,
			wandb:  wandb,
		}, nil
	}

	if desired.replicationInstalled && !actual.replicationInstalled {
		return &wandbRedisHAReplicationCreate{
			desired: desired,
			wandb:   wandb,
		}, nil
	}

	if desired.sentinelInstalled && !actual.sentinelInstalled {
		return &wandbRedisHASentinelCreate{
			desired: desired,
			wandb:   wandb,
		}, nil
	}

	if desired.secretInstalled && !actual.secretInstalled {
		return &wandbRedisHAConnInfoCreate{
			desired: desired,
			wandb:   wandb,
		}, nil
	}

	if actual.GetStatus() != wandb.Status.RedisStatus.State ||
		actual.IsReady() != wandb.Status.RedisStatus.Ready {
		return &wandbRedisHAStatusUpdate{
			wandb:  wandb,
			status: actual.GetStatus(),
			ready:  actual.IsReady(),
		}, nil
	}

	return nil, nil
}

type wandbRedisHAReplicationCreate struct {
	desired wandbRedisHAWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbRedisHAReplicationCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing Redis Replication")
	wandb := c.wandb
	if err := controllerutil.SetOwnerReference(wandb, c.desired.replicationObj, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for Redis Replication")
		return CtrlError(err)
	}
	if err := r.Create(ctx, c.desired.replicationObj); err != nil {
		log.Error(err, "Failed to create Redis Replication")
		return CtrlError(err)
	}
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Creating Redis Replication"
	wandb.Status.RedisStatus.State = "pending"
	if err := r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after creating Redis Replication")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbRedisHASentinelCreate struct {
	desired wandbRedisHAWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbRedisHASentinelCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing Redis Sentinel")
	wandb := c.wandb
	if err := controllerutil.SetOwnerReference(wandb, c.desired.sentinelObj, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for Redis Sentinel")
		return CtrlError(err)
	}
	if err := r.Create(ctx, c.desired.sentinelObj); err != nil {
		log.Error(err, "Failed to create Redis Sentinel")
		return CtrlError(err)
	}
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Creating Redis Sentinel"
	if err := r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after creating Redis Sentinel")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbRedisHAReplicationDelete struct {
	actual wandbRedisHAWrapper
	wandb  *apiv2.WeightsAndBiases
}

func (d *wandbRedisHAReplicationDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling Redis Replication")
	if err := r.Delete(ctx, d.actual.replicationObj); err != nil {
		log.Error(err, "Failed to delete Redis Replication")
		return CtrlError(err)
	}
	wandb := d.wandb
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Deleting Redis Replication"
	if err := r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after deleting Redis Replication")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbRedisHASentinelDelete struct {
	actual wandbRedisHAWrapper
	wandb  *apiv2.WeightsAndBiases
}

func (d *wandbRedisHASentinelDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling Redis Sentinel")
	if err := r.Delete(ctx, d.actual.sentinelObj); err != nil {
		log.Error(err, "Failed to delete Redis Sentinel")
		return CtrlError(err)
	}
	wandb := d.wandb
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Deleting Redis Sentinel"
	if err := r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after deleting Redis Sentinel")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbRedisHAStatusUpdate struct {
	wandb  *apiv2.WeightsAndBiases
	status string
	ready  bool
}

func (s *wandbRedisHAStatusUpdate) Execute(
	ctx context.Context, r *WeightsAndBiasesV2Reconciler,
) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Updating Redis HA status", "status", s.status, "ready", s.ready)
	s.wandb.Status.RedisStatus.State = s.status
	s.wandb.Status.RedisStatus.Ready = s.ready
	if err := r.Status().Update(ctx, s.wandb); err != nil {
		log.Error(err, "Failed to update Redis HA status")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbRedisHAConnInfoCreate struct {
	desired wandbRedisHAWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbRedisHAConnInfoCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Creating Redis HA connection secret")

	if c.desired.secret == nil {
		err := errors.New("desired secret is nil")
		log.Error(err, "Desired Redis HA connection secret is nil")
		return CtrlError(err)
	}

	if err := controllerutil.SetOwnerReference(c.wandb, c.desired.secret, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for Redis HA connection secret")
		return CtrlError(err)
	}

	if err := r.Create(ctx, c.desired.secret); err != nil {
		log.Error(err, "Failed to create Redis HA connection secret")
		return CtrlError(err)
	}

	log.Info("Redis HA connection secret created successfully")
	return CtrlDone(HandlerScope)
}

type wandbRedisHAConnInfoDelete struct {
	wandb *apiv2.WeightsAndBiases
}

func (d *wandbRedisHAConnInfoDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Deleting Redis HA connection secret")

	namespacedName := types.NamespacedName{
		Name:      "wandb-redis-connection",
		Namespace: d.wandb.Namespace,
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, namespacedName, secret)
	if err != nil {
		if machErrors.IsNotFound(err) {
			log.Info("Redis HA connection secret already deleted")
			return CtrlContinue()
		}
		log.Error(err, "Failed to get Redis HA connection secret for deletion")
		return CtrlError(err)
	}

	if err := r.Delete(ctx, secret); err != nil {
		log.Error(err, "Failed to delete Redis HA connection secret")
		return CtrlError(err)
	}

	log.Info("Redis HA connection secret deleted successfully")
	return CtrlDone(HandlerScope)
}
