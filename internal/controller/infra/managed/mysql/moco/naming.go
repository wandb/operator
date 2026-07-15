package moco

import (
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	"k8s.io/apimachinery/pkg/types"
)

// MaxClusterNameLength mirrors Moco's admission cap on MySQLCluster names.
const MaxClusterNameLength = 40

const defaultNameSuffix = "-mysql"

// DefaultSpecName derives the managed MySQL name for a CR instance, shortened
// to Moco's cap.
func DefaultSpecName(crName, instanceKey string) string {
	return common.FitDefaultInfraName(common.InstanceBaseName(crName, instanceKey), defaultNameSuffix, MaxClusterNameLength)
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

func (n *NsNameBuilder) ClusterName() string {
	return n.SpecName()
}

func (n *NsNameBuilder) ClusterNsName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace(),
		Name:      n.ClusterName(),
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

func createNsNameBuilder(baseNsName types.NamespacedName) *NsNameBuilder {
	return CreateNsNameBuilder(baseNsName)
}

func MyCnfConfigMapName(specName string) string {
	return specName + "-mycnf"
}
