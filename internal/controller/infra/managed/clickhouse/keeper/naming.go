package keeper

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/types"
)

// InstallationName derives the Keeper CR name from the ClickHouse spec name.
func InstallationName(chSpecName string) string {
	return chSpecName + "-keeper"
}

// SpecNamespacedName returns the namespaced name of the Keeper CR for a managed
// ClickHouse spec.
func SpecNamespacedName(spec *apiv2.ManagedClickHouseSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      InstallationName(spec.Name),
	}
}
