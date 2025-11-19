package altinity

import (
	"context"
	"fmt"
	"strconv"

	"github.com/wandb/operator/internal/controller/translator/common"
	chiv1 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
)

func (a *altinityClickHouse) updateCHI(
	ctx context.Context, desiredCHI *chiv1.ClickHouseInstallation, clickhouseConfig common.ClickHouseConfig,
) *common.Results {
	results := common.InitResults()

	// Extract connection info from CHI CR
	// Connection format: clickhouse-wandb-clickhouse.{namespace}.svc.cluster.local:9000
	namespace := a.chi.Namespace
	clickhouseHost := fmt.Sprintf("%s.%s.svc.cluster.local", ServiceName, namespace)
	clickhousePort := strconv.Itoa(ClickHouseNativePort)

	connInfo := common.ClickHouseConnInfo{
		Host: clickhouseHost,
		Port: clickhousePort,
		User: ClickHouseUser,
	}
	results.AddStatuses(common.NewClickHouseConnDetail(connInfo))

	return results
}
