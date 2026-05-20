package tenant

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	miniov2 "github.com/wandb/operator/pkg/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("MinIO tenant specs", func() {
	BeforeEach(func() {
		utils.SetOpenShiftMode(false)
	})

	It("renders hardened pool settings and writable tmp mount", func() {
		wandb := minioWandb()

		tenant, err := ToObjectStoreVendorSpec(context.Background(), wandb, minioScheme())
		Expect(err).NotTo(HaveOccurred())
		Expect(tenant).NotTo(BeNil())
		Expect(tenant.Spec.Mountpath).To(Equal(miniov2.MinIOVolumeMountPath))
		Expect(tenant.Spec.Subpath).To(Equal(miniov2.MinIOVolumeSubPath))

		Expect(tenant.Spec.Pools).To(HaveLen(1))
		pool := tenant.Spec.Pools[0]
		expectMinioDefaultPodSecurityContext(pool.SecurityContext)
		expectMinioDefaultContainerSecurityContext(pool.ContainerSecurityContext)
		expectMinioWritableVolume(tenant.Spec.AdditionalVolumes, minioWritableTmpVolumeName)
		expectMinioWritableMount(tenant.Spec.AdditionalVolumeMounts, minioWritableTmpVolumeName, minioWritableTmpMountPath)
	})

	It("omits fixed MinIO IDs in OpenShift mode", func() {
		utils.SetOpenShiftMode(true)

		tenant, err := ToObjectStoreVendorSpec(context.Background(), minioWandb(), minioScheme())
		Expect(err).NotTo(HaveOccurred())
		Expect(tenant).NotTo(BeNil())

		pool := tenant.Spec.Pools[0]
		expectMinioOpenShiftPodSecurityContext(pool.SecurityContext)
		expectMinioOpenShiftContainerSecurityContext(pool.ContainerSecurityContext)
	})
})

func minioScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(apiv2.AddToScheme(scheme)).To(Succeed())
	Expect(miniov2.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func minioWandb() *apiv2.WeightsAndBiases {
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
			ObjectStore: apiv2.ObjectStoreSpec{
				ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{
					Name:        "minio",
					Namespace:   "wandb",
					Replicas:    1,
					StorageSize: "10Gi",
				},
			},
		},
	}
}

func expectMinioDefaultPodSecurityContext(securityContext *corev1.PodSecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).NotTo(BeNil())
	Expect(*securityContext.RunAsUser).To(Equal(minioRunAsUser))
	Expect(securityContext.RunAsGroup).NotTo(BeNil())
	Expect(*securityContext.RunAsGroup).To(Equal(minioRunAsGroup))
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.FSGroup).NotTo(BeNil())
	Expect(*securityContext.FSGroup).To(Equal(minioFSGroup))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectMinioDefaultContainerSecurityContext(securityContext *corev1.SecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).NotTo(BeNil())
	Expect(*securityContext.RunAsUser).To(Equal(minioRunAsUser))
	Expect(securityContext.RunAsGroup).NotTo(BeNil())
	Expect(*securityContext.RunAsGroup).To(Equal(minioRunAsGroup))
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.AllowPrivilegeEscalation).NotTo(BeNil())
	Expect(*securityContext.AllowPrivilegeEscalation).To(BeFalse())
	Expect(securityContext.Capabilities).NotTo(BeNil())
	Expect(securityContext.Capabilities.Drop).To(ContainElement(minioCapabilityAll))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectMinioWritableVolume(volumes []corev1.Volume, name string) {
	found := false
	for _, volume := range volumes {
		if volume.Name == name && volume.EmptyDir != nil {
			found = true
			break
		}
	}
	Expect(found).To(BeTrue())
}

func expectMinioWritableMount(mounts []corev1.VolumeMount, name, mountPath string) {
	found := false
	for _, mount := range mounts {
		if mount.Name == name && mount.MountPath == mountPath {
			found = true
			break
		}
	}
	Expect(found).To(BeTrue())
}

func expectMinioOpenShiftPodSecurityContext(securityContext *corev1.PodSecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).To(BeNil())
	Expect(securityContext.RunAsGroup).To(BeNil())
	Expect(securityContext.FSGroup).To(BeNil())
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectMinioOpenShiftContainerSecurityContext(securityContext *corev1.SecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).To(BeNil())
	Expect(securityContext.RunAsGroup).To(BeNil())
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.AllowPrivilegeEscalation).NotTo(BeNil())
	Expect(*securityContext.AllowPrivilegeEscalation).To(BeFalse())
	Expect(securityContext.Capabilities).NotTo(BeNil())
	Expect(securityContext.Capabilities.Drop).To(ContainElement(minioCapabilityAll))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}
