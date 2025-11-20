package percona

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator/common"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetConditions(
	ctx context.Context,
	client client.Client,
	namespacedName types.NamespacedName,
) ([]common.InfraStatusDetail, error) {
	var err error
	var results []common.InfraStatusDetail
	var actual = &pxcv1.PerconaXtraDBCluster{}

	if err = ctrlcommon.GetResource(
		ctx, client, namespacedName, ResourceTypeName, actual,
	); err != nil {
		return results, err
	}

	if actual == nil {
		return results, nil
	}

	///////////
	// Extract connection info from PXC CR
	// Connection endpoint depends on configuration:
	// - Dev (no ProxySQL): connect directly to PXC service
	// - HA (with ProxySQL): connect via ProxySQL service
	namespace := namespacedName.Namespace
	var mysqlHost string

	if actual.Spec.ProxySQLEnabled() {
		// HA mode: connect via ProxySQL
		mysqlHost = fmt.Sprintf("%s.%s.svc.cluster.local", actual.Name, namespace)
	} else {
		// Dev mode: connect directly to PXC
		mysqlHost = fmt.Sprintf("%s.%s.svc.cluster.local", actual.Name, namespace)
	}

	mysqlPort := strconv.Itoa(3306)

	connInfo := common.MySQLConnInfo{
		Host: mysqlHost,
		Port: mysqlPort,
		User: "root",
	}
	results = append(results, common.NewMySQLConnDetail(connInfo))
	///////////

	return results, nil
}
