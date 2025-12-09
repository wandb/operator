package strimzi

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

func (n *NsNameBuilder) KafkaName() string {
	return n.SpecName()
}

func (n *NsNameBuilder) KafkaNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.KafkaName(),
	}
}

func (n *NsNameBuilder) NodePoolName() string {
	return fmt.Sprintf("%s-node-pool", n.SpecName())
}

func (n *NsNameBuilder) NodePoolNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.NodePoolName(),
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
