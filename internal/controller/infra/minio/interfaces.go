package minio

import (
	"context"

	"github.com/wandb/operator/internal/controller/translator/common"
)

type Minio interface {
	IsStandalone() bool
	IsHighAvailability() bool
}

type ActualMinio interface {
	Upsert(ctx context.Context, minioConfig common.MinioConfig) *common.Results
	Delete(ctx context.Context) *common.Results
}

type DesiredMinio interface{}
