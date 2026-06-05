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

// ClientServiceName returns the name of the Service the Altinity keeper operator
// creates for client (ZooKeeper-protocol) connections to a CHK. The operator's
// namer uses the pattern "keeper-<cr-name>".
func ClientServiceName(chSpecName string) string {
	return "keeper-" + InstallationName(chSpecName)
}

// ClientServiceFQDN returns the in-cluster DNS name of the Keeper client service
// that ClickHouse points its <zookeeper> config at.
func ClientServiceFQDN(namespace, chSpecName string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", ClientServiceName(chSpecName), namespace)
}

// SpecNamespacedName returns the namespaced name of the Keeper CR for a managed
// ClickHouse spec.
func SpecNamespacedName(spec *apiv2.ManagedClickHouseSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      InstallationName(spec.Name),
	}
}
