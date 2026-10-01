package seaweedfs

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	"github.com/wandb/operator/pkg/wandb/manifest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func writeScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(apiv2.AddToScheme(scheme)).To(Succeed())
	Expect(seaweedv1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func hasCondition(conds []metav1.Condition, condType string, status metav1.ConditionStatus) bool {
	for _, c := range conds {
		if c.Type == condType && c.Status == status {
			return true
		}
	}
	return false
}

var _ = Describe("SeaweedFS WriteState", func() {
	var (
		ctx      context.Context
		wandb    *apiv2.WeightsAndBiases
		desired  *seaweedv1.Seaweed
		envCfg   SeaweedS3Config
		specNsn  types.NamespacedName
		errWrite = errors.New("boom: apiserver write failed")
	)

	BeforeEach(func() {
		ctx = context.Background()
		wandb = seaweedWandb()
		var err error
		desired, err = ToObjectStoreVendorSpec(ctx, wandb, wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore, writeScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(desired).NotTo(BeNil())
		envCfg = SeaweedS3Config{AccessKey: "admin"}
		specNsn = types.NamespacedName{Namespace: "wandb", Name: "object-store"}
	})

	It("returns early and never reports a healthy connection when the CR write fails", func() {
		cl := fake.NewClientBuilder().
			WithScheme(writeScheme()).
			WithInterceptorFuncs(interceptor.Funcs{
				Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
					return errWrite
				},
			}).
			Build()

		conds, conn := WriteState(ctx, cl, specNsn, desired, envCfg, wandb)

		Expect(conn).To(BeNil())
		Expect(hasCondition(conds, common.ReconciledType, metav1.ConditionFalse)).To(BeTrue())
		// A failed CR write must not fall through to a "connection ready" report.
		Expect(hasCondition(conds, SeaweedConnectionInfoType, metav1.ConditionTrue)).To(BeFalse())
	})

	It("reports pending-create and writes a connection when the CR write succeeds", func() {
		cl := fake.NewClientBuilder().WithScheme(writeScheme()).Build()

		conds, conn := WriteState(ctx, cl, specNsn, desired, envCfg, wandb)

		Expect(hasCondition(conds, SeaweedCustomResourceType, metav1.ConditionFalse)).To(BeTrue())
		Expect(conn).NotTo(BeNil())
		Expect(hasCondition(conds, SeaweedConnectionInfoType, metav1.ConditionTrue)).To(BeTrue())
	})

	It("preserves existing StatefulSet claim template sizes", func() {
		desired.Spec.Volume.Requests[corev1.ResourceStorage] = resource.MustParse("100Gi")
		desired.Spec.Filer.Persistence.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("50Gi")
		volumeStatefulSet := statefulSetWithStorage(desired.Name+"-volume", desired.Namespace, "10Gi")
		filerStatefulSet := statefulSetWithStorage(desired.Name+"-filer", desired.Namespace, "20Gi")
		cl := fake.NewClientBuilder().
			WithScheme(writeScheme()).
			WithObjects(volumeStatefulSet, filerStatefulSet).
			Build()

		_, _ = WriteState(ctx, cl, specNsn, desired, envCfg, wandb)

		actual := &seaweedv1.Seaweed{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(desired), actual)).To(Succeed())
		Expect(actual.Spec.Volume.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("10Gi")))
		Expect(actual.Spec.Filer.Persistence.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("20Gi")))
	})
})

func statefulSetWithStorage(name, namespace, storage string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: appsv1.StatefulSetSpec{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(storage),
						},
					},
				},
			}},
		},
	}
}
