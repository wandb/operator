package opstree

import (
	"context"
	"log/slog"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	redisv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redissentinel/v1beta2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CheckDetached(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbUID types.UID,
) []metav1.Condition {
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	actual := &redisv1beta2.Redis{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.StandaloneNsName(), StandaloneType, actual)
	if err != nil || !found {
		return nil
	}
	if !common.IsDetached(actual, wandbUID) {
		return nil
	}
	return nil
}

func DetachFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, log := logx.WithSlog(ctx, logx.Redis)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	if err := detachResource(ctx, cl, log, nsnBuilder.StandaloneNsName(), StandaloneType, &redisv1beta2.Redis{}, wandbOwner); err != nil {
		return err
	}
	if err := detachResource(ctx, cl, log, nsnBuilder.SentinelNsName(), SentinelType, &redissentinelv1beta2.RedisSentinel{}, wandbOwner); err != nil {
		return err
	}
	return detachResource(ctx, cl, log, nsnBuilder.ReplicationNsName(), ReplicationType, &redisreplicationv1beta2.RedisReplication{}, wandbOwner)
}

func detachResource[T client.Object](
	ctx context.Context,
	cl client.Client,
	log *slog.Logger,
	nsName types.NamespacedName,
	typeName string,
	actual T,
	wandbOwner client.Object,
) error {
	found, err := common.GetResource(ctx, cl, nsName, typeName, actual)
	if err != nil {
		return err
	}
	if !found {
		log.Info("abort detach: CR not found", "type", typeName)
		return nil
	}
	if common.IsDetached(actual, wandbOwner.GetUID()) {
		log.Debug("CR already detached", "type", typeName)
		return nil
	}

	common.RemoveOwnerReference(actual, wandbOwner.GetUID())
	if err = cl.Update(ctx, actual); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error("error detaching CR", logx.ErrAttr(err), "type", typeName)
		return err
	}
	log.Info("detached CR", "type", typeName, "name", nsName.Name)
	return nil
}
