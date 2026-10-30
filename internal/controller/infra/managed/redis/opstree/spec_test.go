package opstree

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	redisv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redissentinel/v1beta2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Redis vendor specs", func() {
	BeforeEach(func() {
		utils.SetOpenShiftMode(false)
	})

	It("renders hardened standalone Redis settings", func() {
		wandb := redisWandb(false)

		redis, err := ToRedisStandaloneVendorSpec(context.Background(), wandb, wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis, redisScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(redis).NotTo(BeNil())

		expectRedisDefaultPodSecurityContext(redis.Spec.PodSecurityContext)
		expectRedisDefaultContainerSecurityContext(redis.Spec.SecurityContext)
		Expect(redis.Spec.InitContainer).NotTo(BeNil())
		Expect(redis.Spec.InitContainer.Enabled).To(HaveValue(BeFalse()))
		expectRedisDefaultContainerSecurityContext(redis.Spec.InitContainer.SecurityContext)
		expectRedisWritableTmpMount(redis.Spec.Storage.VolumeMount.MountPath)
		Expect(redis.Spec.RedisExporter).NotTo(BeNil())
		expectRedisDefaultContainerSecurityContext(redis.Spec.RedisExporter.SecurityContext)
	})

	It("renders hardened sentinel and replication Redis settings", func() {
		wandb := redisWandb(true)

		sentinel, err := ToRedisSentinelVendorSpec(context.Background(), wandb, wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis, redisScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(sentinel).NotTo(BeNil())
		expectRedisDefaultPodSecurityContext(sentinel.Spec.PodSecurityContext)
		expectRedisDefaultContainerSecurityContext(sentinel.Spec.SecurityContext)
		Expect(sentinel.Spec.InitContainer).NotTo(BeNil())
		Expect(sentinel.Spec.InitContainer.Enabled).To(HaveValue(BeFalse()))
		expectRedisDefaultContainerSecurityContext(sentinel.Spec.InitContainer.SecurityContext)
		Expect(sentinel.Spec.VolumeMount).NotTo(BeNil())
		expectRedisWritableTmpMount(sentinel.Spec.VolumeMount.MountPath)
		Expect(sentinel.Spec.VolumeMount.MountPath).To(ContainElement(corev1.VolumeMount{
			Name: redisSentinelRunVolumeName, MountPath: "/var/run",
		}))
		Expect(sentinel.Spec.VolumeMount.Volume).To(ContainElement(corev1.Volume{
			Name:         redisSentinelRunVolumeName,
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}))
		// Sentinels are not sized by the server manifest; they still need
		// requests so the quorum is schedulable rather than BestEffort.
		Expect(sentinel.Spec.KubernetesConfig.Resources).NotTo(BeNil())
		Expect(sentinel.Spec.KubernetesConfig.Resources.Requests.Cpu().String()).To(Equal("100m"))
		Expect(sentinel.Spec.KubernetesConfig.Resources.Requests.Memory().String()).To(Equal("128Mi"))
		Expect(sentinel.Spec.KubernetesConfig.Resources.Limits.Cpu().String()).To(Equal("250m"))
		Expect(sentinel.Spec.KubernetesConfig.Resources.Limits.Memory().String()).To(Equal("256Mi"))

		replication, err := ToRedisReplicationVendorSpec(context.Background(), wandb, wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis, redisScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(replication).NotTo(BeNil())
		expectRedisDefaultPodSecurityContext(replication.Spec.PodSecurityContext)
		expectRedisDefaultContainerSecurityContext(replication.Spec.SecurityContext)
		Expect(replication.Spec.InitContainer).NotTo(BeNil())
		Expect(replication.Spec.InitContainer.Enabled).To(HaveValue(BeFalse()))
		expectRedisDefaultContainerSecurityContext(replication.Spec.InitContainer.SecurityContext)
		expectRedisWritableTmpMount(replication.Spec.Storage.VolumeMount.MountPath)
	})

	It("lets an explicit CR sentinel resource block replace the default", func() {
		wandb := redisWandb(true)
		spec := wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis
		spec.Sentinel.Config.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("300m"),
				corev1.ResourceMemory: resource.MustParse("300Mi"),
			},
		}

		sentinel, err := ToRedisSentinelVendorSpec(context.Background(), wandb, spec, redisScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(sentinel.Spec.KubernetesConfig.Resources.Requests.Cpu().String()).To(Equal("300m"))
		Expect(sentinel.Spec.KubernetesConfig.Resources.Requests.Memory().String()).To(Equal("300Mi"))
		Expect(sentinel.Spec.KubernetesConfig.Resources.Limits).To(BeEmpty())
	})

	It("omits fixed Redis IDs in OpenShift mode", func() {
		utils.SetOpenShiftMode(true)

		wandb := redisWandb(false)
		redis, err := ToRedisStandaloneVendorSpec(context.Background(), wandb, wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis, redisScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(redis).NotTo(BeNil())

		expectOpenShiftPodSecurityContext(redis.Spec.PodSecurityContext)
		expectOpenShiftContainerSecurityContext(redis.Spec.SecurityContext)
		expectOpenShiftContainerSecurityContext(redis.Spec.InitContainer.SecurityContext)
		expectRedisWritableTmpMount(redis.Spec.Storage.VolumeMount.MountPath)

		wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis.Sentinel.Enabled = true
		sentinel, err := ToRedisSentinelVendorSpec(context.Background(), wandb, wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis, redisScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		expectOpenShiftPodSecurityContext(sentinel.Spec.PodSecurityContext)
		expectOpenShiftContainerSecurityContext(sentinel.Spec.SecurityContext)
		Expect(sentinel.Spec.VolumeMount.MountPath).To(ContainElement(corev1.VolumeMount{
			Name: redisSentinelRunVolumeName, MountPath: "/var/run",
		}))
	})

	It("retains ownership labels on actual standalone and replication PVCs", func() {
		wandb := redisWandb(true)
		labels := BuildWandbRedisLabels(wandb)
		standalone := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "redis-redis-0", Namespace: "wandb", Labels: map[string]string{"existing": "keep"}}}
		replication := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "custom-data-redis-replica-0", Namespace: "wandb"}}
		retained := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "custom-data-redis-replica-3", Namespace: "wandb"}}
		legacy := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "redis-data-legacy-0", Namespace: "wandb"}}
		unrelated := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "redis-redis-other-0", Namespace: "wandb"}}
		statefulSet := func(name, template string) *appsv1.StatefulSet {
			return &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "wandb"}, Spec: appsv1.StatefulSetSpec{
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: template}}},
			}}
		}
		cl := fake.NewClientBuilder().WithScheme(redisScheme()).WithObjects(standalone, replication, retained, legacy, unrelated,
			statefulSet("redis", "redis"), statefulSet("redis-replica", "custom-data"), statefulSet("legacy", "redis-data")).Build()
		Expect(ensurePVCLabels(context.Background(), cl, "wandb", []string{"redis", "redis-replica", "legacy", "not-created-yet"}, labels)).To(Succeed())
		for _, pvc := range []*corev1.PersistentVolumeClaim{standalone, replication, retained, legacy} {
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(pvc), pvc)).To(Succeed())
			for key, value := range labels {
				Expect(pvc.Labels).To(HaveKeyWithValue(key, value))
			}
		}
		Expect(standalone.Labels).To(HaveKeyWithValue("existing", "keep"))
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(unrelated), unrelated)).To(Succeed())
		Expect(unrelated.Labels).To(BeEmpty())
	})
})

func redisScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(appsv1.AddToScheme(scheme)).To(Succeed())
	Expect(apiv2.AddToScheme(scheme)).To(Succeed())
	Expect(redisv1beta2.AddToScheme(scheme)).To(Succeed())
	Expect(redissentinelv1beta2.AddToScheme(scheme)).To(Succeed())
	Expect(redisreplicationv1beta2.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func redisWandb(sentinel bool) *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiv2.GroupVersion.String(),
			Kind:       "WeightsAndBiases",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "wandb",
		},
		Spec: apiv2.WeightsAndBiasesSpec{
			Redis: map[string]apiv2.RedisSpec{
				apiv2.DefaultInstanceName: {
					ManagedRedis: &apiv2.ManagedRedisSpec{
						Name:        "redis",
						Namespace:   "wandb",
						StorageSize: "1Gi",
						Telemetry:   apiv2.Telemetry{Enabled: true},
						Sentinel:    apiv2.RedisSentinelSpec{Enabled: sentinel},
					},
				},
			},
		},
	}
}

func expectRedisDefaultPodSecurityContext(securityContext *corev1.PodSecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).NotTo(BeNil())
	Expect(*securityContext.RunAsUser).To(Equal(redisRunAsUser))
	Expect(securityContext.RunAsGroup).NotTo(BeNil())
	Expect(*securityContext.RunAsGroup).To(Equal(redisRunAsGroup))
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.FSGroup).NotTo(BeNil())
	Expect(*securityContext.FSGroup).To(Equal(redisFSGroup))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectRedisDefaultContainerSecurityContext(securityContext *corev1.SecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).NotTo(BeNil())
	Expect(*securityContext.RunAsUser).To(Equal(redisRunAsUser))
	Expect(securityContext.RunAsGroup).NotTo(BeNil())
	Expect(*securityContext.RunAsGroup).To(Equal(redisRunAsGroup))
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.AllowPrivilegeEscalation).NotTo(BeNil())
	Expect(*securityContext.AllowPrivilegeEscalation).To(BeFalse())
	Expect(securityContext.ReadOnlyRootFilesystem).To(HaveValue(BeTrue()))
	Expect(securityContext.Capabilities).NotTo(BeNil())
	Expect(securityContext.Capabilities.Drop).To(ContainElement(redisCapabilityAll))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectRedisWritableTmpMount(mounts []corev1.VolumeMount) {
	found := false
	for _, mount := range mounts {
		if mount.Name == redisWritableTmpVolumeName && mount.MountPath == redisWritableTmpMountPath {
			found = true
			break
		}
	}
	Expect(found).To(BeTrue())
}

func expectOpenShiftPodSecurityContext(securityContext *corev1.PodSecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).To(BeNil())
	Expect(securityContext.RunAsGroup).To(BeNil())
	Expect(securityContext.FSGroup).To(BeNil())
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectOpenShiftContainerSecurityContext(securityContext *corev1.SecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).To(BeNil())
	Expect(securityContext.RunAsGroup).To(BeNil())
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.AllowPrivilegeEscalation).NotTo(BeNil())
	Expect(*securityContext.AllowPrivilegeEscalation).To(BeFalse())
	Expect(securityContext.ReadOnlyRootFilesystem).To(HaveValue(BeTrue()))
	Expect(securityContext.Capabilities).NotTo(BeNil())
	Expect(securityContext.Capabilities.Drop).To(ContainElement(redisCapabilityAll))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}
