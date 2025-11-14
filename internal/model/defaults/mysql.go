package defaults

import (
	"fmt"

	v2 "github.com/wandb/operator/api/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// Storage sizes
	DevMySQLStorageSize   = "1Gi"
	SmallMySQLStorageSize = "10Gi"

	// Resource requests/limits for small size
	SmallMySQLCpuRequest    = "500m"
	SmallMySQLCpuLimit      = "1000m"
	SmallMySQLMemoryRequest = "1Gi"
	SmallMySQLMemoryLimit   = "2Gi"

	// Percona XtraDB Cluster images
	DevPXCImage   = "perconalab/percona-xtradb-cluster-operator:main-pxc8.0"
	SmallPXCImage = "percona/percona-xtradb-cluster:8.0"

	// Component images
	ProxySQLImage     = "percona/proxysql2:2.7.3"
	LogCollectorImage = "perconalab/percona-xtradb-cluster-operator:main-logcollector"
	CRVersion         = "1.18.0"
)

func MySQL(profile v2.WBSize) (v2.WBMySQLSpec, error) {
	var err error
	var storageSize string
	spec := v2.WBMySQLSpec{
		Enabled:   true,
		Namespace: DefaultNamespace,
	}

	switch profile {
	case v2.WBSizeDev:
		storageSize = DevMySQLStorageSize
		spec.StorageSize = storageSize
	case v2.WBSizeSmall:
		storageSize = SmallMySQLStorageSize
		spec.StorageSize = storageSize

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallMySQLCpuRequest); err != nil {
			return spec, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallMySQLCpuLimit); err != nil {
			return spec, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallMySQLMemoryRequest); err != nil {
			return spec, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallMySQLMemoryLimit); err != nil {
			return spec, err
		}

		spec.Config = &v2.WBMySQLConfig{
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    cpuRequest,
					v1.ResourceMemory: memoryRequest,
				},
				Limits: v1.ResourceList{
					v1.ResourceCPU:    cpuLimit,
					v1.ResourceMemory: memoryLimit,
				},
			},
		}
	default:
		return spec, fmt.Errorf("unsupported size for MySQL: %s (only 'dev' and 'small' are supported)", profile)
	}

	return spec, nil
}
