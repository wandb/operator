package clickhouse

import (
	"context"

	"github.com/wandb/operator/internal/model"
)

type ClickHouse interface {
	IsStandalone() bool
	IsHighAvailability() bool
}

type ActualClickHouse interface {
	Upsert(ctx context.Context, clickhouseConfig model.ClickHouseConfig) *model.Results
	Delete(ctx context.Context) *model.Results
}

type DesiredClickHouse interface{}
