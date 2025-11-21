package strimzi

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

const (
	// Kafka version and metadata
	KafkaVersion         = "4.1.0"
	KafkaMetadataVersion = "4.1-IV0"

	ConnectionName = "wandb-kafka-connection"

	// Listener configuration
	PlainListenerName = "plain"
	PlainListenerPort = 9092
	TLSListenerName   = "tls"
	TLSListenerPort   = 9093
	ListenerType      = "internal"

	// NodePool roles (for KRaft mode)
	RoleBroker     = "broker"
	RoleController = "controller"

	// Storage configuration
	StorageType        = "persistent-claim"
	StorageDeleteClaim = false
)

const (
	KafkaResourceType    = "Kafka"
	NodePoolResourceType = "KafkaNodePool"
)

func KafkaName(specName string) string {
	return specName
}

func NodePoolName(specName string) string {
	return fmt.Sprintf("%s-node-pool", specName)
}

func KafkaNamespacedName(specNamespacedName types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: specNamespacedName.Namespace,
		Name:      KafkaName(specNamespacedName.Name),
	}
}

func NodePoolNamespacedName(specNamespacedName types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: specNamespacedName.Namespace,
		Name:      NodePoolName(specNamespacedName.Name),
	}
}
