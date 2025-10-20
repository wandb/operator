package wandb_v2

import (
	"context"
	"fmt"
	"time"

	pxcv1 "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

type BackupState struct {
	BackupName  string
	StartedAt   *metav1.Time
	CompletedAt *metav1.Time
	State       string
}

type PerconaBackupExecutor struct {
	Client         client.Client
	ClusterName    string
	Namespace      string
	StorageName    string
	TimeoutSeconds int
}

type BackupResult struct {
	InProgress   bool
	Completed    bool
	Failed       bool
	Message      string
	RequeueAfter time.Duration
}

func (p *PerconaBackupExecutor) EnsureBackup(ctx context.Context, currentState *BackupState) (*BackupState, BackupResult, error) {
	if currentState == nil || currentState.BackupName == "" {
		return p.startNewBackup(ctx)
	}

	return p.checkBackupStatus(ctx, currentState)
}

func (p *PerconaBackupExecutor) startNewBackup(ctx context.Context) (*BackupState, BackupResult, error) {
	log := ctrl.LoggerFrom(ctx)

	backupName := fmt.Sprintf("%s-backup-%d", p.ClusterName, time.Now().Unix())
	now := metav1.Now()

	backup := &pxcv1.PerconaXtraDBClusterBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: p.Namespace,
		},
		Spec: pxcv1.PXCBackupSpec{
			PXCCluster:  p.ClusterName,
			StorageName: p.StorageName,
		},
	}

	log.Info("Creating Percona backup", "backupName", backupName, "cluster", p.ClusterName)

	if err := p.Client.Create(ctx, backup); err != nil {
		if machErrors.IsAlreadyExists(err) {
			log.Info("Backup already exists, checking status", "backupName", backupName)
			return p.checkBackupStatus(ctx, &BackupState{
				BackupName: backupName,
				StartedAt:  &now,
				State:      "creating",
			})
		}
		return nil, BackupResult{Failed: true, Message: fmt.Sprintf("failed to create backup: %v", err)}, err
	}

	state := &BackupState{
		BackupName: backupName,
		StartedAt:  &now,
		State:      "creating",
	}

	return state, BackupResult{
		InProgress:   true,
		Message:      "Backup created, waiting for completion",
		RequeueAfter: 30 * time.Second,
	}, nil
}

func (p *PerconaBackupExecutor) checkBackupStatus(ctx context.Context, currentState *BackupState) (*BackupState, BackupResult, error) {
	log := ctrl.LoggerFrom(ctx)

	backup := &pxcv1.PerconaXtraDBClusterBackup{}
	err := p.Client.Get(ctx, types.NamespacedName{
		Name:      currentState.BackupName,
		Namespace: p.Namespace,
	}, backup)

	if err != nil {
		if machErrors.IsNotFound(err) {
			return currentState, BackupResult{
				Failed:  true,
				Message: "Backup resource not found",
			}, fmt.Errorf("backup not found: %w", err)
		}
		return currentState, BackupResult{
			Failed:  true,
			Message: fmt.Sprintf("Failed to get backup status: %v", err),
		}, err
	}

	timeout := p.TimeoutSeconds
	if timeout == 0 {
		timeout = 600
	}

	elapsed := time.Since(currentState.StartedAt.Time)
	if elapsed > time.Duration(timeout)*time.Second {
		return currentState, BackupResult{
			Failed:  true,
			Message: fmt.Sprintf("Backup timed out after %v", elapsed),
		}, fmt.Errorf("backup timed out")
	}

	log.Info("Backup status", "state", backup.Status.State, "backup", currentState.BackupName, "elapsed", elapsed)

	currentState.State = string(backup.Status.State)

	switch backup.Status.State {
	case pxcv1.BackupSucceeded:
		now := metav1.Now()
		currentState.CompletedAt = &now
		log.Info("Backup completed successfully", "backup", currentState.BackupName, "duration", elapsed)
		return currentState, BackupResult{
			Completed: true,
			Message:   "Backup completed successfully",
		}, nil

	case pxcv1.BackupFailed:
		return currentState, BackupResult{
			Failed:  true,
			Message: fmt.Sprintf("Backup failed: %s", backup.Status.State),
		}, fmt.Errorf("backup failed")

	default:
		return currentState, BackupResult{
			InProgress:   true,
			Message:      fmt.Sprintf("Backup in progress: %s", backup.Status.State),
			RequeueAfter: 30 * time.Second,
		}, nil
	}
}
