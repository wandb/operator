package altinity

import (
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
	return createNsNameBuilder(types.NamespacedName{Name: specName}).InstallationName()
}

func ClusterName(specName string) string {
	return createNsNameBuilder(types.NamespacedName{Name: specName}).ClusterName()
}

func VolumeTemplateName(specName string) string {
	return createNsNameBuilder(types.NamespacedName{Name: specName}).VolumeTemplateName()
}
