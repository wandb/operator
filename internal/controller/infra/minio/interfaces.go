package minio

import (
	"context"

	"github.com/wandb/operator/internal/model"
)

type Minio interface {
	IsStandalone() bool
	IsHighAvailability() bool
}

type ActualMinio interface {
	Upsert(ctx context.Context, minioConfig model.MinioConfig) *model.Results
	Delete(ctx context.Context) *model.Results
}

type DesiredMinio interface{}
