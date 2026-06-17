package moco

import (
	"context"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	mococonstants "github.com/cybozu-go/moco/pkg/constants"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	MysqlModuleName           = "moco"
	MocoMySQLImage            = "ghcr.io/cybozu-go/moco/mysql:8.4.8"
	DefaultMySQLExporterImage = "prom/mysqld-exporter:v0.15.1"

	// Moco names the resulting PVCs "<dataVolumeName>-<cluster.PrefixedName()>-<ordinal>"
	// (= "<dataVolumeName>-moco-<cluster>-<n>"); ensurePVCLabels and purge rely on this.
	dataVolumeName = mococonstants.MySQLDataVolumeName
)

const (
	mocoMySQLRunAsUser  int64 = mococonstants.ContainerUID
	mocoMySQLRunAsGroup int64 = mococonstants.ContainerGID
	mocoMySQLFSGroup    int64 = mococonstants.ContainerGID

	mocoMySQLCapabilityAll corev1.Capability = "ALL"
)

func ToMocoMySQLClusterSpec(
	ctx context.Context,
	spec apiv2.ManagedMysqlSpec,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*mocov1beta2.MySQLCluster, *corev1.ConfigMap, error) {

	replicas := spec.Replicas

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MyCnfConfigMapName(spec.Name),
			Namespace: spec.Namespace,
			Labels:    BuildWandbMysqlLabels(wandb),
		},
		Data: map[string]string{
			"sync_binlog":                    "1",
			"innodb_flush_log_at_trx_commit": "1",
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
				Spec:                buildMocoPodSpec(spec.Config.Resources),
				OverwriteContainers: mocoOverwriteContainers(),
			},
			VolumeClaimTemplates: []mocov1beta2.PersistentVolumeClaim{
				{
					ObjectMeta: mocov1beta2.ObjectMeta{Name: dataVolumeName},
					Spec:       buildPVCSpec(spec.StorageSize),
				},
			},
		},
	}

	// A non-empty Collectors list makes MOCO inject the mysqld_exporter sidecar.
	if spec.Telemetry.Enabled {
		cluster.Spec.Collectors = []string{"engine_innodb_status", "info_schema.innodb_metrics"}
	}

	if err := controllerutil.SetControllerReference(wandb, cluster, scheme); err != nil {
		return nil, nil, err
	}
	return cluster, cm, nil
}

func buildMocoPodSpec(resources corev1.ResourceRequirements) mocov1beta2.PodSpecApplyConfiguration {
	container := corev1ac.Container().
		WithName("mysqld").
		WithImage(MocoMySQLImage).
		WithSecurityContext(mocoContainerSecurityContext())

	if resources.Requests != nil || resources.Limits != nil {
		container = container.WithResources(
			corev1ac.ResourceRequirements().
				WithRequests(resources.Requests).
				WithLimits(resources.Limits),
		)
	}

	podSpec := corev1ac.PodSpec().
		WithSecurityContext(mocoPodSecurityContext()).
		WithContainers(container)
	return mocov1beta2.PodSpecApplyConfiguration(*podSpec)
}

func mocoPodSecurityContext() *corev1ac.PodSecurityContextApplyConfiguration {
	securityContext := corev1ac.PodSecurityContext().
		WithRunAsNonRoot(true).
		WithSeccompProfile(corev1ac.SeccompProfile().
			WithType(corev1.SeccompProfileTypeRuntimeDefault))
	if !utils.IsOpenShift() {
		securityContext = securityContext.
			WithRunAsUser(mocoMySQLRunAsUser).
			WithRunAsGroup(mocoMySQLRunAsGroup).
			WithFSGroup(mocoMySQLFSGroup).
			WithFSGroupChangePolicy(corev1.FSGroupChangeOnRootMismatch)
	}
	return securityContext
}

func mocoContainerSecurityContext() *corev1ac.SecurityContextApplyConfiguration {
	securityContext := corev1ac.SecurityContext().
		WithRunAsNonRoot(true).
		WithAllowPrivilegeEscalation(false).
		WithCapabilities(corev1ac.Capabilities().WithDrop(mocoMySQLCapabilityAll)).
		WithSeccompProfile(corev1ac.SeccompProfile().
			WithType(corev1.SeccompProfileTypeRuntimeDefault))
	if !utils.IsOpenShift() {
		securityContext = securityContext.
			WithRunAsUser(mocoMySQLRunAsUser).
			WithRunAsGroup(mocoMySQLRunAsGroup)
	}
	return securityContext
}

func mocoOverwriteContainers() []mocov1beta2.OverwriteContainer {
	securityContext := (*mocov1beta2.SecurityContextApplyConfiguration)(mocoContainerSecurityContext())
	return []mocov1beta2.OverwriteContainer{
		{
			Name:            mocov1beta2.AgentContainerName,
			SecurityContext: securityContext.DeepCopy(),
		},
		{
			Name:            mocov1beta2.InitContainerName,
			SecurityContext: securityContext.DeepCopy(),
		},
		{
			Name:            mocov1beta2.SlowQueryLogAgentContainerName,
			SecurityContext: securityContext.DeepCopy(),
		},
		{
			Name:            mocov1beta2.ExporterContainerName,
			SecurityContext: securityContext.DeepCopy(),
		},
	}
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
