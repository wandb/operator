package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysql/mysql"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	v2 "github.com/wandb/operator/pkg/vendored/mysql-operator/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func ToMysqlMySQLVendorSpec(
	ctx context.Context,
	spec apiv2.WBMySQLSpec,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*v2.InnoDBCluster, error) {
	ctx, log := logx.WithSlog(ctx, logx.Mysql)

	if !spec.Enabled {
		return nil, nil
	}

	specName := spec.Name
	nsnBuilder := mysql.CreateNsNameBuilder(types.NamespacedName{
		Name:      specName,
		Namespace: spec.Namespace,
	})

	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

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
			DatadirVolumeClaimTemplate: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: storageQuantity,
						},
					},
				},
			},
		},
	}

	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
		innodb.Spec.PodSpec = &corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:      "mysql",
					Resources: spec.Config.Resources,
				},
			},
		}
	}

	if spec.Affinity != nil || spec.Tolerations != nil {
		if innodb.Spec.PodSpec == nil {
			innodb.Spec.PodSpec = &corev1.PodSpec{}
		}
		innodb.Spec.PodSpec.Affinity = spec.Affinity
		if spec.Tolerations != nil {
			innodb.Spec.PodSpec.Tolerations = *spec.Tolerations
		}
	}

	if err := ctrl.SetControllerReference(wandb, innodb, scheme); err != nil {
		log.Error("failed to set owner reference on InnoDBCluster CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return innodb, nil
}

func BuildWandbMysqlLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return BuildWandbLabels(wandb, translator.MysqlModuleName)
}

func ToMysqlOnDeleteRule(wandb *apiv2.WeightsAndBiases, retentionPolicy apiv2.WBRetentionPolicy) translator.OnDeleteRule {
	return ToOnDeleteRule(wandb, retentionPolicy, translator.MysqlModuleName)
}
