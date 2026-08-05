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
})
