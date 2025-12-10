package percona

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
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
) (*translator.MysqlStatus, error) {
	var err error
	var actual = &pxcv1.PerconaXtraDBCluster{}
	var status = &translator.MysqlStatus{}

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.ClusterNsName(), ResourceTypeName, actual,
	); err != nil {
		return nil, err
	}

	if actual == nil {
		status.State = "Not Installed"
		status.Ready = false
		return status, nil
	}

	///////////////////////////////////
	// set connection details

	connInfo := readConnectionDetails(actual, specNamespacedName)

	var connection *translator.InfraConnection
	if connection, err = writeMySQLConnInfo(
		ctx, client, wandbOwner, nsNameBldr, connInfo,
	); err != nil {
		return nil, err
	}

	if connection != nil {
		status.Connection = *connection
	}

	///////////////////////////////////
	// add conditions

	///////////////////////////////////
	// set top-level summary

	return status, nil
}
