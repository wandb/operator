package seaweedfs

import (
	"fmt"
	"strings"

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
	return fmt.Sprintf("%s-s3-config", n.SpecName())
}

func (n *NsNameBuilder) ConfigNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.ConfigName(),
	}
}

func (n *NsNameBuilder) ServiceName() string {
	return fmt.Sprintf("%s-filer", n.SpecName())
}

// SccRoleBindingName grants the default SA use of anyuid (OpenShift only).
func (n *NsNameBuilder) SccRoleBindingName() string {
	return fmt.Sprintf("%s-scc-anyuid", n.SpecName())
}

func (n *NsNameBuilder) SccRoleBindingNsName() types.NamespacedName {
	return types.NamespacedName{Namespace: n.Namespace(), Name: n.SccRoleBindingName()}
}

func (n *NsNameBuilder) connectionBaseName() string {
	return strings.TrimSuffix(n.SpecName(), "-seaweedfs")
}

func (n *NsNameBuilder) ConnectionName() string {
	return fmt.Sprintf("%s-objectstore-connection", n.connectionBaseName())
}

func (n *NsNameBuilder) ConnectionNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.ConnectionName(),
	}
}

func createNsNameBuilder(baseNsName types.NamespacedName) *NsNameBuilder {
	return CreateNsNameBuilder(baseNsName)
}

func SeaweedName(specName string) string {
	return specName
}

func ConfigName(specName string) string {
	return fmt.Sprintf("%s-s3-config", specName)
}
