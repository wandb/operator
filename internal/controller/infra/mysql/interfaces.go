package mysql

import (
	"context"

	"github.com/wandb/operator/internal/controller/translator/common"
)

type MySQL interface {
	IsStandalone() bool
	IsHighAvailability() bool
}

type ActualMySQL interface {
	Upsert(ctx context.Context, mysqlConfig common.MySQLConfig) *common.Results
	Delete(ctx context.Context) *common.Results
}

type DesiredMySQL interface{}
