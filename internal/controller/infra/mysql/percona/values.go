package percona

const (
	// Resource names
	PXCName        = "wandb-mysql"
	PXCServiceName = "wandb-mysql-pxc"
	ProxySQLName   = "wandb-mysql-proxysql"
	ConnectionName = "wandb-mysql-connection"
	SecretsName    = "wandb-mysql-secrets"

	// MySQL configuration
	MySQLPort     = 3306
	MySQLUser     = "root"
	MySQLDatabase = "wandb"

	// Storage configuration
	StorageType = "persistentVolumeClaim"
)
