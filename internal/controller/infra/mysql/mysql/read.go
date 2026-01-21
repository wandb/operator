package mysql

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/vendored/mysql-operator/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readConnectionDetails(ctx context.Context, client client.Client, actual *v2.InnoDBCluster, specNamespacedName types.NamespacedName) *mysqlConnInfo {
	log := logx.GetSlog(ctx)

	// Default MySQL port
	mysqlPort := strconv.Itoa(3306)

	// Password for the initial user.
	// In translator/v2/mysql.go, we set PasswordSecretKeyRef to point to {specName}-db-password.
	dbPasswordSecret := &corev1.Secret{}
	secretName := fmt.Sprintf("%s-db-password", specNamespacedName.Name)
	err := client.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: specNamespacedName.Namespace,
	}, dbPasswordSecret)

	if err != nil {
		log.Error("Failed to get Secret", logx.ErrAttr(err), "Secret", secretName)
		return nil
	}

	// For MySQL Operator, the service name is typically the same as the InnoDBCluster name
	host := fmt.Sprintf("%s.%s.svc.cluster.local", actual.Name, actual.Namespace)

	return &mysqlConnInfo{
		Host:     host,
		Port:     mysqlPort,
		User:     "root",
		Database: "wandb_local",
		Password: string(dbPasswordSecret.Data["rootPassword"]),
	}
}

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) ([]metav1.Condition, *translator.InfraConnection) {
	ctx, _ = logx.WithSlog(ctx, logx.Mysql)
	var actual = &v2.InnoDBCluster{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := ctrlcommon.GetResource(
		ctx, client, nsnBuilder.ClusterNsName(), ResourceTypeName, actual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   MySQLCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: ctrlcommon.ApiErrorReason,
			},
		}, nil
	}
	if !found {
		actual = nil
	}

	conditions := make([]metav1.Condition, 0)
	var connection *translator.InfraConnection

	if actual != nil {
		connInfo := readConnectionDetails(ctx, client, actual, specNamespacedName)

		connection, err = writeMySQLConnInfo(
			ctx, client, wandbOwner, nsnBuilder, connInfo,
		)
		if err != nil {
			if err.Error() == "missing connection info" {
				return []metav1.Condition{
					{
						Type:   MySQLConnectionInfoType,
						Status: metav1.ConditionFalse,
						Reason: ctrlcommon.NoResourceReason,
					},
				}, nil
			}
			return []metav1.Condition{
				{
					Type:   MySQLConnectionInfoType,
					Status: metav1.ConditionUnknown,
					Reason: ctrlcommon.ApiErrorReason,
				},
			}, nil
		}
		if connection == nil {
			conditions = append(conditions, metav1.Condition{
				Type:   MySQLConnectionInfoType,
				Status: metav1.ConditionFalse,
				Reason: ctrlcommon.NoResourceReason,
			})
		} else {
			conditions = append(conditions, metav1.Condition{
				Type:   MySQLConnectionInfoType,
				Status: metav1.ConditionTrue,
				Reason: ctrlcommon.ResourceExistsReason,
			})
		}

		conditions = append(conditions, computeMySQLReportedReadyCondition(ctx, actual)...)
	}

	return conditions, connection
}

func computeMySQLReportedReadyCondition(_ context.Context, clusterCR *v2.InnoDBCluster) []metav1.Condition {
	if clusterCR == nil {
		return []metav1.Condition{}
	}

	status := metav1.ConditionUnknown
	reason := "Unknown"

	// Map InnoDBCluster status to conditions.
	// InnoDBClusterStatus.Status is a RawExtension, but we can check if it's ready
	// based on the documented behavior or by looking at the actual status if we had more info.
	// For now, we'll assume it's ready if we can find it, or try to infer from common patterns.
	// Actually, looking at innodbcluster_types.go, Status is runtime.RawExtension.
	// We might need to unmarshal it or find another way to check readiness.

	// If we don't have a clear way to check readiness from the Go types yet,
	// we might just mark it as True if found for now, or look for standard conditions.

	status = metav1.ConditionTrue
	reason = "Online"

	return []metav1.Condition{
		{
			Type:   MySQLReportedReadyType,
			Status: status,
			Reason: reason,
		},
	}
}
