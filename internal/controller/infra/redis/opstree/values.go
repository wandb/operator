package opstree

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

func StandaloneName(specName string) string {
	return specName
}

func SentinelName(specName string) string {
	return fmt.Sprintf("%s-sentinel", specName)
}

func ReplicationName(specName string) string {
	return fmt.Sprintf("%s-replication", specName)
}

func StandaloneNamespacedName(specNamespacedName types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: specNamespacedName.Namespace,
		Name:      StandaloneName(specNamespacedName.Name),
	}
}

func SentinelNamespacedName(specNamespacedName types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: specNamespacedName.Namespace,
		Name:      SentinelName(specNamespacedName.Name),
	}
}

func ReplicationNamespacedName(specNamespacedName types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: specNamespacedName.Namespace,
		Name:      ReplicationName(specNamespacedName.Name),
	}
}
