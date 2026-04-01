package v2

import (
	"context"
	"encoding/json"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/managed/mysql/mariadb"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/vendored/mariadb-operator/k8s.mariadb.com/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func createMySQLExporterMariaDBContainers(telemetry apiv2.Telemetry, clusterName string) []v1alpha1.Container {
	if !telemetry.Enabled {
		return nil
	}

	internalSecretName := fmt.Sprintf("internal-%s", clusterName)

	return []v1alpha1.Container{
		{
			Name:            "mysqld-exporter",
			Image:           DefaultMySQLExporterImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"/bin/sh", "-c"},
			Args: []string{`
cat > /tmp/.my.cnf <<EOF
[client]
user=monitor
password=${MYSQLD_EXPORTER_PASSWORD}
host=localhost
port=3306
EOF
exec /bin/mysqld_exporter --config.my-cnf=/tmp/.my.cnf
`},
			Env: []v1alpha1.EnvVar{
				{
					Name: "MYSQLD_EXPORTER_PASSWORD",
					ValueFrom: &v1alpha1.EnvVarSource{
						SecretKeyRef: &v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: internalSecretName,
							},
							Key: "monitor",
						},
					},
				},
			},
		},
	}
}

func ToMariaDBMySQLVendorSpec(
	ctx context.Context,
	spec apiv2.ManagedMysqlSpec,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*v1alpha1.MariaDB, error) {
	_, log := logx.WithSlog(ctx, logx.Mysql)

	specName := spec.Name
	nsnBuilder := mariadb.CreateNsNameBuilder(types.NamespacedName{
		Name:      specName,
		Namespace: spec.Namespace,
	})

	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

	mariaDB := &v1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.ClusterName(),
			Namespace: spec.Namespace,
		},
		Spec: v1alpha1.MariaDBSpec{
			Replicas:        spec.Replicas,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Storage: v1alpha1.Storage{
				Size: &storageQuantity,
			},
			Database: ptr.To("wandb_local"),
			Username: ptr.To("wandb_local"),
			PasswordSecretKeyRef: &v1alpha1.GeneratedSecretKeyRef{
				SecretKeySelector: v1alpha1.SecretKeySelector{
					LocalObjectReference: v1alpha1.LocalObjectReference{
						Name: fmt.Sprintf("%s-%s", spec.Name, "db-password"),
					},
					Key: "password",
				},
			},
		},
	}

	if spec.Replicas > 1 {
		mariaDB.Spec.Galera = &v1alpha1.Galera{
			Enabled: true,
		}
	}

	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
		mariaDB.Spec.Resources = &v1alpha1.ResourceRequirements{
			Requests: spec.Config.Resources.Requests,
			Limits:   spec.Config.Resources.Limits,
		}
	}

	if spec.Affinity != nil {
		mariaDB.Spec.Affinity = &v1alpha1.AffinityConfig{}
		if spec.Affinity.NodeAffinity != nil {
			b, _ := json.Marshal(spec.Affinity.NodeAffinity)
			var nodeAffinity v1alpha1.NodeAffinity
			if err := json.Unmarshal(b, &nodeAffinity); err == nil {
				mariaDB.Spec.Affinity.NodeAffinity = &nodeAffinity
			}
		}
		if spec.Affinity.PodAntiAffinity != nil {
			b, _ := json.Marshal(spec.Affinity.PodAntiAffinity)
			var podAntiAffinity v1alpha1.PodAntiAffinity
			if err := json.Unmarshal(b, &podAntiAffinity); err == nil {
				mariaDB.Spec.Affinity.PodAntiAffinity = &podAntiAffinity
			}
		}
	}

	if spec.Telemetry.Enabled {
		mariaDB.Spec.Metrics = &v1alpha1.MariadbMetrics{
			Exporter: v1alpha1.Exporter{
				Image: DefaultMySQLExporterImage,
			},
		}
		mariaDB.Spec.SidecarContainers = createMySQLExporterMariaDBContainers(spec.Telemetry, nsnBuilder.ClusterName())
	}

	if err := ctrl.SetControllerReference(owner, mariaDB, scheme); err != nil {
		log.Error("failed to set owner reference on MariaDB CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return mariaDB, nil
}
