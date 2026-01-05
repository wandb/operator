package strimzi

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
	KafkaResourceType     = "Kafka"
	NodePoolResourceType  = "KafkaNodePool"
	AppConnTypeName       = "KafkaAppConn"
	AppBackupConnTypeName = "KafkaBackupAppConn"
)
