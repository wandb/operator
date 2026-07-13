package opstree

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	"github.com/wandb/operator/pkg/utils"
	redisv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redissentinel/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
		Expect(sentinel.Spec.VolumeMount).NotTo(BeNil())
		expectRedisWritableTmpMount(sentinel.Spec.VolumeMount.MountPath)

		replication, err := ToRedisReplicationVendorSpec(context.Background(), wandb, wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis, redisScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(replication).NotTo(BeNil())
		expectRedisDefaultPodSecurityContext(replication.Spec.PodSecurityContext)
		expectRedisDefaultContainerSecurityContext(replication.Spec.SecurityContext)
		expectRedisWritableTmpMount(replication.Spec.Storage.VolumeMount.MountPath)
	})

	It("omits fixed Redis IDs in OpenShift mode", func() {
		utils.SetOpenShiftMode(true)

		wandb := redisWandb(false)
		redis, err := ToRedisStandaloneVendorSpec(context.Background(), wandb, wandb.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis, redisScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(redis).NotTo(BeNil())

		expectOpenShiftPodSecurityContext(redis.Spec.PodSecurityContext)
		expectOpenShiftContainerSecurityContext(redis.Spec.SecurityContext)
		expectRedisWritableTmpMount(redis.Spec.Storage.VolumeMount.MountPath)
	})
})

func redisScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
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
	Expect(securityContext.Capabilities).NotTo(BeNil())
	Expect(securityContext.Capabilities.Drop).To(ContainElement(redisCapabilityAll))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}
