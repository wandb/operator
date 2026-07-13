package reconciler_test

import (
	"context"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	v2 "github.com/wandb/operator/internal/controller/reconciler"
	"github.com/wandb/operator/internal/controller/infra/managed/objectstore/seaweedfs"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// localManifestVersion is the checked-in server manifest used for local dev; the
// sizing assertions below mirror its bucket.default.sizing block.
const localManifestVersion = "0.83.0-clickhouse-keeper.2"

func objectStoreScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(apiv2.AddToScheme(scheme)).To(Succeed())
	Expect(seaweedv1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func objectStoreWandb(size apiv2.Size) *apiv2.WeightsAndBiases {
	tolerations := []corev1.Toleration{}
	return &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb"},
		Spec: apiv2.WeightsAndBiasesSpec{
			Size:        size,
			Tolerations: &tolerations,
			ObjectStore: map[string]apiv2.ObjectStoreSpec{
				apiv2.DefaultInstanceName: {
					ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{
						Name:      "object-store",
						Namespace: "wandb",
						Config:    apiv2.ObjectStoreConfig{AccessKey: "admin"},
					},
				},
			},
		},
	}
}

var _ = Describe("ObjectStore sizing per tier", func() {
	var mfst serverManifest.Manifest

	BeforeEach(func() {
		repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
		Expect(err).NotTo(HaveOccurred())
		repository := "file://" + filepath.Join(repoRoot, "hack", "testing-manifests", "server-manifest")
		mfst, err = serverManifest.GetServerManifest(context.Background(), repository, localManifestVersion)
		Expect(err).NotTo(HaveOccurred())
		Expect(mfst.Bucket).To(HaveKey("default"))
	})

	// Values mirror bucket.default.sizing in the local manifest. Every tier must
	// keep the same hardening regardless of size: 1024MB rollover, auto-fill
	// disk (maxVolumeCounts 0), and a small fixed filer disk.
	DescribeTable("renders a healthy Seaweed spec for each size",
		func(size apiv2.Size, wantReplicas int32, wantVolumeSize, wantReplication string, wantCPU string) {
			wandb := objectStoreWandb(size)
			v2.ApplyInfraSizing(wandb, mfst)

			seaweed, err := seaweedfs.ToObjectStoreVendorSpec(context.Background(), wandb, wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore, objectStoreScheme(), mfst)
			Expect(err).NotTo(HaveOccurred())
			Expect(seaweed).NotTo(BeNil())

			Expect(seaweed.Spec.Volume.Replicas).To(Equal(wantReplicas))
			Expect(seaweed.Spec.Volume.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse(wantVolumeSize)))
			Expect(seaweed.Spec.Master.DefaultReplication).NotTo(BeNil())
			Expect(*seaweed.Spec.Master.DefaultReplication).To(Equal(wantReplication))

			// Hardening that must hold for every tier.
			Expect(*seaweed.Spec.Master.VolumeSizeLimitMB).To(Equal(int32(1024)))
			Expect(seaweed.Spec.Volume.MaxVolumeCounts).NotTo(BeNil())
			Expect(*seaweed.Spec.Volume.MaxVolumeCounts).To(Equal(int32(0)))
			Expect(seaweed.Spec.Filer.Persistence.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("1Gi")))

			if wantCPU == "" {
				Expect(seaweed.Spec.Volume.Requests).NotTo(HaveKey(corev1.ResourceCPU))
			} else {
				Expect(seaweed.Spec.Volume.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(wantCPU)))
			}
		},
		Entry("dev", apiv2.Size("dev"), int32(1), "10Gi", "000", ""),
		Entry("micro", apiv2.Size("micro"), int32(3), "50Gi", "001", "1"),
		Entry("small", apiv2.Size("small"), int32(3), "100Gi", "001", "2"),
		Entry("medium", apiv2.Size("medium"), int32(3), "100Gi", "001", "4"),
		Entry("large", apiv2.Size("large"), int32(3), "200Gi", "001", "8"),
		Entry("xlarge", apiv2.Size("xlarge"), int32(3), "200Gi", "001", "8"),
		Entry("2xlarge", apiv2.Size("2xlarge"), int32(3), "200Gi", "001", "8"),
	)
})
