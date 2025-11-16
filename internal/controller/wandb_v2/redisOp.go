package wandb_v2

import (
	"context"
	"errors"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/ctrlqueue"
	common "github.com/wandb/operator/internal/vendored/redis-operator/common/v1beta2"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	corev1 "k8s.io/api/core/v1"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type wandbRedisWrapper struct {
	installed       bool
	obj             *redisv1beta2.Redis
	secretInstalled bool
	secret          *corev1.Secret
}

func (w *wandbRedisWrapper) IsReady() bool {
	if !w.installed || w.obj == nil {
		return false
	}
	return true
}

func (w *wandbRedisWrapper) GetStatus() string {
	if !w.installed || w.obj == nil {
		return "NotInstalled"
	}
	return "ready"
}

type wandbRedisDoReconcile interface {
	Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) ctrlqueue.CtrlState
}

func redisNamespacedName(wandb *apiv2.WeightsAndBiases) types.NamespacedName {
	namespace := wandb.Spec.Redis.Namespace
	if namespace == "" {
		namespace = wandb.Namespace
	}
	return types.NamespacedName{
		Name:      "wandb-redis",
		Namespace: namespace,
	}
}

func (r *WeightsAndBiasesV2Reconciler) handleRedis(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) ctrlqueue.CtrlState {
	var err error
	var desiredRedis wandbRedisWrapper
	var actualRedis wandbRedisWrapper
	var reconciliation wandbRedisDoReconcile
	log := ctrl.LoggerFrom(ctx)
	namespacedName := redisNamespacedName(wandb)

	if !wandb.Spec.Redis.Enabled {
		log.Info("Redis not enabled, skipping")
		return ctrlqueue.CtrlContinue()
	}

	log.Info("Handling Redis")

	if actualRedis, err = getActualRedis(ctx, r, namespacedName); err != nil {
		log.Error(err, "Failed to get actual Redis")
		return ctrlqueue.CtrlError(err)
	}

	if desiredRedis, err = getDesiredRedis(ctx, wandb, namespacedName, actualRedis); err != nil {
		log.Error(err, "Failed to compute desired Redis state")
		return ctrlqueue.CtrlError(err)
	}

	if reconciliation, err = computeRedisReconcileDrift(ctx, wandb, desiredRedis, actualRedis); err != nil {
		log.Error(err, "Failed to compute Redis reconciliation drift")
		return ctrlqueue.CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return ctrlqueue.CtrlContinue()
}

func getActualRedis(
	ctx context.Context, reconciler *WeightsAndBiasesV2Reconciler, namespacedName types.NamespacedName,
) (
	wandbRedisWrapper, error,
) {
	result := wandbRedisWrapper{
		installed:       false,
		obj:             nil,
		secretInstalled: false,
		secret:          nil,
	}
	obj := &redisv1beta2.Redis{}
	err := reconciler.Get(ctx, namespacedName, obj)
	if err != nil {
		if machErrors.IsNotFound(err) {
			return result, nil
		}
		return result, err
	}
	result.obj = obj
	result.installed = true

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

func getDesiredRedis(
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

func computeRedisReconcileDrift(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, desiredRedis, actualRedis wandbRedisWrapper,
) (
	wandbRedisDoReconcile, error,
) {
	if !desiredRedis.installed && actualRedis.installed {
		if actualRedis.secretInstalled {
			return &wandbRedisConnInfoDelete{
				wandb: wandb,
			}, nil
		}
		return &wandbRedisDelete{
			actual: actualRedis,
			wandb:  wandb,
		}, nil
	}
	if desiredRedis.installed && !actualRedis.installed {
		return &wandbRedisCreate{
			desired: desiredRedis,
			wandb:   wandb,
		}, nil
	}

	if desiredRedis.secretInstalled && !actualRedis.secretInstalled {
		return &wandbRedisConnInfoCreate{
			desired: desiredRedis,
			wandb:   wandb,
		}, nil
	}

	if actualRedis.GetStatus() != string(wandb.Status.RedisStatus.State) ||
		actualRedis.IsReady() != wandb.Status.RedisStatus.Ready {
		return &wandbRedisStatusUpdate{
			wandb:  wandb,
			status: actualRedis.GetStatus(),
			ready:  actualRedis.IsReady(),
		}, nil
	}
	return nil, nil
}

type wandbRedisCreate struct {
	desired wandbRedisWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbRedisCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) ctrlqueue.CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing Redis")
	wandb := c.wandb
	if err = controllerutil.SetOwnerReference(wandb, c.desired.obj, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for Redis")
		return ctrlqueue.CtrlError(err)
	}
	if err = r.Create(ctx, c.desired.obj); err != nil {
		log.Error(err, "Failed to create Redis")
		return ctrlqueue.CtrlError(err)
	}
	wandb.Status.State = apiv2.WBStateUpdating
	wandb.Status.RedisStatus.State = "pending"
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after creating Redis")
		return ctrlqueue.CtrlError(err)
	}
	return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
}

type wandbRedisDelete struct {
	actual wandbRedisWrapper
	wandb  *apiv2.WeightsAndBiases
}

func (d *wandbRedisDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) ctrlqueue.CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling Redis")
	if err = r.Delete(ctx, d.actual.obj); err != nil {
		log.Error(err, "Failed to delete Redis")
		return ctrlqueue.CtrlError(err)
	}
	wandb := d.wandb
	wandb.Status.State = apiv2.WBStateUpdating
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after deleting Redis")
		return ctrlqueue.CtrlError(err)
	}
	return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
}

