package wandb_v2

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
)

type DatabaseBackupExecutor interface {
	Backup(ctx context.Context) error
}

type NoOpBackupExecutor struct {
	DatabaseName string
	Namespace    string
}

func (n *NoOpBackupExecutor) Backup(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info(
		"Skipping database backup - no-op executor",
		"database", n.DatabaseName,
		"namespace", n.Namespace,
	)
	return nil
}
