package altinity

import (
	"fmt"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity/keeper"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
)

type NsNameBuilder struct {
	baseNsName types.NamespacedName
}

func CreateNsNameBuilder(baseNsName types.NamespacedName) *NsNameBuilder {
	return &NsNameBuilder{
		baseNsName: baseNsName,
	}
}

func (n *NsNameBuilder) Namespace() string {
	return n.baseNsName.Namespace
}

func (n *NsNameBuilder) SpecName() string {
	return n.baseNsName.Name
}

func (n *NsNameBuilder) InstallationName() string {
	return n.SpecName()
}

func (n *NsNameBuilder) InstallationNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.InstallationName(),
	}
}

func (n *NsNameBuilder) ClusterName() string {
	name := n.SpecName()
	if len(name) > 15 {
		name = name[:15]
	}
	// Trim trailing hyphens
	for len(name) > 0 && name[len(name)-1] == '-' {
		name = name[:len(name)-1]
	}
	return name
}

func (n *NsNameBuilder) VolumeTemplateName() string {
	return fmt.Sprintf("%s-voltempl", n.SpecName())
}

func (n *NsNameBuilder) PodTemplateName() string {
	return fmt.Sprintf("%s-podtempl", n.SpecName())
}

func (n *NsNameBuilder) ConnectionName() string {
	return fmt.Sprintf("%s-connection", n.SpecName())
}

func (n *NsNameBuilder) ConnectionNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.ConnectionName(),
	}
}

// Internal function for backward compatibility within the package
func createNsNameBuilder(baseNsName types.NamespacedName) *NsNameBuilder {
	return CreateNsNameBuilder(baseNsName)
}

const (
	// chiClusterName is the single cluster the CHI defines.
	chiClusterName = "default"

	// defaultNameSuffix is appended to the CR name by the defaulting webhook;
	// terse to leave the CR name as much of the derived-name budget as possible.
	defaultNameSuffix = "-chi"

	// maxExpectedHostOrdinal reserves two digits each for shard and replica
	// ordinals (largest expected cluster ~100 pods), so a persisted name can
	// never overflow when the cluster is scaled up later.
	maxExpectedHostOrdinal = 99
)

// baseName strips the "-chi" suffix; the Keeper builds its names on the base.
func baseName(specName string) string {
	return strings.TrimSuffix(specName, defaultNameSuffix)
}

// KeeperNsName is the Keeper CR's namespaced name for a managed CH spec.
func KeeperNsName(spec *apiv2.ManagedClickHouseSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      keeper.InstallationName(baseName(spec.Name)),
	}
}

// perHostConfigVolumeName mirrors the Altinity operator's per-host
// ConfigMap/StatefulSet-volume name for a CHI (pkg/model/chi/namer/patterns.go);
// the longest CHI-derived name and a DNS-1123 label.
func perHostConfigVolumeName(specName string, shardOrdinal, replicaOrdinal int) string {
	return fmt.Sprintf("chi-%s-deploy-confd-%s-%d-%d", specName, chiClusterName, shardOrdinal, replicaOrdinal)
}

// MaxSpecNameLength is the longest defaulted ("<base>-chi") spec name whose
// derived names all fit DNS-1123 labels; Keeper names build on the base, so
// their room extends by the suffix the base gives back.
func MaxSpecNameLength() int {
	chiRoom := validation.DNS1123LabelMaxLength -
		len(perHostConfigVolumeName("", maxExpectedHostOrdinal, maxExpectedHostOrdinal))
	chkRoom := validation.DNS1123LabelMaxLength -
		len(keeper.PerHostConfigVolumeName("", maxExpectedHostOrdinal, maxExpectedHostOrdinal)) + len(defaultNameSuffix)
	return min(chiRoom, chkRoom)
}

// DefaultSpecName derives the managed ClickHouse name for a CR instance,
// shortening it when the plain form would overflow the derived-name budget.
func DefaultSpecName(crName, instanceKey string) string {
	return common.FitDefaultInfraName(common.InstanceBaseName(crName, instanceKey), defaultNameSuffix, MaxSpecNameLength())
}

// ValidateDerivedNames reports why a spec name cannot be deployed: derived
// per-host volume names must fit DNS-1123 labels, and the Altinity operator
// wedges silently when they don't. Nil when every derived name fits.
func ValidateDerivedNames(spec *apiv2.ManagedClickHouseSpec) error {
	for _, derived := range []string{
		keeper.PerHostConfigVolumeName(baseName(spec.Name), maxExpectedHostOrdinal, maxExpectedHostOrdinal),
		perHostConfigVolumeName(spec.Name, maxExpectedHostOrdinal, maxExpectedHostOrdinal),
	} {
		// derived length grows 1:1 with the name, so excess → max usable length
		if over := len(derived) - validation.DNS1123LabelMaxLength; over > 0 {
			return fmt.Errorf(
				"managed ClickHouse name %q cannot be deployed: the Altinity operator derives object name %q from it, which exceeds the %d-character DNS-1123 label limit; use at most %d characters, e.g. by shortening the CR name or setting spec.clickhouse.managedClickhouse.name",
				spec.Name, derived, validation.DNS1123LabelMaxLength, len(spec.Name)-over,
			)
		}
		if errs := validation.IsDNS1123Label(derived); len(errs) > 0 {
			return fmt.Errorf(
				"managed ClickHouse name %q cannot be deployed: the Altinity operator derives object name %q from it, which is not a valid DNS-1123 label (%s)",
				spec.Name, derived, strings.Join(errs, "; "),
			)
		}
	}
	return nil
}
