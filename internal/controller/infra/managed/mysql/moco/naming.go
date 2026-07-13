package moco

import (
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	"k8s.io/apimachinery/pkg/types"
)

// MaxClusterNameLength mirrors Moco's MySQLCluster admission webhook, which
// rejects names longer than 40 characters (Moco prefixes derived StatefulSets
// and CronJobs with "moco-"/"moco-backup-").
const MaxClusterNameLength = 40

const defaultNameSuffix = "-mysql"

// DefaultSpecName derives the managed MySQL name for a CR, shortening it when
// the plain "<name>-mysql" would exceed Moco's cluster-name cap.
func DefaultSpecName(crName string) string {
	return common.FitDefaultInfraName(crName, defaultNameSuffix, MaxClusterNameLength)
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
