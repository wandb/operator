package altinity

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("ClickHouse vendor specs", func() {
	BeforeEach(func() {
		utils.SetOpenShiftMode(false)
	})

	It("renders hardened pod templates with writable runtime mounts", func() {
		wandb := clickHouseWandb()

		chi, err := ToClickHouseVendorSpec(context.Background(), wandb, clickHouseScheme())
		Expect(err).NotTo(HaveOccurred())
		Expect(chi).NotTo(BeNil())
		Expect(chi.Spec.Templates.PodTemplates).To(HaveLen(1))

		podSpec := chi.Spec.Templates.PodTemplates[0].Spec
		expectClickHouseDefaultPodSecurityContext(podSpec.SecurityContext)
		expectClickHouseWritableVolume(podSpec.Volumes, clickHouseTmpVolumeName)
		expectClickHouseWritableVolume(podSpec.Volumes, clickHouseLogVolumeName)
		expectClickHouseWritableVolume(podSpec.Volumes, clickHouseRunVolumeName)

		Expect(podSpec.Containers).To(HaveLen(1))
		container := podSpec.Containers[0]
		Expect(container.Image).To(Equal(ClickHouseImage))
		Expect(container.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
		expectClickHouseDefaultContainerSecurityContext(container.SecurityContext)
		expectClickHouseWritableMount(container.VolumeMounts, clickHouseTmpVolumeName, clickHouseTmpMountPath)
		expectClickHouseWritableMount(container.VolumeMounts, clickHouseLogVolumeName, clickHouseLogMountPath)
		expectClickHouseWritableMount(container.VolumeMounts, clickHouseRunVolumeName, clickHouseRunMountPath)
	})

	It("omits fixed ClickHouse IDs in OpenShift mode", func() {
		utils.SetOpenShiftMode(true)

		chi, err := ToClickHouseVendorSpec(context.Background(), clickHouseWandb(), clickHouseScheme())
		Expect(err).NotTo(HaveOccurred())
		Expect(chi).NotTo(BeNil())

		podSpec := chi.Spec.Templates.PodTemplates[0].Spec
		expectClickHouseOpenShiftPodSecurityContext(podSpec.SecurityContext)
		expectClickHouseOpenShiftContainerSecurityContext(podSpec.Containers[0].SecurityContext)
	})
})

func clickHouseScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(apiv2.AddToScheme(scheme)).To(Succeed())
	Expect(chiv1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func clickHouseWandb() *apiv2.WeightsAndBiases {
	tolerations := []corev1.Toleration{}
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
			Tolerations: &tolerations,
			ClickHouse: apiv2.ClickHouseSpec{
				ManagedClickHouse: &apiv2.ManagedClickHouseSpec{
					Name:        "clickhouse",
					Namespace:   "wandb",
					Replicas:    1,
					StorageSize: "10Gi",
					Config: apiv2.ClickHouseConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("500m"),
							},
						},
					},
				},
			},
		},
	}
}

func expectClickHouseDefaultPodSecurityContext(securityContext *corev1.PodSecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).NotTo(BeNil())
	Expect(*securityContext.RunAsUser).To(Equal(clickHouseRunAsUser))
	Expect(securityContext.RunAsGroup).NotTo(BeNil())
	Expect(*securityContext.RunAsGroup).To(Equal(clickHouseRunAsGroup))
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.FSGroup).NotTo(BeNil())
	Expect(*securityContext.FSGroup).To(Equal(clickHouseFSGroup))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectClickHouseDefaultContainerSecurityContext(securityContext *corev1.SecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).NotTo(BeNil())
	Expect(*securityContext.RunAsUser).To(Equal(clickHouseRunAsUser))
	Expect(securityContext.RunAsGroup).NotTo(BeNil())
	Expect(*securityContext.RunAsGroup).To(Equal(clickHouseRunAsGroup))
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.AllowPrivilegeEscalation).NotTo(BeNil())
	Expect(*securityContext.AllowPrivilegeEscalation).To(BeFalse())
	Expect(securityContext.Capabilities).NotTo(BeNil())
	Expect(securityContext.Capabilities.Drop).To(ContainElement(clickHouseCapabilityAll))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectClickHouseWritableMount(mounts []corev1.VolumeMount, name, mountPath string) {
	found := false
	for _, mount := range mounts {
		if mount.Name == name && mount.MountPath == mountPath {
			found = true
			break
		}
	}
	Expect(found).To(BeTrue())
}

func expectClickHouseOpenShiftPodSecurityContext(securityContext *corev1.PodSecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).To(BeNil())
	Expect(securityContext.RunAsGroup).To(BeNil())
	Expect(securityContext.FSGroup).To(BeNil())
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectClickHouseOpenShiftContainerSecurityContext(securityContext *corev1.SecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).To(BeNil())
	Expect(securityContext.RunAsGroup).To(BeNil())
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.AllowPrivilegeEscalation).NotTo(BeNil())
	Expect(*securityContext.AllowPrivilegeEscalation).To(BeFalse())
	Expect(securityContext.Capabilities).NotTo(BeNil())
	Expect(securityContext.Capabilities.Drop).To(ContainElement(clickHouseCapabilityAll))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectClickHouseWritableVolume(volumes []corev1.Volume, name string) {
	found := false
	for _, volume := range volumes {
		if volume.Name == name && volume.EmptyDir != nil {
			found = true
			break
		}
	}
	Expect(found).To(BeTrue())
}
