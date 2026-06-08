package keeper

import (
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/types"
)

// InstallationName derives the Keeper CR name from the ClickHouse spec name.
func InstallationName(chSpecName string) string {
	return chSpecName + "-keeper"
}

// ClientServiceName is the Altinity-created client Service name ("keeper-<cr-name>").
func ClientServiceName(chSpecName string) string {
	return "keeper-" + InstallationName(chSpecName)
}

// ClientServiceFQDN is the in-cluster DNS the CHI's <zookeeper> config points at.
func ClientServiceFQDN(namespace, chSpecName string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", ClientServiceName(chSpecName), namespace)
}

// SpecNamespacedName is the Keeper CR's namespaced name for a managed CH spec.
func SpecNamespacedName(spec *apiv2.ManagedClickHouseSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      InstallationName(spec.Name),
	}
}
