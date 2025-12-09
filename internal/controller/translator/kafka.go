package translator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/////////////////////////////////////////////////
// Kafka Constants

const (
	KafkaVersion         = "4.1.0"
	KafkaMetadataVersion = "4.1-IV0"
)

/////////////////////////////////////////////////
// Kafka Status

// KafkaStatus is a representation of Status that must support round-trip translation
// between any version of WBKafkaStatus and itself
type KafkaStatus struct {
	Ready      bool
	State      string
	Conditions []metav1.Condition
	Connection InfraConnection
}
