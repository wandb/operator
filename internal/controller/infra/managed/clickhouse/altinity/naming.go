package altinity

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
