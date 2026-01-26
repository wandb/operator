package mariadb

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/vendored/mariadb-operator/k8s.mariadb.com/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readConnectionDetails(ctx context.Context, client client.Client, actual *v1alpha1.MariaDB, specNamespacedName types.NamespacedName) *mysqlConnInfo {
	log := logx.GetSlog(ctx)

	// MariaDB operator uses a service for connection.
	// The host is available in actual.Status.Host (or can be inferred from service name)
	// Default port is 3306.
	mysqlPort := strconv.Itoa(int(actual.Spec.Port))
	if mysqlPort == "0" {
		mysqlPort = "3306"
	}

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

	return &mysqlConnInfo{
		Host:     "", // TODO generate the hostname correctly for mariaDB
		Port:     mysqlPort,
		User:     "wandb_local",
		Database: "wandb_local",
		Password: string(dbPasswordSecret.Data["password"]),
	}
}

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) ([]metav1.Condition, *translator.InfraConnection) {
	ctx, _ = logx.WithSlog(ctx, logx.Mysql)
	var actual = &v1alpha1.MariaDB{}

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

func computeMySQLReportedReadyCondition(_ context.Context, clusterCR *v1alpha1.MariaDB) []metav1.Condition {
	if clusterCR == nil {
		return []metav1.Condition{}
	}

	var status metav1.ConditionStatus
	reason := "Unknown"

	// Map MariaDB status to conditions
	if meta.IsStatusConditionTrue(clusterCR.Status.Conditions, v1alpha1.ConditionTypeReady) {
		status = metav1.ConditionTrue
		reason = "Ready"
	} else {
		status = metav1.ConditionFalse
		condition := meta.FindStatusCondition(clusterCR.Status.Conditions, v1alpha1.ConditionTypeReady)
		if condition != nil && condition.Reason != "" {
			reason = condition.Reason
		}
	}

	return []metav1.Condition{
		{
			Type:   MySQLReportedReadyType,
			Status: status,
			Reason: reason,
		},
	}
}
