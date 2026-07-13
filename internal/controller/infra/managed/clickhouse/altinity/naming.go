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

	// defaultNameSuffix is what the defaulting webhook appends to the CR name.
	// Terse ("chi" = ClickHouseInstallation) to leave the CR name as much of
	// the derived-name DNS-1123 label budget as possible. Owned by the keeper
	// package, which swaps it for "-chk" when naming the Keeper CR.
	defaultNameSuffix = keeper.ChiNameSuffix

	// assumedMaxHostOrdinal sizes name budgets when replica counts are not yet
	// known at admission (they are resolved from the server manifest, which
	// uses 1 or 3); it reserves room for up to 10 replicas per shard.
	assumedMaxHostOrdinal = 9
)

// maxHostOrdinal is the highest host ordinal to budget names for; counts that
// are unset (0) or below the assumed headroom fall back to the assumption.
func maxHostOrdinal(replicas int32) int {
	if int(replicas)-1 > assumedMaxHostOrdinal {
		return int(replicas) - 1
	}
	return assumedMaxHostOrdinal
}

// perHostConfigVolumeName mirrors the Altinity operator's per-host ConfigMap
// name for a CHI — "chi-{cr}-deploy-confd-{cluster}-{shard}-{replica}"
// (upstream pkg/model/chi/namer/patterns.go, patternConfigMapHostName). Like
// the Keeper equivalent it doubles as a StatefulSet volume name, so it must
// fit a DNS-1123 label and is the longest CHI-derived name.
func perHostConfigVolumeName(specName string, shardOrdinal, replicaOrdinal int) string {
	return fmt.Sprintf("chi-%s-deploy-confd-%s-%d-%d", specName, chiClusterName, shardOrdinal, replicaOrdinal)
}

// MaxSpecNameLength is the longest managed ClickHouse spec name of the
// defaulted "<base>-chi" shape whose derived per-host object names all fit
// DNS-1123 labels. The Keeper probe passes the bare suffix so the "-chi" →
// "-chk" swap its InstallationName performs is reflected in the budget.
func MaxSpecNameLength() int {
	chiRoom := validation.DNS1123LabelMaxLength - len(perHostConfigVolumeName("", ShardsCount-1, assumedMaxHostOrdinal))
	chkRoom := validation.DNS1123LabelMaxLength -
		len(keeper.PerHostConfigVolumeName(defaultNameSuffix, assumedMaxHostOrdinal)) + len(defaultNameSuffix)
	return min(chiRoom, chkRoom)
}

// DefaultSpecName derives the managed ClickHouse name for a CR, shortening it
// when the plain "<name>-chi" would push derived per-host names past the
// DNS-1123 label limit.
func DefaultSpecName(crName string) string {
	return common.FitDefaultInfraName(crName, defaultNameSuffix, MaxSpecNameLength())
}

// ValidateDerivedNames reports why a managed ClickHouse spec name cannot be
// deployed: the Altinity operator derives per-host StatefulSet volume names
// from it that must fit DNS-1123 labels, and it does not surface the
// apiserver's rejection when they don't — the installation just never
// converges. Returns nil when every derived name fits.
func ValidateDerivedNames(spec *apiv2.ManagedClickHouseSpec) error {
	for _, derived := range []string{
		keeper.PerHostConfigVolumeName(spec.Name, maxHostOrdinal(spec.Keeper.Replicas)),
		perHostConfigVolumeName(spec.Name, ShardsCount-1, maxHostOrdinal(spec.Replicas)),
	} {
		// Derived length grows 1:1 with the spec name, so the excess converts
		// directly into a maximum length for a name of this shape.
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
