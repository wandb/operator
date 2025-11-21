package tenant

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

const (
	// Resource names
	ConnectionName = "wandb-minio-connection"
	ConfigSecret   = "wandb-minio-config"

	// Minio configuration
	MinioPort      = 443
	MinioAccessKey = "minio"
	MinioBucket    = "wandb"
)

func TenantName(specName string) string {
	return specName
}

func ConfigName(specName string) string {
	return fmt.Sprintf("%s-config", specName)
}

func ServiceName(specName string) string {
	return fmt.Sprintf("%s-hl", specName)
}

func PoolName(specName string) string {
	return fmt.Sprintf("%s-pool", specName)
}

func TenantNamespacedName(specNamespacedName types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: specNamespacedName.Namespace,
		Name:      TenantName(specNamespacedName.Name),
	}
}
