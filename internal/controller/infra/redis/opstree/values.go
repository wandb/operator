package opstree

import "k8s.io/apimachinery/pkg/types"

const (
	namePrefix       = "wandb-redis"
	standaloneName   = "wandb-redis"
	replicationName  = "wandb-redis"
	sentinelName     = "wandb-redis-sentinel"
	standaloneImage  = "quay.io/opstree/redis:v7.0.15"
	replicationImage = "quay.io/opstree/redis:v7.0.15"
	sentinelImage    = "quay.io/opstree/redis-sentinel:v7.0.12"

	vendorName = "OpstreeRedis"
	vendorKey  = "vendor"
)

func standaloneNamespacedName(namespace string) types.NamespacedName {
	return types.NamespacedName{
		Name:      standaloneName,
		Namespace: namespace,
	}
}

func replicationNamespacedName(namespace string) types.NamespacedName {
	return types.NamespacedName{
		Name:      replicationName,
		Namespace: namespace,
	}
}

func sentinelNamespacedName(namespace string) types.NamespacedName {
	return types.NamespacedName{
		Name:      sentinelName,
		Namespace: namespace,
	}
}
