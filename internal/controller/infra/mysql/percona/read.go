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

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
) ([]common.MySQLCondition, error) {
	var err error
	var results []common.MySQLCondition
	var actual = &pxcv1.PerconaXtraDBCluster{}

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.ClusterNsName(), ResourceTypeName, actual,
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
	namespace := specNamespacedName.Namespace
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
	results = append(results, common.NewMySQLConnCondition(connInfo))
	///////////

	return results, nil
}
