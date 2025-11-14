package mysql

import (
	"context"

	"github.com/wandb/operator/internal/model"
)

type MySQL interface {
	IsStandalone() bool
	IsHighAvailability() bool
}

type ActualMySQL interface {
	Upsert(ctx context.Context, mysqlConfig model.MySQLConfig) *model.Results
	Delete(ctx context.Context) *model.Results
}

type DesiredMySQL interface{}
