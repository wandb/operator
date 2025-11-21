package percona

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

func ClusterName(specName string) string {
	return fmt.Sprintf("%s-cluster", specName)
}

func ClusterNamespacedName(specNamespacedName types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: specNamespacedName.Namespace,
		Name:      ClusterName(specNamespacedName.Name),
	}
}
