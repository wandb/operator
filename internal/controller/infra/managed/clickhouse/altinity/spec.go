package altinity

import (
	"context"
	"crypto/sha256"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity/keeper"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	chtypes "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/common/types"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	ClickhouseModuleName = "clickhouse"

	// TODO: remove this hardcoded default once all supported manifest versions
	// supply clickhouse.<instance>.images.server.
	defaultClickHouseImage = "altinity/clickhouse-server:25.8.16.10002.altinitystable"
)

func ClickHouseImage(img manifest.ImageRef, globalImageRegistry string) string {
	if out := img.GetImage(globalImageRegistry); out != "" {
		return out
	}
	// Fallback for older manifests that don't supply the image.
	return defaultClickHouseImage
}

const (
	clickHouseRunAsUser  int64 = 101
	clickHouseRunAsGroup int64 = 101
	clickHouseFSGroup    int64 = 101

	clickHouseTmpVolumeName = "clickhouse-tmp"
	clickHouseTmpMountPath  = "/tmp"
	clickHouseLogVolumeName = "clickhouse-log"
	clickHouseLogMountPath  = "/var/log/clickhouse-server"
	clickHouseRunVolumeName = "clickhouse-run"
	clickHouseRunMountPath  = "/var/run/clickhouse-server"

	clickHouseCapabilityAll corev1.Capability = "ALL"
)

func clickHousePodSecurityContext() *corev1.PodSecurityContext {
	if utils.IsOpenShift() {
		return &corev1.PodSecurityContext{
			RunAsNonRoot:   ptr.To(true),
			SeccompProfile: clickHouseRuntimeDefaultSeccompProfile(),
		}
	}

	return &corev1.PodSecurityContext{
		RunAsUser:      ptr.To(clickHouseRunAsUser),
		RunAsGroup:     ptr.To(clickHouseRunAsGroup),
		RunAsNonRoot:   ptr.To(true),
		FSGroup:        ptr.To(clickHouseFSGroup),
		SeccompProfile: clickHouseRuntimeDefaultSeccompProfile(),
	}
}

func clickHouseContainerSecurityContext() *corev1.SecurityContext {
	securityContext := &corev1.SecurityContext{
		RunAsNonRoot:             ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{clickHouseCapabilityAll},
		},
		SeccompProfile: clickHouseRuntimeDefaultSeccompProfile(),
	}
	if !utils.IsOpenShift() {
		securityContext.RunAsUser = ptr.To(clickHouseRunAsUser)
		securityContext.RunAsGroup = ptr.To(clickHouseRunAsGroup)
	}
	return securityContext
}

func clickHouseWritableVolumes() []corev1.Volume {
	return []corev1.Volume{
		writableEmptyDirVolume(clickHouseTmpVolumeName),
		writableEmptyDirVolume(clickHouseLogVolumeName),
		writableEmptyDirVolume(clickHouseRunVolumeName),
	}
}

func clickHouseWritableVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: clickHouseTmpVolumeName, MountPath: clickHouseTmpMountPath},
		{Name: clickHouseLogVolumeName, MountPath: clickHouseLogMountPath},
		{Name: clickHouseRunVolumeName, MountPath: clickHouseRunMountPath},
	}
}

func clickHouseRuntimeDefaultSeccompProfile() *corev1.SeccompProfile {
	return &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
}

func writableEmptyDirVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// ToClickHouseVendorSpec converts a ClickHouseSpec to a ClickHouseInstallation CR.
// This function translates the high-level ClickHouse spec into the vendor-specific
// ClickHouseInstallation format used by the Altinity operator.
func ToClickHouseVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
	objStorage *ObjectStorageConn,
	mfst manifest.Manifest,
) (*v1.ClickHouseInstallation, error) {
	_, log := logx.WithSlog(ctx, logx.ClickHouse)
	spec := wandb.Spec.ClickHouse.ManagedClickHouse

	if spec == nil {
		return nil, nil
	}

	// Managed ClickHouse stores table data in the object store; require it.
	if objStorage == nil {
		return nil, fmt.Errorf("managed ClickHouse requires object storage, but none was resolved")
	}

	nsnBuilder := CreateNsNameBuilder(types.NamespacedName{
		Namespace: spec.Namespace, Name: spec.Name,
	})

	// This PV holds only metadata/system tables and the S3 read-through cache;
	// table data lives in the bucket.
	storageQuantity := resource.MustParse(spec.StorageSize)

	// Reserve ~20% of the local PV for metadata/system; the rest backs the cache.
	cacheMaxSizeBytes := storageQuantity.Value() * 8 / 10

	// Create user settings with password
	passwordSha256 := fmt.Sprintf("%x", sha256.Sum256([]byte(ClickHousePassword)))
	userSettings := v1.NewSettings()
	userSettings.Set(
		fmt.Sprintf("%s/password_sha256_hex", ClickHouseUser),
		v1.NewSettingScalar(passwordSha256),
	)
	userSettings.Set(
		fmt.Sprintf("%s/networks/ip", ClickHouseUser),
		v1.NewSettingScalar("::/0"),
	)
	userSettings.Set(
		fmt.Sprintf("%s/allow_databases/database", ClickHouseUser),
		v1.NewSettingVector([]string{ClickHouseDatabase, "db_management"}),
	)

	// Create server settings
	serverSettings := v1.NewSettings()

	// Define the S3 disk + cache + storage policy and make it the server-wide default.
	applyStorageConfiguration(serverSettings, objStorage, cacheMaxSizeBytes)

	// Enable built-in Prometheus metrics endpoint if telemetry is enabled
	if spec.Telemetry.Enabled {
		serverSettings.Set("prometheus/endpoint", v1.NewSettingScalar("/metrics"))
		serverSettings.Set("prometheus/port", v1.NewSettingScalar("9363"))
		serverSettings.Set("prometheus/metrics", v1.NewSettingScalar("true"))
		serverSettings.Set("prometheus/events", v1.NewSettingScalar("true"))
		serverSettings.Set("prometheus/asynchronous_metrics", v1.NewSettingScalar("true"))
		serverSettings.Set("prometheus/status_info", v1.NewSettingScalar("true"))
	}

	reclaimPolicy := v1.PVCReclaimPolicyUnspecified
	if wandb.GetRetentionPolicy(spec.ManagedInfraSpec).OnDelete == apiv2.PurgeOnDelete {
		reclaimPolicy = v1.PVCReclaimPolicyDelete
	}

	podSpec := corev1.PodSpec{
		SecurityContext: clickHousePodSecurityContext(),
		Affinity:        wandb.GetAffinity(spec.ManagedInfraSpec),
		Tolerations:     *wandb.GetTolerations(spec.ManagedInfraSpec),
		Volumes:         clickHouseWritableVolumes(),
		Containers: []corev1.Container{
			{
				Name:            "clickhouse",
				Image:           ClickHouseImage(mfst.Clickhouse["default"].Images["server"], wandb.Spec.Global.ImageRegistry),
				SecurityContext: clickHouseContainerSecurityContext(),
				VolumeMounts:    clickHouseWritableVolumeMounts(),
			},
		},
	}

	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
		podSpec.Containers[0].Resources = corev1.ResourceRequirements{
			Requests: spec.Config.Resources.Requests,
			Limits:   spec.Config.Resources.Limits,
		}
	}

	chi := &v1.ClickHouseInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.InstallationName(),
			Namespace: nsnBuilder.Namespace(),
			Labels: map[string]string{
				"app": CHIName,
			},
		},
		Spec: v1.ChiSpec{
			Configuration: &v1.Configuration{
				Clusters: []*v1.Cluster{
					{
						Name: "default",
						Layout: &v1.ChiClusterLayout{
							ShardsCount:   ShardsCount,
							ReplicasCount: int(spec.Replicas),
						},
					},
				},
				Users:    userSettings,
				Settings: serverSettings,
				Zookeeper: &v1.ZookeeperConfig{
					Nodes: v1.ZookeeperNodes{
						{
							Host: keeper.ClientServiceFQDN(spec.Namespace, spec.Name),
							Port: chtypes.NewInt32(int32(keeper.KeeperClientPort)),
						},
					},
				},
			},
			Defaults: &v1.Defaults{
				Templates: &v1.TemplatesList{
					PodTemplate:             nsnBuilder.PodTemplateName(),
					DataVolumeClaimTemplate: nsnBuilder.VolumeTemplateName(),
				},
			},
			Templates: &v1.Templates{
				PodTemplates: []v1.PodTemplate{
					{
						Name: nsnBuilder.PodTemplateName(),
						ObjectMeta: metav1.ObjectMeta{
							Labels: utils.MergeMapsStringString(
								BuildWandbClickhouseLabels(wandb),
								common.StandardLabels(wandb, "clickhouse", common.RoleAnalyticsDB, ""),
							),
						},
						Spec: podSpec,
					},
				},
				VolumeClaimTemplates: []v1.VolumeClaimTemplate{
					{
						Name: nsnBuilder.VolumeTemplateName(),
						ObjectMeta: metav1.ObjectMeta{
							Labels: BuildWandbClickhouseLabels(wandb),
						},
						StorageManagement: v1.StorageManagement{
							PVCReclaimPolicy: reclaimPolicy,
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: storageQuantity,
								},
							},
						},
					},
				},
			},
		},
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(wandb, chi, scheme); err != nil {
		log.Error("failed to set owner reference on CHI CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return chi, nil
}

func BuildWandbClickhouseLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return common.BuildWandbLabels(wandb, ClickhouseModuleName)
}

func ToClickHouseOnDeleteRule(wandb *apiv2.WeightsAndBiases, retentionPolicy apiv2.RetentionPolicy) common.OnDeleteRule {
	return common.ToOnDeleteRule(wandb, retentionPolicy, ClickhouseModuleName)
}
