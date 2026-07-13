package seaweedfs

import (
	"fmt"
	"strings"

	"github.com/wandb/operator/internal/controller/common"
	"k8s.io/apimachinery/pkg/types"
)

// MaxSpecNameLength is a conservative budget for the object-store spec name:
// the seaweedfs operator derives per-component workloads and Services by
// suffixing the Seaweed name ("-master", "-volume", "-filer", peer Services,
// pod ordinals). 40 leaves 23 characters of headroom inside the 63-char
// DNS-1123 label limit, more than the longest known combination.
const MaxSpecNameLength = 40

const defaultNameSuffix = "-seaweedfs"

// DefaultSpecName derives the managed object-store name for a CR, shortening
// it when the plain "<name>-seaweedfs" would leave derived names past the
// budget. The suffix is preserved so ConnectionName can still strip it.
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
