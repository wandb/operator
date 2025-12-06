package percona

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	transcommon "github.com/wandb/operator/internal/controller/translator/common"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readConnectionDetails(actual *pxcv1.PerconaXtraDBCluster, specNamespacedName types.NamespacedName) *mysqlConnInfo {
	namespace := specNamespacedName.Namespace
	var mysqlHost string

	if actual.Spec.ProxySQLEnabled() {
		mysqlHost = fmt.Sprintf("%s.%s.svc.cluster.local", actual.Name, namespace)
	} else {
		mysqlHost = fmt.Sprintf("%s.%s.svc.cluster.local", actual.Name, namespace)
	}

	mysqlPort := strconv.Itoa(3306)

	return &mysqlConnInfo{
		Host: mysqlHost,
		Port: mysqlPort,
		User: "root",
	}
}

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) ([]transcommon.MySQLCondition, error) {
	var err error
	var results []transcommon.MySQLCondition
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

	connInfo := readConnectionDetails(actual, specNamespacedName)

	var connection *transcommon.MySQLConnection
	if connection, err = writeMySQLConnInfo(
		ctx, client, wandbOwner, nsNameBldr, connInfo,
	); err != nil {
		return results, err
	}

	results = append(results, transcommon.NewMySQLConnCondition(*connection))

	return results, nil
}
