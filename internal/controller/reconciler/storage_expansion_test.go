package reconciler

import (
	"context"
	"testing"

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

func TestReconcileSeaweedFSVolumeExpansion(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

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

	t.Run("blocks an increase when the StorageClass is not expandable", func(t *testing.T) {
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(pvc("10Gi", "10Gi"), storageClass(false)).
			Build()

		blockers, pending, err := reconcileSeaweedFSVolumeExpansion(context.Background(), c, wandb("100Gi"))
		if err != nil {
			t.Fatal(err)
		}
		if len(blockers) != 1 {
			t.Fatalf("got %d blockers, want 1", len(blockers))
		}
		if len(pending) != 0 {
			t.Fatalf("got pending expansions %v, want none", pending)
		}
		want := `objectStore/volume requires PVC wandb/mount0-wandb-seaweedfs-volume-0 to grow from 10Gi to 100Gi, but StorageClass "standard" does not allow volume expansion`
		if got := blockers[0].String(); got != want {
			t.Fatalf("blocker = %q, want %q", got, want)
		}
	})

	t.Run("grows the PVC and waits for capacity", func(t *testing.T) {
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(pvc("10Gi", "10Gi"), storageClass(true)).
			Build()

		blockers, pending, err := reconcileSeaweedFSVolumeExpansion(context.Background(), c, wandb("100Gi"))
		if err != nil {
			t.Fatal(err)
		}
		if len(blockers) != 0 {
			t.Fatalf("got blockers %v, want none", blockers)
		}
		if len(pending) != 1 {
			t.Fatalf("got %d pending expansions, want 1", len(pending))
		}

		actual := &corev1.PersistentVolumeClaim{}
		if err := c.Get(context.Background(), client.ObjectKey{Namespace: "wandb", Name: "mount0-wandb-seaweedfs-volume-0"}, actual); err != nil {
			t.Fatal(err)
		}
		if got := actual.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse("100Gi")) != 0 {
			t.Fatalf("PVC request = %s, want 100Gi", got.String())
		}
	})

	t.Run("grows the filer PVC", func(t *testing.T) {
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
		if err != nil {
			t.Fatal(err)
		}
		if len(blockers) != 0 || len(pending) != 1 {
			t.Fatalf("got blockers %v and pending %v, want one pending expansion", blockers, pending)
		}

		actual := &corev1.PersistentVolumeClaim{}
		if err := c.Get(context.Background(), client.ObjectKeyFromObject(filerPVC), actual); err != nil {
			t.Fatal(err)
		}
		if got := actual.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse("50Gi")) != 0 {
			t.Fatalf("PVC request = %s, want 50Gi", got.String())
		}
	})

	t.Run("reports convergence when request and capacity match", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc("100Gi", "100Gi")).Build()

		blockers, pending, err := reconcileSeaweedFSVolumeExpansion(context.Background(), c, wandb("100Gi"))
		if err != nil {
			t.Fatal(err)
		}
		if len(blockers) != 0 || len(pending) != 0 {
			t.Fatalf("got blockers %v and pending %v, want neither", blockers, pending)
		}
	})

	t.Run("never shrinks a PVC", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc("100Gi", "100Gi")).Build()

		blockers, pending, err := reconcileSeaweedFSVolumeExpansion(context.Background(), c, wandb("10Gi"))
		if err != nil {
			t.Fatal(err)
		}
		if len(blockers) != 0 || len(pending) != 0 {
			t.Fatalf("got blockers %v and pending %v, want neither", blockers, pending)
		}

		actual := &corev1.PersistentVolumeClaim{}
		if err := c.Get(context.Background(), client.ObjectKey{Namespace: "wandb", Name: "mount0-wandb-seaweedfs-volume-0"}, actual); err != nil {
			t.Fatal(err)
		}
		if got := actual.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse("100Gi")) != 0 {
			t.Fatalf("PVC request = %s, want 100Gi", got.String())
		}
	})
}
