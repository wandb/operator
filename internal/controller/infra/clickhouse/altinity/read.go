package altinity

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator/common"
	chiv1 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
) ([]common.ClickHouseCondition, error) {
	var err error
	var results []common.ClickHouseCondition
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

	///////////
	// Extract connection info from CHI CR
	// Connection format: clickhouse-wandb-clickhouse.{namespace}.svc.cluster.local:9000
	clickhouseHost := fmt.Sprintf("%s.%s.svc.cluster.local", ServiceName, specNamespacedName.Namespace)
	clickhousePort := strconv.Itoa(ClickHouseNativePort)

	connInfo := common.ClickHouseConnInfo{
		Host: clickhouseHost,
		Port: clickhousePort,
		User: ClickHouseUser,
	}
	results = append(results, common.NewClickHouseConnCondition(connInfo))
	///////////

	return results, nil
}
