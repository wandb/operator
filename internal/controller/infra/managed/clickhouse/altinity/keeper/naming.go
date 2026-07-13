package keeper

import (
	"fmt"
)

// chkNameSuffix is terse on purpose: it comes out of the DNS-1123 budget of
// every derived per-host name (see PerHostConfigVolumeName).
const chkNameSuffix = "-chk"

// InstallationName derives the Keeper CR name from the base shared with the
// installation ("<base>-chi" / "<base>-chk"). Re-derived every reconcile:
// changing the scheme after managed ClickHouse ships needs a migration path.
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

// keeperShardOrdinal is always 0: the CHK layout sets only ReplicasCount.
const keeperShardOrdinal = 0

// PerHostConfigVolumeName mirrors the Altinity operator's per-host
// ConfigMap/StatefulSet-volume name (pkg/model/chk/namer/patterns.go). The
// longest derived name, and a DNS-1123 label: past 63 chars the apiserver
// rejects the StatefulSet and Altinity retries without surfacing the failure.
func PerHostConfigVolumeName(baseName string, replicaOrdinal int) string {
	return fmt.Sprintf("chk-%s-deploy-confd-%s-%d-%d",
		InstallationName(baseName), ClusterName, keeperShardOrdinal, replicaOrdinal)
}
