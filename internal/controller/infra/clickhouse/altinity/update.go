package altinity

import (
	"context"
	"fmt"
	"strconv"

	chiv1 "github.com/wandb/operator/api/altinity-clickhouse-vendored/clickhouse.altinity.com/v1"
	"github.com/wandb/operator/internal/model"
)

func (a *altinityClickHouse) updateCHI(
	ctx context.Context, desiredCHI *chiv1.ClickHouseInstallation, clickhouseConfig model.ClickHouseConfig,
) *model.Results {
	results := model.InitResults()

	// Extract connection info from CHI CR
	// Connection format: clickhouse-wandb-clickhouse.{namespace}.svc.cluster.local:9000
	namespace := a.chi.Namespace
	clickhouseHost := fmt.Sprintf("%s.%s.svc.cluster.local", ServiceName, namespace)
	clickhousePort := strconv.Itoa(ClickHouseNativePort)

	connInfo := model.ClickHouseConnInfo{
		Host: clickhouseHost,
		Port: clickhousePort,
		User: ClickHouseUser,
	}
	results.AddStatuses(model.NewClickHouseConnDetail(connInfo))

	return results
}
