package mysql

import (
	"context"
	"fmt"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	apiv2 "github.com/wandb/operator/api/v2"
	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readConnectionDetails(ctx context.Context, c client.Client, actual *mocov1beta2.MySQLCluster, nn types.NamespacedName) *mysqlConnInfo {
	log := logx.GetSlog(ctx)

	cred := &corev1.Secret{}
	secretName := "moco-" + nn.Name
	if err := c.Get(ctx, types.NamespacedName{Name: secretName, Namespace: nn.Namespace}, cred); err != nil {
		log.Error("Failed to get Moco credentials", logx.ErrAttr(err), "Secret", secretName)
		return nil
	}
	pw := string(cred.Data["WRITABLE_PASSWORD"])
	if pw == "" {
		return nil
	}
	return &mysqlConnInfo{
		Host:     fmt.Sprintf("moco-%s-primary.%s.svc.cluster.local", actual.Name, actual.Namespace),
		Port:     "3306",
		User:     "moco-writable",
		Database: "wandb_local",
		Password: pw,
	}
}

func ReadState(
	ctx context.Context,
	k8sClient client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
	onDeleteRule ctrlcommon.OnDeleteRule,
) ([]metav1.Condition, *apiv2.MysqlConnection) {
	ctx, _ = logx.WithSlog(ctx, logx.Mysql)
	log := logx.GetSlog(ctx)

	var actual = &mocov1beta2.MySQLCluster{}
	conditions := make([]metav1.Condition, 0)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := ctrlcommon.GetResource(
		ctx, k8sClient, nsnBuilder.ClusterNsName(), ResourceTypeName, actual,
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
		if onDeleteRule.Policy == ctrlcommon.Purge {
			log.Debug(
				"Attempting to purge associated mysql resources after deletion",
				"tenantName", nsnBuilder.ClusterName(),
			)
			err = purgeAssociatedResources(ctx, k8sClient, specNamespacedName.Namespace, onDeleteRule.Selector)
			if err != nil {
				conditions = append(
					conditions,
					metav1.Condition{
						Type:   MySQLCustomResourceType,
						Status: metav1.ConditionUnknown,
						Reason: ctrlcommon.ApiErrorReason,
					},
				)
			} else {
				conditions = append(conditions, metav1.Condition{
					Type:   MySQLCustomResourceType,
					Status: metav1.ConditionFalse,
					Reason: ctrlcommon.PendingDeleteReason,
				},
				)
			}
		}
	}

	var connection *apiv2.MysqlConnection

	if actual != nil {
		connInfo := readConnectionDetails(ctx, k8sClient, actual, specNamespacedName)

		connection, err = writeMySQLConnInfo(
			ctx, k8sClient, wandbOwner, nsnBuilder, connInfo,
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

func computeMySQLReportedReadyCondition(_ context.Context, clusterCR *mocov1beta2.MySQLCluster) []metav1.Condition {
	if clusterCR == nil {
		return []metav1.Condition{}
	}
	for _, c := range clusterCR.Status.Conditions {
		if c.Type == "Healthy" && c.Status == metav1.ConditionTrue {
			return []metav1.Condition{{Type: MySQLReportedReadyType, Status: metav1.ConditionTrue, Reason: "Online"}}
		}
	}
	return []metav1.Condition{{Type: MySQLReportedReadyType, Status: metav1.ConditionFalse, Reason: "NotReady"}}

}
