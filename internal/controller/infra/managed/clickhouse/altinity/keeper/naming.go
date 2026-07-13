package keeper

import (
	"fmt"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/types"
)

// ChiNameSuffix is the suffix the defaulting webhook appends to the CR name
// to derive the managed ClickHouse name ("<base>-chi"). It is defined here
// rather than in the altinity package (which imports this one) so
// InstallationName can swap it for the Keeper suffix without an import cycle.
const ChiNameSuffix = "-chi"

// chkNameSuffix is deliberately terse ("chk" = ClickHouseKeeperInstallation):
// every character comes out of the DNS-1123 label budget of the per-host
// names the Altinity operator derives (see PerHostConfigVolumeName).
const chkNameSuffix = "-chk"

// InstallationName derives the Keeper CR name from the ClickHouse spec name,
// swapping a trailing "-chi" for "-chk" so the pair reads "<base>-chi" /
// "<base>-chk" and the Keeper consumes no more of the derived-name budget
// than the ClickHouse installation does.
//
// The result is re-derived every reconcile, never persisted: once a release
// ships managed ClickHouse, changing this scheme orphans running Keepers and
// repoints the CHI at an empty ensemble, so it will need a migration path.
func InstallationName(chSpecName string) string {
	return strings.TrimSuffix(chSpecName, ChiNameSuffix) + chkNameSuffix
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

// keeperShardOrdinal is always 0: the CHK spec sets only ReplicasCount, so the
// Altinity operator lays all hosts out in a single shard.
const keeperShardOrdinal = 0

// PerHostConfigVolumeName mirrors the Altinity operator's per-host ConfigMap
// name for a CHK — "chk-{cr}-deploy-confd-{cluster}-{shard}-{replica}"
// (upstream pkg/model/chk/namer/patterns.go, patternConfigMapHostName). The
// operator also uses it as a volume name in the host StatefulSet, and volume
// names must be DNS-1123 labels (63 chars): when this is longer the apiserver
// rejects the StatefulSet and the Altinity operator retries forever without
// surfacing the failure. It is the longest name derived from the spec name,
// so it defines the name budget.
func PerHostConfigVolumeName(chSpecName string, replicaOrdinal int) string {
	return fmt.Sprintf("chk-%s-deploy-confd-%s-%d-%d",
		InstallationName(chSpecName), ClusterName, keeperShardOrdinal, replicaOrdinal)
}
