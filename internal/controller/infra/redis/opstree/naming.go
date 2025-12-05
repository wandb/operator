package opstree

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
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
	return fmt.Sprintf("%s", n.SpecName())
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

// Internal function for backward compatibility within the package
func createNsNameBuilder(baseNsName types.NamespacedName) *NsNameBuilder {
	return CreateNsNameBuilder(baseNsName)
}
