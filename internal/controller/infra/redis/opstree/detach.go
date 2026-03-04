package opstree

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	redisv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redissentinel/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DetachFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, _ = logx.WithSlog(ctx, logx.Redis)
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	ownerUID := wandbOwner.GetUID()

	if err := common.DetachOwnerReference(ctx, cl, nsnBuilder.StandaloneNsName(), StandaloneType, &redisv1beta2.Redis{}, ownerUID); err != nil {
		return err
	}
	if err := common.DetachOwnerReference(ctx, cl, nsnBuilder.SentinelNsName(), SentinelType, &redissentinelv1beta2.RedisSentinel{}, ownerUID); err != nil {
		return err
	}
	return common.DetachOwnerReference(ctx, cl, nsnBuilder.ReplicationNsName(), ReplicationType, &redisreplicationv1beta2.RedisReplication{}, ownerUID)
}
