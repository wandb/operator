package mysql

import (
	"context"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	MysqlModuleName           = "mysql"
	DefaultMySQLExporterImage = "prom/mysqld-exporter:v0.15.1"
)

func ToMocoMySQLClusterSpec(
	ctx context.Context,
	spec apiv2.ManagedMysqlSpec,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*mocov1beta2.MySQLCluster, *corev1.ConfigMap, error) {
	replicas := coerceOddReplicas(spec.Replicas)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MyCnfConfigMapName(spec.Name),
			Namespace: spec.Namespace,
			Labels:    BuildWandbMysqlLabels(wandb),
		},
		Data: map[string]string{
			"sync_binlog":                    "1",
			"innodb_flush_log_at_trx_commit": "1",
			// do NOT set binlog_format/gtid_mode/semi-sync — Moco enforces these
		},
	}
	if err := controllerutil.SetControllerReference(wandb, cm, scheme); err != nil {
		return nil, nil, err
	}

	cluster := &mocov1beta2.MySQLCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: spec.Name, Namespace: spec.Namespace,
			Labels: BuildWandbMysqlLabels(wandb),
		},
		Spec: mocov1beta2.MySQLClusterSpec{
			Replicas:           replicas,
			MySQLConfigMapName: ptr.To(MyCnfConfigMapName(spec.Name)),
			PodTemplate: mocov1beta2.PodTemplateSpec{
				Spec: buildMocoPodSpec(spec.Config.Resources),
			},
			VolumeClaimTemplates: []mocov1beta2.PersistentVolumeClaim{
				{
					ObjectMeta: mocov1beta2.ObjectMeta{Name: "mysql-data"},
					Spec:       buildPVCSpec(spec.StorageSize),
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(wandb, cluster, scheme); err != nil {
		return nil, nil, err
	}
	return cluster, cm, nil
}

func coerceOddReplicas(n int32) int32 {
	switch {
	case n <= 1:
		return 1
	case n == 2:
		return 3
	case n == 4:
		return 5
	case n%2 == 0:
		return n - 1
	default:
		return n
	}
}

func buildMocoPodSpec(resources corev1.ResourceRequirements) mocov1beta2.PodSpecApplyConfiguration {
	container := corev1ac.Container().
		WithName("mysqld").
		WithImage("ghcr.io/cybozu-go/moco/mysql:8.4.8")

	if resources.Requests != nil || resources.Limits != nil {
		container = container.WithResources(
			corev1ac.ResourceRequirements().
				WithRequests(resources.Requests).
				WithLimits(resources.Limits),
		)
	}

	podSpec := corev1ac.PodSpec().WithContainers(container)
	return mocov1beta2.PodSpecApplyConfiguration(*podSpec)
}

func buildPVCSpec(storageSize string) mocov1beta2.PersistentVolumeClaimSpecApplyConfiguration {
	quantity, _ := resource.ParseQuantity(storageSize)
	pvcSpec := corev1ac.PersistentVolumeClaimSpec().
		WithAccessModes(corev1.ReadWriteOnce).
		WithResources(
			corev1ac.VolumeResourceRequirements().
				WithRequests(corev1.ResourceList{
					corev1.ResourceStorage: quantity,
				}),
		)
	return mocov1beta2.PersistentVolumeClaimSpecApplyConfiguration(*pvcSpec)
}

func BuildWandbMysqlLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return common.BuildWandbLabels(wandb, MysqlModuleName)
}

func ToMysqlOnDeleteRule(wandb *apiv2.WeightsAndBiases, retentionPolicy apiv2.RetentionPolicy) common.OnDeleteRule {
	return common.ToOnDeleteRule(wandb, retentionPolicy, MysqlModuleName)
}
