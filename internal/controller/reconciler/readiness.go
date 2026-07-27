package reconciler

import (
	"context"
	"fmt"
	"sort"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	readyConditionType = "Ready"

	migrationPhaseRunning   = "Running"
	migrationPhaseFailed    = "Failed"
	migrationPhaseSucceeded = "Succeeded"
	migrationPhaseUnknown   = "Unknown"
)

func setReadyStatus(wandb *apiv2.WeightsAndBiases, ready bool, reason, message string) {
	wandb.Status.Ready = ready
	status := metav1.ConditionFalse
	if ready {
		status = metav1.ConditionTrue
	}
	apimeta.SetStatusCondition(&wandb.Status.Conditions, metav1.Condition{
		Type:               readyConditionType,
		Status:             status,
		ObservedGeneration: wandb.Generation,
		Reason:             reason,
		Message:            message,
	})
}

func updateReadyStatus(
	ctx context.Context,
	c ctrlclient.Client,
	wandb *apiv2.WeightsAndBiases,
	statusBefore apiv2.WeightsAndBiasesStatus,
	ready bool,
	reason string,
	message string,
) error {
	setReadyStatus(wandb, ready, reason, message)
	return updateWandbStatusIfChanged(ctx, c, wandb, statusBefore)
}

func infrastructureBlockers(wandb *apiv2.WeightsAndBiases) []string {
	var blockers []string
	for key := range wandb.Spec.Redis {
		if !wandb.Status.RedisStatus[key].Ready {
			blockers = append(blockers, "redis/"+key)
		}
	}
	for key := range wandb.Spec.MySQL {
		if !wandb.Status.MySQLStatus[key].Ready {
			blockers = append(blockers, "mysql/"+key)
		}
	}
	if wandb.Spec.Kafka.ManagedKafka != nil && !wandb.Status.KafkaStatus.Ready {
		blockers = append(blockers, "kafka")
	}
	for key := range wandb.Spec.ObjectStore {
		if !wandb.Status.ObjectStoreStatus[key].Ready {
			blockers = append(blockers, "objectStore/"+key)
		}
	}
	for key := range wandb.Spec.ClickHouse {
		if !wandb.Status.ClickHouseStatus[key].Ready {
			blockers = append(blockers, "clickhouse/"+key)
		}
	}
	sort.Strings(blockers)
	return blockers
}

func mysqlInitializationReadiness(wandb *apiv2.WeightsAndBiases) (string, string) {
	var failed []string
	var pending []string
	for key, spec := range wandb.Spec.MySQL {
		if spec.ManagedMysql == nil {
			continue
		}
		status := wandb.Status.Wandb.MySQLInit[key]
		switch {
		case status.Succeeded:
		case status.Failed:
			failed = append(failed, key)
		default:
			pending = append(pending, key)
		}
	}
	sort.Strings(failed)
	sort.Strings(pending)
	if len(failed) > 0 {
		return "MySQLInitializationFailed", "MySQL initialization jobs failed: " + strings.Join(failed, ", ")
	}
	return "MySQLInitializationPending", "waiting for MySQL initialization jobs: " + strings.Join(pending, ", ")
}

func migrationReadiness(wandb *apiv2.WeightsAndBiases) (string, string) {
	status := wandb.Status.Wandb.Migration
	if status.Phase != migrationPhaseFailed && status.Reason != migrationPhaseFailed {
		phase := status.Phase
		if phase == "" {
			phase = status.Reason
		}
		if phase == "" {
			phase = migrationPhaseUnknown
		}
		return "MigrationPending", fmt.Sprintf("migration phase is %s for version %s", phase, status.Version)
	}

	var failures []string
	for name, job := range status.Jobs {
		if !job.Failed && job.Phase != migrationPhaseFailed {
			continue
		}
		detail := job.Name
		if detail == "" {
			detail = name
		}
		switch {
		case job.Message != "":
			detail += ": " + job.Message
		case job.Reason != "":
			detail += ": " + job.Reason
		}
		failures = append(failures, detail)
	}
	sort.Strings(failures)
	if len(failures) == 0 {
		return "MigrationFailed", "one or more migration jobs failed"
	}
	return "MigrationFailed", "migration jobs failed: " + strings.Join(failures, "; ")
}
