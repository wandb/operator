package altinity

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	transcommon "github.com/wandb/operator/internal/controller/translator"
	chiv1 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readConnectionDetails(specNamespacedName types.NamespacedName) *clickhouseConnInfo {
	clickhouseHost := fmt.Sprintf("%s.%s.svc.cluster.local", ServiceName, specNamespacedName.Namespace)
	clickhousePort := strconv.Itoa(ClickHouseNativePort)

	return &clickhouseConnInfo{
		Host: clickhouseHost,
		Port: clickhousePort,
		User: ClickHouseUser,
	}
}

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) ([]transcommon.ClickHouseCondition, error) {
	var err error
	var results []transcommon.ClickHouseCondition
	var actual = &chiv1.ClickHouseInstallation{}

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.InstallationNsName(), ResourceTypeName, actual,
	); err != nil {
		return results, err
	}

	if actual == nil {
		return results, nil
	}

	connInfo := readConnectionDetails(specNamespacedName)

	var connection *transcommon.ClickHouseConnection
	if connection, err = writeClickHouseConnInfo(
		ctx, client, wandbOwner, nsNameBldr, connInfo,
	); err != nil {
		return results, err
	}

	results = append(results, transcommon.NewClickHouseConnCondition(*connection))

	return results, nil
}
