package altinity

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

const (
	// Resource names
	CHIName        = "wandb-clickhouse"
	ServiceName    = "clickhouse-wandb-clickhouse"
	ConnectionName = "wandb-clickhouse-connection"

	// ClickHouse configuration
	ClickHouseNativePort = 9000
	ClickHouseHTTPPort   = 8123
	ClickHouseUser       = "test_user"
	ClickHousePassword   = "test_password"

	// Cluster configuration
	ShardsCount = 1
)

func InstallationName(specName string) string {
	return fmt.Sprintf("%s-altinity-install", specName)
}

func ClusterName(specName string) string {
	return specName
}

func VolumeTemplateName(specName string) string {
	return fmt.Sprintf("%s-volume-template", specName)
}

func InstallationNamespacedName(specNamespacedName types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: specNamespacedName.Namespace,
		Name:      InstallationName(specNamespacedName.Name),
	}
}
