package percona

import (
	"context"
	"fmt"
	"strconv"

	"github.com/wandb/operator/internal/model"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
)

func (a *perconaPXC) updatePXC(
	ctx context.Context, desiredPXC *pxcv1.PerconaXtraDBCluster, mysqlConfig model.MySQLConfig,
) *model.Results {
	results := model.InitResults()

	// Extract connection info from PXC CR
	// Connection endpoint depends on configuration:
	// - Dev (no ProxySQL): connect directly to PXC service
	// - HA (with ProxySQL): connect via ProxySQL service
	namespace := a.pxc.Namespace
	var mysqlHost string

	if mysqlConfig.ProxySQLEnabled {
		// HA mode: connect via ProxySQL
		mysqlHost = fmt.Sprintf("%s.%s.svc.cluster.local", ProxySQLName, namespace)
	} else {
		// Dev mode: connect directly to PXC
		mysqlHost = fmt.Sprintf("%s.%s.svc.cluster.local", PXCServiceName, namespace)
	}

	mysqlPort := strconv.Itoa(MySQLPort)

	connInfo := model.MySQLConnInfo{
		Host: mysqlHost,
		Port: mysqlPort,
		User: MySQLUser,
	}
	results.AddStatuses(model.NewMySQLConnDetail(connInfo))

	return results
}
