package percona

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/utils"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func readConnectionDetails(ctx context.Context, client client.Client, actual *pxcv1.PerconaXtraDBCluster, specNamespacedName types.NamespacedName) *mysqlConnInfo {
	log := ctrllog.FromContext(ctx)
	//namespace := specNamespacedName.Namespace
	//var mysqlHost string

	//if actual.Spec.ProxySQLEnabled() {
	//	mysqlHost = fmt.Sprintf("%s.%s.svc.cluster.local", actual.Name, namespace)
	//} else {
	//	mysqlHost = fmt.Sprintf("%s.%s.svc.cluster.local", actual.Name, namespace)
	//}

	mysqlPort := strconv.Itoa(3306)

	dbPasswordSecret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s", "internal", actual.Name), Namespace: specNamespacedName.Namespace}, dbPasswordSecret)
	if err != nil {
		log.Error(err, "Failed to get Secret", "Secret", fmt.Sprintf("%s-%s", specNamespacedName.Name, "user-db-password"))
		return nil
	}

	return &mysqlConnInfo{
		Host:     actual.Status.Host,
		Port:     mysqlPort,
		User:     "root",
		Database: "wandb_local",
		Password: string(dbPasswordSecret.Data["root"]),
	}
}

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) (*translator.MysqlStatus, error) {
	var err error
	var found bool
	var actual = &pxcv1.PerconaXtraDBCluster{}
	var status = &translator.MysqlStatus{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	if found, err = ctrlcommon.GetResource(
		ctx, client, nsnBuilder.ClusterNsName(), ResourceTypeName, actual,
	); err != nil {
		return nil, err
	}
	if !found {
		actual = nil
	}

	if actual != nil {
		///////////////////////////////////
		// set connection details

		connInfo := readConnectionDetails(ctx, client, actual, specNamespacedName)

		var connection *translator.InfraConnection
		if connection, err = writeMySQLConnInfo(
			ctx, client, wandbOwner, nsnBuilder, connInfo,
		); err != nil {
			return nil, err
		}

		if connection != nil {
			status.Connection = *connection
		}

		///////////////////////////////////
		// add conditions

	}
	///////////////////////////////////
	// set top-level summary

	if actual != nil {
		if actual.Status.Status == pxcv1.AppStateReady {
			status.State = "Ready"
			status.Ready = true
		} else {
			status.State = utils.Capitalize(string(actual.Status.Status))
			status.Ready = false
		}
	} else {
		status.State = "Not Installed"
		status.Ready = false
	}

	return status, nil
}
