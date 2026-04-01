package translator

import (
	corev1 "k8s.io/api/core/v1"
)

const KafkaModuleName = "kafka"

/////////////////////////////////////////////////
// Kafka Constants

const (
	KafkaVersion         = "4.1.0"
	KafkaMetadataVersion = "4.1-IV0"
)

/////////////////////////////////////////////////
// Kafka Connection

type KafkaConnection struct {
	Host           corev1.SecretKeySelector
	Port           corev1.SecretKeySelector
	BrokerEndpoint corev1.SecretKeySelector
	URL            corev1.SecretKeySelector
}

/////////////////////////////////////////////////
// Kafka Status

type KafkaStatus struct {
	InfraStatus
	Connection KafkaConnection
}
