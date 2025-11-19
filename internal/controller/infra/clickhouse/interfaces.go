package clickhouse

import (
	"context"

	"github.com/wandb/operator/internal/controller/translator/common"
)

type ClickHouse interface {
	IsStandalone() bool
	IsHighAvailability() bool
}

type ActualClickHouse interface {
	Upsert(ctx context.Context, clickhouseConfig common.ClickHouseConfig) *common.Results
	Delete(ctx context.Context) *common.Results
}

type DesiredClickHouse interface{}