type wandbRedisStatusUpdate struct {
	wandb  *apiv2.WeightsAndBiases
	status string
	ready  bool
}

func (s *wandbRedisStatusUpdate) Execute(
	ctx context.Context, r *WeightsAndBiasesV2Reconciler,
) ctrlqueue.CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Updating Redis status", "status", s.status, "ready", s.ready)
	s.wandb.Status.RedisStatus.State = apiv2.WBStateUpdating
	s.wandb.Status.RedisStatus.Ready = s.ready
	if err := r.Status().Update(ctx, s.wandb); err != nil {
		log.Error(err, "Failed to update Redis status")
		return ctrlqueue.CtrlError(err)
	}
	return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
}

type wandbRedisConnInfoCreate struct {
	desired wandbRedisWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbRedisConnInfoCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) ctrlqueue.CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Creating Redis connection secret")

	if c.desired.secret == nil {
		err := errors.New("desired secret is nil")
		log.Error(err, "Desired Redis connection secret is nil")
		return ctrlqueue.CtrlError(err)
	}

	if err := controllerutil.SetOwnerReference(c.wandb, c.desired.secret, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for Redis connection secret")
		return ctrlqueue.CtrlError(err)
	}

	if err := r.Create(ctx, c.desired.secret); err != nil {
		log.Error(err, "Failed to create Redis connection secret")
		return ctrlqueue.CtrlError(err)
	}

	log.Info("Redis connection secret created successfully")
	return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
}

type wandbRedisConnInfoDelete struct {
	wandb *apiv2.WeightsAndBiases
}

func (d *wandbRedisConnInfoDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) ctrlqueue.CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Deleting Redis connection secret")

	namespace := d.wandb.Spec.Redis.Namespace
	if namespace == "" {
		namespace = d.wandb.Namespace
	}
	namespacedName := types.NamespacedName{
		Name:      "wandb-redis-connection",
		Namespace: namespace,
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, namespacedName, secret)
	if err != nil {
		if machErrors.IsNotFound(err) {
			log.Info("Redis connection secret already deleted")
			return ctrlqueue.CtrlContinue()
		}
		log.Error(err, "Failed to get Redis connection secret for deletion")
		return ctrlqueue.CtrlError(err)
	}

	if err := r.Delete(ctx, secret); err != nil {
		log.Error(err, "Failed to delete Redis connection secret")
		return ctrlqueue.CtrlError(err)
	}

	log.Info("Redis connection secret deleted successfully")
	return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
}
