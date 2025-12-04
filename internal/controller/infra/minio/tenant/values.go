package tenant

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

const (
	MinioPort                  = 443
	MinioAccessKey             = "minio"
	MinioBucket                = "wandb"
	TenantMinioRootUserKey     = "rootUser"
	TenantMinioRootPasswordKey = "rootPassword"
	TenantMinioBrowserKey      = "browser"
)

type MinioEnvConfig struct {
	RootUser            string
	MinioBrowserSetting string
}

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

func ConfigNamespacedName(specNamespacedName types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: specNamespacedName.Namespace,
		Name:      ConfigName(specNamespacedName.Name),
	}
}
