package defaults

import (
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DevMySQLStorageSize   = "1Gi"
	SmallMySQLStorageSize = "10Gi"

	SmallMySQLCpuRequest    = "500m"
	SmallMySQLCpuLimit      = "1000m"
	SmallMySQLMemoryRequest = "1Gi"
	SmallMySQLMemoryLimit   = "2Gi"

	DevPXCImage   = "perconalab/percona-xtradb-cluster-operator:main-pxc8.0"
	SmallPXCImage = "percona/percona-xtradb-cluster:8.0"

	ProxySQLImage     = "percona/proxysql2:2.7.3"
	LogCollectorImage = "perconalab/percona-xtradb-cluster-operator:main-logcollector"
	CRVersion         = "1.18.0"

	DefaultMysqlName = "wandb-mysql"
)

func BuildMySQLDefaults(size Size, ownerNamespace string) (translator.MySQLConfig, error) {
	var err error
	var storageSize string
	config := translator.MySQLConfig{
		Enabled:        true,
		Namespace:      ownerNamespace,
		Name:           DefaultMysqlName,
		DeploymentType: apiv2.MySQLTypeMariadb,
	}

	switch size {
	case SizeDev:
		storageSize = DevMySQLStorageSize
		config.StorageSize = storageSize
		config.Replicas = 1
		config.PXCImage = DevPXCImage
		config.ProxySQLEnabled = false
		config.ProxySQLReplicas = 0
		config.ProxySQLImage = ""
		config.TLSEnabled = false
		config.LogCollectorEnabled = true
		config.LogCollectorImage = LogCollectorImage
		config.AllowUnsafePXCSize = true
		config.AllowUnsafeProxySize = true
	case SizeSmall:
		storageSize = SmallMySQLStorageSize
		config.StorageSize = storageSize
		config.Replicas = 3
		config.PXCImage = SmallPXCImage
		config.ProxySQLEnabled = true
		config.ProxySQLReplicas = 3
		config.ProxySQLImage = ProxySQLImage
		config.TLSEnabled = true
		config.LogCollectorEnabled = false
		config.LogCollectorImage = ""
		config.AllowUnsafePXCSize = false
		config.AllowUnsafeProxySize = false

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallMySQLCpuRequest); err != nil {
			return config, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallMySQLCpuLimit); err != nil {
			return config, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallMySQLMemoryRequest); err != nil {
			return config, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallMySQLMemoryLimit); err != nil {
			return config, err
		}

		config.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuRequest,
				corev1.ResourceMemory: memoryRequest,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memoryLimit,
			},
		}
	default:
		return config, fmt.Errorf("unsupported size for MySQL: %s (only 'dev' and 'small' are supported)", size)
	}

	return config, nil
}
