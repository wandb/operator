package wandb_v2

import (
	"context"
	"errors"
	"time"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var defaultRedisRequeueSeconds = 30
var defaultRedisRequeueDuration = time.Duration(defaultRedisRequeueSeconds) * time.Second

type wandbRedisWrapper struct {
	installed bool
	obj       *redisv1beta2.Redis
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
	Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState
}

func redisNamespacedName(req ctrl.Request) types.NamespacedName {
	return types.NamespacedName{
		Name:      "wandb-redis",
		Namespace: req.Namespace,
	}
}

func (r *WeightsAndBiasesV2Reconciler) handleRedis(
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

	log.Info("Handling Redis")

	if actualRedis, err = getActualRedis(ctx, r, namespacedName); err != nil {
		return CtrlError(err)
	}

	if desiredRedis, err = getDesiredRedis(ctx, wandb, namespacedName); err != nil {
		return CtrlError(err)
	}

	if reconciliation, err = computeRedisReconcileDrift(ctx, wandb, desiredRedis, actualRedis); err != nil {
		return CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return CtrlContinue()
}

func getActualRedis(
	ctx context.Context, reconciler *WeightsAndBiasesV2Reconciler, namespacedName types.NamespacedName,
) (
	wandbRedisWrapper, error,
) {
	result := wandbRedisWrapper{
		installed: false,
		obj:       nil,
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
	return result, nil
}

func getDesiredRedis(
	_ context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName,
) (
	wandbRedisWrapper, error,
) {
	result := wandbRedisWrapper{
		installed: false,
		obj:       nil,
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
	return result, nil
}

func computeRedisReconcileDrift(
	_ context.Context, wandb *apiv2.WeightsAndBiases, desiredRedis, actualRedis wandbRedisWrapper,
) (
	wandbRedisDoReconcile, error,
) {
	if !desiredRedis.installed && actualRedis.installed {
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
	if actualRedis.GetStatus() != wandb.Status.RedisStatus.State ||
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

func (c *wandbRedisCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing Redis")
	wandb := c.wandb
	if err = controllerutil.SetOwnerReference(wandb, c.desired.obj, r.Scheme); err != nil {
		return CtrlError(err)
	}
	if err = r.Create(ctx, c.desired.obj); err != nil {
		return CtrlError(err)
	}
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Creating Redis"
	wandb.Status.RedisStatus.State = "pending"
	if err = r.Status().Update(ctx, wandb); err != nil {
		return CtrlError(err)
	}
	return CtrlDone(ctrl.Result{RequeueAfter: defaultRedisRequeueDuration})
}

type wandbRedisDelete struct {
	actual wandbRedisWrapper
	wandb  *apiv2.WeightsAndBiases
}

func (d *wandbRedisDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling Redis")
	if err = r.Delete(ctx, d.actual.obj); err != nil {
		return CtrlError(err)
	}
	wandb := d.wandb
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Deleting Redis"
	if err = r.Status().Update(ctx, wandb); err != nil {
		return CtrlError(err)
	}
	return CtrlDone(ctrl.Result{RequeueAfter: defaultRedisRequeueDuration})
}

type wandbRedisStatusUpdate struct {
	wandb  *apiv2.WeightsAndBiases
	status string
	ready  bool
}

func (s *wandbRedisStatusUpdate) Execute(
	ctx context.Context, r *WeightsAndBiasesV2Reconciler,
) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Updating Redis status", "status", s.status, "ready", s.ready)
	s.wandb.Status.RedisStatus.State = s.status
	s.wandb.Status.RedisStatus.Ready = s.ready
	if err := r.Status().Update(ctx, s.wandb); err != nil {
		return CtrlError(err)
	}
	return CtrlDone(ctrl.Result{RequeueAfter: defaultRedisRequeueDuration})
}
