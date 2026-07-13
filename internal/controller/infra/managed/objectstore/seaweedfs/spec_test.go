package seaweedfs

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("SeaweedFS vendor specs", func() {
	It("renders writable runtime mounts for SeaweedFS components", func() {
		wandb := seaweedWandb()

		seaweed, err := ToObjectStoreVendorSpec(context.Background(), wandb, wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore, seaweedScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(seaweed).NotTo(BeNil())

		Expect(seaweed.Name).To(Equal(SeaweedName("object-store")))
		Expect(seaweed.Namespace).To(Equal("wandb"))
		Expect(seaweed.Labels).To(HaveKeyWithValue("app", SeaweedName("object-store")))
		Expect(seaweed.Spec.Image).To(Equal(SeaweedImage(manifest.ImageRef{}, "")))

		expectSeaweedWritableVolume(seaweed.Spec.Master.Volumes)
		expectSeaweedWritableMount(seaweed.Spec.Master.VolumeMounts)
		expectSeaweedWritableVolume(seaweed.Spec.Volume.Volumes)
		expectSeaweedWritableMount(seaweed.Spec.Volume.VolumeMounts)
		expectSeaweedWritableVolume(seaweed.Spec.Filer.Volumes)
		expectSeaweedWritableMount(seaweed.Spec.Filer.VolumeMounts)
		Expect(seaweed.Spec.Master.ExtraArgs).To(ContainElement("-ip.bind=0.0.0.0"))
		Expect(seaweed.Spec.Volume.ExtraArgs).To(ContainElement("-ip.bind=0.0.0.0"))
		Expect(seaweed.Spec.Filer.ExtraArgs).To(ContainElement("-ip.bind=0.0.0.0"))
	})

	It("retargets the image to spec.global.imageRegistry when set", func() {
		wandb := seaweedWandb()
		wandb.Spec.Global.ImageRegistry = "reg.corp:5000"

		mfst := manifest.Manifest{
			Bucket: map[string]manifest.InfraConfig{
				"default": {
					Images: map[string]manifest.ImageRef{
						"seaweedfs": {Repository: "chrislusf/seaweedfs", Tag: "latest"},
					},
				},
			},
		}

		seaweed, err := ToObjectStoreVendorSpec(context.Background(), wandb, wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore, seaweedScheme(), mfst)
		Expect(err).NotTo(HaveOccurred())
		Expect(seaweed).NotTo(BeNil())
		Expect(seaweed.Spec.Image).To(Equal("reg.corp:5000/chrislusf/seaweedfs:latest"))
	})

	It("keeps the filer writable data path explicit", func() {
		wandb := seaweedWandb()
		seaweed, err := ToObjectStoreVendorSpec(context.Background(), wandb, wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore, seaweedScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(seaweed).NotTo(BeNil())
		Expect(seaweed.Spec.Filer.Config).NotTo(BeNil())
		Expect(*seaweed.Spec.Filer.Config).To(ContainSubstring(`dir = "` + seaweedFilerDataMountPath + `"`))
		Expect(seaweed.Spec.Filer.Persistence).NotTo(BeNil())
		Expect(seaweed.Spec.Filer.Persistence.MountPath).NotTo(BeNil())
		Expect(*seaweed.Spec.Filer.Persistence.MountPath).To(Equal(seaweedFilerDataMountPath))
	})

	It("preserves managed resource overrides", func() {
		wandb := seaweedWandb()
		seaweed, err := ToObjectStoreVendorSpec(context.Background(), wandb, wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore, seaweedScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(seaweed).NotTo(BeNil())
		Expect(seaweed.Spec.Volume.ResourceRequirements.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
	})

	It("reserves storage headroom for writable volumes", func() {
		wandb := seaweedWandb()
		seaweed, err := ToObjectStoreVendorSpec(context.Background(), wandb, wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore, seaweedScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())

		Expect(seaweed.Spec.Master.VolumeSizeLimitMB).NotTo(BeNil())
		Expect(*seaweed.Spec.Master.VolumeSizeLimitMB).To(Equal(int32(1024)))
		Expect(seaweed.Spec.Volume.MaxVolumeCounts).NotTo(BeNil())
		Expect(*seaweed.Spec.Volume.MaxVolumeCounts).To(Equal(int32(9)))
	})

	DescribeTable("computes a writable volume layout",
		func(storage string, expectedSizeMB, expectedMaxVolumes int32) {
			size, count := volumeLayout(resource.MustParse(storage))
			Expect(size).To(Equal(expectedSizeMB))
			Expect(count).To(Equal(expectedMaxVolumes))
		},
		Entry("a development volume", "10Gi", int32(1024), int32(9)),
		Entry("the upstream minimum example", "2Gi", int32(1024), int32(1)),
		Entry("a sub-gibibyte volume", "512Mi", int32(256), int32(1)),
		Entry("a large volume", "1Ti", int32(1024), int32(1023)),
	)

	It("pins s3 gateway signature verification to the in-cluster endpoint", func() {
		wandb := seaweedWandb()
		seaweed, err := ToObjectStoreVendorSpec(context.Background(), wandb, wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore, seaweedScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(seaweed).NotTo(BeNil())
		Expect(seaweed.Spec.S3.Env).To(ContainElement(corev1.EnvVar{
			Name:  "S3_EXTERNAL_URL",
			Value: "http://" + SeaweedName("object-store") + "-s3.wandb.svc.cluster.local:" + S3Port,
		}))
	})

	It("uses https for the s3 external URL when TLS is enabled", func() {
		wandb := seaweedWandb()
		wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore.SeaweedObjectStoreSpec.TlsEnabled = true

		seaweed, err := ToObjectStoreVendorSpec(context.Background(), wandb, wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore, seaweedScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(seaweed).NotTo(BeNil())
		Expect(seaweed.Spec.S3.Env).To(ContainElement(corev1.EnvVar{
			Name:  "S3_EXTERNAL_URL",
			Value: "https://" + SeaweedName("object-store") + "-s3.wandb.svc.cluster.local:" + S3Port,
		}))
	})

	It("sets metrics ports on master, volume, and filer", func() {
		wandb := seaweedWandb()
		seaweed, err := ToObjectStoreVendorSpec(context.Background(), wandb, wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore, seaweedScheme(), manifest.Manifest{})
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
			ObjectStore: map[string]apiv2.ObjectStoreSpec{
				apiv2.DefaultInstanceName: {
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
