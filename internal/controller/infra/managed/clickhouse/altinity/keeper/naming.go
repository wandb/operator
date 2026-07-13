package keeper

import (
	"fmt"
)

// chkNameSuffix is deliberately terse ("chk" = ClickHouseKeeperInstallation):
// every character comes out of the DNS-1123 label budget of the per-host
// names the Altinity operator derives (see PerHostConfigVolumeName).
const chkNameSuffix = "-chk"

// InstallationName derives the Keeper CR name from the base name the managed
// ClickHouse resources share (the spec name minus its "-chi" suffix, which
// the altinity package strips), so the pair reads "<base>-chi" / "<base>-chk"
// and the Keeper consumes no more of the derived-name budget than the
// ClickHouse installation does.
//
// The result is re-derived every reconcile, never persisted: once a release
// ships managed ClickHouse, changing this scheme orphans running Keepers and
// repoints the CHI at an empty ensemble, so it will need a migration path.
func InstallationName(baseName string) string {
	return baseName + chkNameSuffix
}

// ClientServiceName is the Altinity-created client Service name ("keeper-<chk-name>").
func ClientServiceName(baseName string) string {
	return "keeper-" + InstallationName(baseName)
}

// ClientServiceFQDN is the in-cluster DNS the CHI's <zookeeper> config points at.
func ClientServiceFQDN(namespace, baseName string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", ClientServiceName(baseName), namespace)
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
// surfacing the failure. It is the longest name derived from the base name,
// so it defines the name budget.
func PerHostConfigVolumeName(baseName string, replicaOrdinal int) string {
	return fmt.Sprintf("chk-%s-deploy-confd-%s-%d-%d",
		InstallationName(baseName), ClusterName, keeperShardOrdinal, replicaOrdinal)
}
