package seaweedfs

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("SeaweedFS vendor specs", func() {
	It("renders writable runtime mounts for SeaweedFS components", func() {
		wandb := seaweedWandb()

		seaweed, err := ToObjectStoreVendorSpec(context.Background(), wandb, seaweedScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(seaweed).NotTo(BeNil())

		Expect(seaweed.Name).To(Equal(SeaweedName("object-store")))
		Expect(seaweed.Namespace).To(Equal("wandb"))
		Expect(seaweed.Labels).To(HaveKeyWithValue("app", SeaweedName("object-store")))
		Expect(seaweed.Spec.Image).To(Equal(SeaweedImage(manifest.ImageRef{})))

		expectSeaweedWritableVolume(seaweed.Spec.Master.Volumes)
		expectSeaweedWritableMount(seaweed.Spec.Master.VolumeMounts)
		expectSeaweedWritableVolume(seaweed.Spec.Volume.Volumes)
		expectSeaweedWritableMount(seaweed.Spec.Volume.VolumeMounts)
		expectSeaweedWritableVolume(seaweed.Spec.Filer.Volumes)
		expectSeaweedWritableMount(seaweed.Spec.Filer.VolumeMounts)
	})

	It("retargets the image to spec.global.imageRegistry when set", func() {
		wandb := seaweedWandb()
		wandb.Spec.Global.ImageRegistry = "reg.corp:5000"

		seaweed, err := ToObjectStoreVendorSpec(context.Background(), wandb, seaweedScheme())
		Expect(err).NotTo(HaveOccurred())
		Expect(seaweed).NotTo(BeNil())
		Expect(seaweed.Spec.Image).To(Equal("reg.corp:5000/chrislusf/seaweedfs:latest"))
	})

	It("keeps the filer writable data path explicit", func() {
		seaweed, err := ToObjectStoreVendorSpec(context.Background(), seaweedWandb(), seaweedScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(seaweed).NotTo(BeNil())
		Expect(seaweed.Spec.Filer.Config).NotTo(BeNil())
		Expect(*seaweed.Spec.Filer.Config).To(ContainSubstring(`dir = "` + seaweedFilerDataMountPath + `"`))
		Expect(seaweed.Spec.Filer.Persistence).NotTo(BeNil())
		Expect(seaweed.Spec.Filer.Persistence.MountPath).NotTo(BeNil())
		Expect(*seaweed.Spec.Filer.Persistence.MountPath).To(Equal(seaweedFilerDataMountPath))
	})

	It("preserves managed resource overrides", func() {
		seaweed, err := ToObjectStoreVendorSpec(context.Background(), seaweedWandb(), seaweedScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(seaweed).NotTo(BeNil())
		Expect(seaweed.Spec.Volume.ResourceRequirements.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
	})

	It("sets metrics ports on master, volume, and filer", func() {
		seaweed, err := ToObjectStoreVendorSpec(context.Background(), seaweedWandb(), seaweedScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(seaweed).NotTo(BeNil())

		Expect(seaweed.Spec.Master.MetricsPort).NotTo(BeNil())
		Expect(*seaweed.Spec.Master.MetricsPort).To(Equal(seaweedMasterMetricsPort))

		Expect(seaweed.Spec.Volume.MetricsPort).NotTo(BeNil())
		Expect(*seaweed.Spec.Volume.MetricsPort).To(Equal(seaweedVolumeMetricsPort))

		Expect(seaweed.Spec.Filer.MetricsPort).NotTo(BeNil())
		Expect(*seaweed.Spec.Filer.MetricsPort).To(Equal(seaweedFilerMetricsPort))
	})
})

func seaweedScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(apiv2.AddToScheme(scheme)).To(Succeed())
	Expect(seaweedv1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func seaweedWandb() *apiv2.WeightsAndBiases {
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
					Name:        "object-store",
					Namespace:   "wandb",
					Replicas:    1,
					StorageSize: "10Gi",
					Config: apiv2.ObjectStoreConfig{
						AccessKey: "admin",
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

func expectSeaweedWritableMount(mounts []corev1.VolumeMount) {
	found := false
	for _, mount := range mounts {
		if mount.Name == seaweedWritableTmpVolumeName && mount.MountPath == seaweedWritableTmpMountPath {
			found = true
			break
		}
	}
	Expect(found).To(BeTrue())
}

func expectSeaweedWritableVolume(volumes []corev1.Volume) {
	found := false
	for _, volume := range volumes {
		if volume.Name == seaweedWritableTmpVolumeName && volume.EmptyDir != nil {
			found = true
			break
		}
	}
	Expect(found).To(BeTrue())
}
