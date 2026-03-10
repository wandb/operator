package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysql/percona"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	pxcv1 "github.com/wandb/operator/pkg/vendored/percona-operator/pxc/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	DefaultMySQLExporterImage = "prom/mysqld-exporter:v0.15.1"
	DefaultMySQLExporterPort  = 9104
)

func createMySQLExporterSidecar(telemetry apiv2.Telemetry, clusterName string) []corev1.Container {
	if !telemetry.Enabled {
		return nil
	}

	internalSecretName := fmt.Sprintf("internal-%s", clusterName)

	return []corev1.Container{
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
			Ports: []corev1.ContainerPort{
				{
					Name:          "metrics",
					ContainerPort: int32(DefaultMySQLExporterPort),
					Protocol:      corev1.ProtocolTCP,
				},
			},
			Env: []corev1.EnvVar{
				{
					Name: "MYSQLD_EXPORTER_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: internalSecretName,
							},
							Key: "monitor",
						},
					},
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/metrics",
						Port: intstr.FromInt(DefaultMySQLExporterPort),
					},
				},
				InitialDelaySeconds: 60,
				PeriodSeconds:       10,
			},
		},
	}
}

func ToPerconaMySQLVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*pxcv1.PerconaXtraDBCluster, error) {
	ctx, log := logx.WithSlog(ctx, logx.Mysql)
	spec := wandb.Spec.MySQL

	if !spec.Enabled {
		return nil, nil
	}

	specName := spec.Name
	nsnBuilder := percona.CreateNsNameBuilder(types.NamespacedName{
		Name:      specName,
		Namespace: spec.Namespace,
	})

	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

	proxySQLEnabled := spec.Replicas > 1
	tlsEnabled := spec.Replicas > 1
	allowUnsafePXCSize := spec.Replicas == 1
	allowUnsafeProxySize := spec.Replicas == 1

	pxcImage := translator.ProdPXCImage

	configuration := `[mysqld]
pxc_strict_mode=PERMISSIVE
`

	pxc := &pxcv1.PerconaXtraDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.ClusterName(),
			Namespace: spec.Namespace,
			Labels: map[string]string{
				"app": nsnBuilder.ClusterName(),
			},
		},
		Spec: pxcv1.PerconaXtraDBClusterSpec{
			CRVersion: translator.PXCCRVersion,
			Unsafe: pxcv1.UnsafeFlags{
				PXCSize:   allowUnsafePXCSize,
				TLS:       !tlsEnabled,
				ProxySize: allowUnsafeProxySize,
			},
			PXC: &pxcv1.PXCSpec{
				PodSpec: &pxcv1.PodSpec{
					Size:  spec.Replicas,
					Image: pxcImage,
					VolumeSpec: &pxcv1.VolumeSpec{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: storageQuantity,
								},
							},
						},
					},
					Affinity: &pxcv1.PodAffinity{
						Advanced: spec.Affinity,
					},
					Tolerations:   *wandb.GetTolerations(spec.WBInfraSpec),
					Configuration: configuration,
				},
			},
			TLS: &pxcv1.TLSSpec{
				Enabled: &tlsEnabled,
			},
			Users: []pxcv1.User{{
				Name:            "wandb_local",
				DBs:             []string{"wandb_local"},
				Hosts:           []string{"%"},
				Grants:          []string{"ALL"},
				WithGrantOption: true,
				PasswordSecretRef: &pxcv1.SecretKeySelector{
					Name: fmt.Sprintf("%s-%s", specName, "db-password"),
					Key:  "password",
				},
			}},
		},
	}

	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
		pxc.Spec.PXC.PodSpec.Resources = corev1.ResourceRequirements{
			Requests: spec.Config.Resources.Requests,
			Limits:   spec.Config.Resources.Limits,
		}
	}

	if proxySQLEnabled {
		proxySQLReplicas := int32(3)
		if spec.Replicas < 3 {
			proxySQLReplicas = spec.Replicas
		}
		pxc.Spec.ProxySQL = &pxcv1.ProxySQLSpec{
			PodSpec: pxcv1.PodSpec{
				Enabled: true,
				Size:    proxySQLReplicas,
				Image:   translator.ProxySQLImage,
				VolumeSpec: &pxcv1.VolumeSpec{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}
	} else {
		pxc.Spec.HAProxy = &pxcv1.HAProxySpec{
			PodSpec: pxcv1.PodSpec{
				Enabled: false,
			},
		}
	}

	if spec.Replicas == 1 {
		pxc.Spec.LogCollector = &pxcv1.LogCollectorSpec{
			Enabled: true,
			Image:   translator.LogCollectorImg,
		}
	}

	pxc.Spec.PXC.Sidecars = createMySQLExporterSidecar(spec.Telemetry, nsnBuilder.ClusterName())

	if err := ctrl.SetControllerReference(wandb, pxc, scheme); err != nil {
		log.Error("failed to set owner reference on PXC CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return pxc, nil
}
