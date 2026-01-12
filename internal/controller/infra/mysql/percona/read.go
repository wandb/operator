package percona

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readConnectionDetails(ctx context.Context, client client.Client, actual *pxcv1.PerconaXtraDBCluster, specNamespacedName types.NamespacedName) *mysqlConnInfo {
	log := logx.FromContext(ctx)

	mysqlPort := strconv.Itoa(3306)

	dbPasswordSecret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s", "internal", actual.Name), Namespace: specNamespacedName.Namespace}, dbPasswordSecret)
	if err != nil {
		log.Error(err, "Failed to get Secret", "Secret", fmt.Sprintf("%s-%s", specNamespacedName.Name, "user-db-password"))
		return nil
	}

	return &mysqlConnInfo{
		Host:     actual.Status.Host,
		Port:     mysqlPort,
		User:     "root",
		Database: "wandb_local",
		Password: string(dbPasswordSecret.Data["root"]),
	}
}

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) ([]metav1.Condition, *translator.InfraConnection) {
	ctx, _ = logx.IntoContext(ctx, logx.Mysql)
	var actual = &pxcv1.PerconaXtraDBCluster{}

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

func computeMySQLReportedReadyCondition(_ context.Context, clusterCR *pxcv1.PerconaXtraDBCluster) []metav1.Condition {
	if clusterCR == nil {
		return []metav1.Condition{}
	}

	status := metav1.ConditionUnknown
	reason := string(clusterCR.Status.Status)

	switch clusterCR.Status.Status {
	case pxcv1.AppStateReady:
		status = metav1.ConditionTrue
	case pxcv1.AppStateInit, pxcv1.AppStatePaused, pxcv1.AppStateStopping, pxcv1.AppStateError:
		status = metav1.ConditionFalse
	}

	return []metav1.Condition{
		{
			Type:   MySQLReportedReadyType,
			Status: status,
			Reason: reason,
		},
	}
}
