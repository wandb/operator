package tenant

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

func (n *NsNameBuilder) SpecNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.SpecName(),
	}
}

func (n *NsNameBuilder) ConfigName() string {
	return fmt.Sprintf("%s-config", n.SpecName())
}

func (n *NsNameBuilder) ConfigNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.ConfigName(),
	}
}

func (n *NsNameBuilder) ServiceName() string {
	return fmt.Sprintf("%s-hl", n.SpecName())
}

func (n *NsNameBuilder) PoolName() string {
	return fmt.Sprintf("%s-pool", n.SpecName())
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

// Standalone helper functions for backward compatibility with translator code
func TenantName(specName string) string {
	return specName
}

func ConfigName(specName string) string {
	return fmt.Sprintf("%s-config", specName)
}

func PoolName(specName string) string {
	return fmt.Sprintf("%s-pool", specName)
}
