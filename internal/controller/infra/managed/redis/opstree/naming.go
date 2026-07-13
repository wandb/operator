package opstree

import (
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	"k8s.io/apimachinery/pkg/types"
)

// MaxSpecNameLength is a conservative budget for the Redis spec name. This
// package derives CR names by suffixing it (ReplicationName adds "-replica"),
// and the opstree operator suffixes those further for workloads and Services
// ("-sentinel", "-headless", "-additional", pod ordinals — see the vendored
// redis-operator types). 40 leaves 23 characters of headroom inside the
// 63-char DNS-1123 label limit, more than the longest known combination.
const MaxSpecNameLength = 40

const defaultNameSuffix = "-redis"

// DefaultSpecName derives the managed Redis name for a CR, shortening it when
// the plain "<name>-redis" would leave derived names past the budget.
func DefaultSpecName(crName string) string {
	return common.FitDefaultInfraName(crName, defaultNameSuffix, MaxSpecNameLength)
}

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

func (n *NsNameBuilder) StandaloneName() string {
	return n.SpecName()
}

func (n *NsNameBuilder) StandaloneNamespace() string {
	return n.Namespace()
}

func (n *NsNameBuilder) StandaloneNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.StandaloneName(),
	}
}

func (n *NsNameBuilder) SentinelName() string {
	return n.SpecName()
}

func (n *NsNameBuilder) SentinelNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.SentinelName(),
	}
}

func (n *NsNameBuilder) ReplicationName() string {
	return fmt.Sprintf("%s-replica", n.SpecName())
}

func (n *NsNameBuilder) ReplicationNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.ReplicationName(),
	}
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
