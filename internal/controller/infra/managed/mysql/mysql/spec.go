package mysql

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	v2 "github.com/wandb/operator/pkg/vendored/mysql-operator/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	MysqlModuleName           = "mysql"
	DefaultMySQLExporterImage = "prom/mysqld-exporter:v0.15.1"
)

func ToMysqlMySQLVendorSpec(
	ctx context.Context,
	spec apiv2.ManagedMysqlSpec,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*v2.InnoDBCluster, error) {
	_, log := logx.WithSlog(ctx, logx.Mysql)

	specName := spec.Name
	nsnBuilder := createNsNameBuilder(types.NamespacedName{
		Name:      specName,
		Namespace: spec.Namespace,
	})

	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

	mycnf := `
[mysqld]
binlog_format = 'ROW'
binlog_row_image = 'MINIMAL'
innodb_flush_log_at_trx_commit = 1
innodb_online_alter_log_max_size = 268435456
max_prepared_stmt_count = 1048576
sort_buffer_size = '67108864'
sync_binlog = 1
`

	innodb := &v2.InnoDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.ClusterName(),
			Namespace: spec.Namespace,
		},
		Spec: v2.InnoDBClusterSpec{
			SecretName:       fmt.Sprintf("%s-%s", spec.Name, "db-password"),
			Instances:        spec.Replicas,
			TLSUseSelfSigned: true,
			ImagePullPolicy:  corev1.PullIfNotPresent,
			PodLabels:        BuildWandbMysqlLabels(wandb),
			DatadirVolumeClaimTemplate: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storageQuantity,
					},
				},
			},
			Router: &v2.RouterSpec{
				Instances: spec.Replicas,
			},
			Mycnf: mycnf,
		},
	}

	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
		innodb.Spec.PodSpec = &v2.InnoDBPodSpec{
			Containers: []corev1.Container{
				{
					Name:      "mysql",
					Resources: spec.Config.Resources,
				},
			},
		}
	}

	if wandb.GetAffinity(spec.ManagedInfraSpec) != nil || wandb.GetTolerations(spec.ManagedInfraSpec) != nil {
		if innodb.Spec.PodSpec == nil {
			innodb.Spec.PodSpec = &v2.InnoDBPodSpec{}
		}
		innodb.Spec.PodSpec.Affinity = wandb.GetAffinity(spec.ManagedInfraSpec)
		if tols := wandb.GetTolerations(spec.ManagedInfraSpec); tols != nil {
			innodb.Spec.PodSpec.Tolerations = *tols
		}
	}

	if spec.Telemetry.Enabled {
		innodb.Spec.Metrics = &v2.MetricsSpec{
			Enable: true,
			Image:  DefaultMySQLExporterImage,
		}
	}

	if err := ctrl.SetControllerReference(wandb, innodb, scheme); err != nil {
		log.Error("failed to set owner reference on InnoDBCluster CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return innodb, nil
}

func BuildWandbMysqlLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return common.BuildWandbLabels(wandb, MysqlModuleName)
}

func ToMysqlOnDeleteRule(wandb *apiv2.WeightsAndBiases, retentionPolicy apiv2.RetentionPolicy) common.OnDeleteRule {
	return common.ToOnDeleteRule(wandb, retentionPolicy, MysqlModuleName)
}
