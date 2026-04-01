package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/managed/mysql/mysql"
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

func createMySQLExporterOracleSidecar(clusterName string) corev1.Container {
	dbPasswordSecretName := fmt.Sprintf("%s-db-password", clusterName)

	return corev1.Container{
		Name:            "mysqld-exporter",
		Image:           DefaultMySQLExporterImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/bin/sh", "-c"},
		Args: []string{`
cat > /tmp/.my.cnf <<EOF
[client]
user=${MYSQLD_EXPORTER_USER}
password=${MYSQLD_EXPORTER_PASSWORD}
host=127.0.0.1
port=3306
EOF
exec /bin/mysqld_exporter --config.my-cnf=/tmp/.my.cnf
`},
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: int32(DefaultMySQLExporterPort),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name: "MYSQLD_EXPORTER_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: dbPasswordSecretName,
						},
						Key: "rootUser",
					},
				},
			},
			{
				Name: "MYSQLD_EXPORTER_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: dbPasswordSecretName,
						},
						Key: "rootPassword",
					},
				},
			},
		},
	}
}

func ToMysqlMySQLVendorSpec(
	ctx context.Context,
	spec apiv2.ManagedMysqlSpec,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*v2.InnoDBCluster, error) {
	_, log := logx.WithSlog(ctx, logx.Mysql)

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

	innodb.Spec.PodSpec = &corev1.PodSpec{
		Containers: []corev1.Container{},
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

	if wandb.GetAffinity(spec.ManagedInfraSpec) != nil || wandb.GetTolerations(spec.ManagedInfraSpec) != nil {
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

		if innodb.Spec.PodSpec == nil {
			innodb.Spec.PodSpec = &corev1.PodSpec{}
		}
		innodb.Spec.PodSpec.Containers = append(
			innodb.Spec.PodSpec.Containers,
			createMySQLExporterOracleSidecar(spec.Name),
		)
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

func ToMysqlOnDeleteRule(wandb *apiv2.WeightsAndBiases, retentionPolicy apiv2.RetentionPolicy) translator.OnDeleteRule {
	return ToOnDeleteRule(wandb, retentionPolicy, translator.MysqlModuleName)
}
