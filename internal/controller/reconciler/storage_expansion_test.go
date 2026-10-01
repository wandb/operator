package reconciler

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("SeaweedFS volume expansion", func() {
	var scheme *runtime.Scheme
	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(storagev1.AddToScheme(scheme)).To(Succeed())
	})

	wandb := func(storageSize string) *apiv2.WeightsAndBiases {
		return &apiv2.WeightsAndBiases{
			Spec: apiv2.WeightsAndBiasesSpec{
				ObjectStore: map[string]apiv2.ObjectStoreSpec{
					apiv2.DefaultInstanceName: {
						ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{
							Name:        "wandb-seaweedfs",
							Namespace:   "wandb",
							StorageSize: storageSize,
						},
					},
				},
			},
		}
	}
	pvc := func(request, capacity string) *corev1.PersistentVolumeClaim {
		return &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mount0-wandb-seaweedfs-volume-0",
				Namespace: "wandb",
				Labels: map[string]string{
					"app.kubernetes.io/instance":  "wandb-seaweedfs",
					"app.kubernetes.io/component": "volume",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: ptr.To("standard"),
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(request),
					},
				},
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(capacity),
				},
			},
		}
	}
	storageClass := func(expandable bool) *storagev1.StorageClass {
		return &storagev1.StorageClass{
			ObjectMeta:           metav1.ObjectMeta{Name: "standard"},
			AllowVolumeExpansion: ptr.To(expandable),
		}
	}

	It("blocks an increase when the StorageClass is not expandable", func() {
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(pvc("10Gi", "10Gi"), storageClass(false)).
			Build()

		blockers, pending, err := reconcileSeaweedFSVolumeExpansion(context.Background(), c, wandb("100Gi"))
		Expect(err).NotTo(HaveOccurred())
		Expect(blockers).To(HaveLen(1))
		Expect(pending).To(BeEmpty())
		want := `objectStore/volume requires PVC wandb/mount0-wandb-seaweedfs-volume-0 to grow from 10Gi to 100Gi, but StorageClass "standard" does not allow volume expansion`
		Expect(blockers[0].String()).To(Equal(want))
	})

	It("grows the PVC and waits for capacity", func() {
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(pvc("10Gi", "10Gi"), storageClass(true)).
			Build()

		blockers, pending, err := reconcileSeaweedFSVolumeExpansion(context.Background(), c, wandb("100Gi"))
		Expect(err).NotTo(HaveOccurred())
		Expect(blockers).To(BeEmpty())
		Expect(pending).To(HaveLen(1))

		actual := &corev1.PersistentVolumeClaim{}
		Expect(c.Get(context.Background(), client.ObjectKey{Namespace: "wandb", Name: "mount0-wandb-seaweedfs-volume-0"}, actual)).To(Succeed())
		Expect(actual.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("100Gi")))
	})

	It("skips targets without a name or namespace", func() {
		claim := pvc("10Gi", "10Gi")
		claim.Labels["app.kubernetes.io/instance"] = ""
		desired := wandb("100Gi")
		desired.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore.Name = ""
		desired.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore.Namespace = ""
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim, storageClass(true)).Build()

		blockers, pending, err := reconcileSeaweedFSVolumeExpansion(context.Background(), c, desired)

		Expect(err).NotTo(HaveOccurred())
		Expect(blockers).To(BeEmpty())
		Expect(pending).To(BeEmpty())
		actual := &corev1.PersistentVolumeClaim{}
		Expect(c.Get(context.Background(), client.ObjectKeyFromObject(claim), actual)).To(Succeed())
		Expect(actual.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("10Gi")))
	})

	It("grows the filer PVC", func() {
		filerPVC := pvc("20Gi", "20Gi")
		filerPVC.Name = "data-wandb-seaweedfs-filer-0"
		filerPVC.Labels["app.kubernetes.io/component"] = "filer"
		desired := wandb("10Gi")
		desired.Spec.ObjectStore[apiv2.DefaultInstanceName].
			ManagedObjectStore.SeaweedObjectStoreSpec.FilerStorageSize = "50Gi"
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(filerPVC, storageClass(true)).
			Build()

		blockers, pending, err := reconcileSeaweedFSVolumeExpansion(context.Background(), c, desired)
		Expect(err).NotTo(HaveOccurred())
		Expect(blockers).To(BeEmpty())
		Expect(pending).To(HaveLen(1))

		actual := &corev1.PersistentVolumeClaim{}
		Expect(c.Get(context.Background(), client.ObjectKeyFromObject(filerPVC), actual)).To(Succeed())
		Expect(actual.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("50Gi")))
	})

	It("reports convergence when request and capacity match", func() {
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc("100Gi", "100Gi")).Build()

		blockers, pending, err := reconcileSeaweedFSVolumeExpansion(context.Background(), c, wandb("100Gi"))
		Expect(err).NotTo(HaveOccurred())
		Expect(blockers).To(BeEmpty())
		Expect(pending).To(BeEmpty())
	})

	It("never shrinks a PVC", func() {
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc("100Gi", "100Gi")).Build()

		blockers, pending, err := reconcileSeaweedFSVolumeExpansion(context.Background(), c, wandb("10Gi"))
		Expect(err).NotTo(HaveOccurred())
		Expect(blockers).To(BeEmpty())
		Expect(pending).To(BeEmpty())

		actual := &corev1.PersistentVolumeClaim{}
		Expect(c.Get(context.Background(), client.ObjectKey{Namespace: "wandb", Name: "mount0-wandb-seaweedfs-volume-0"}, actual)).To(Succeed())
		Expect(actual.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("100Gi")))
	})
})
