package tenant

const (
	// Resource names
	TenantName     = "wandb-minio"
	ServiceName    = "wandb-minio-hl"
	ConnectionName = "wandb-minio-connection"
	ConfigSecret   = "wandb-minio-config"

	// Minio configuration
	MinioPort      = 443
	MinioAccessKey = "minio"
	MinioBucket    = "wandb"

	// Pool configuration
	PoolName = "pool-0"
)
