package keeper

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	chkv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse-keeper.altinity.com/v1"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Keeper vendor spec", func() {
	BeforeEach(func() {
		utils.SetOpenShiftMode(false)
	})

	It("builds a CHK with explicit replicas, storage, and a hardened pod", func() {
		wandb := keeperWandb()
		wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.Keeper = apiv2.ClickHouseKeeperSpec{
			Replicas:    5,
			StorageSize: "20Gi",
			Config: apiv2.ClickHouseConfig{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m")},
				},
			},
		}

		chk, err := ToKeeperVendorSpec(context.Background(), wandb, wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse, keeperScheme(), keeperNsName())
		Expect(err).NotTo(HaveOccurred())
		Expect(chk).NotTo(BeNil())
		Expect(chk.Name).To(Equal("clickhouse-chk"))
		Expect(chk.Namespace).To(Equal("wandb"))

		Expect(chk.Spec.Configuration.Clusters).To(HaveLen(1))
		Expect(chk.Spec.Configuration.Clusters[0].Layout.ReplicasCount).To(Equal(5))

		Expect(chk.Spec.Templates.VolumeClaimTemplates).To(HaveLen(1))
		storage := chk.Spec.Templates.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
		Expect(storage).To(Equal(resource.MustParse("20Gi")))

		Expect(chk.Spec.Templates.PodTemplates).To(HaveLen(1))
		container := chk.Spec.Templates.PodTemplates[0].Spec.Containers[0]
		Expect(container.Image).To(Equal(KeeperImage))
		Expect(container.Name).To(Equal(keeperContainerName))
		Expect(container.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("250m")))

		sc := chk.Spec.Templates.PodTemplates[0].Spec.SecurityContext
		Expect(sc).NotTo(BeNil())
		Expect(sc.RunAsUser).NotTo(BeNil())
		Expect(*sc.RunAsUser).To(Equal(keeperRunAsUser))
		Expect(sc.RunAsNonRoot).NotTo(BeNil())
		Expect(*sc.RunAsNonRoot).To(BeTrue())
	})

	It("errors when keeper storage size is unset (no operator defaults)", func() {
		wandb := keeperWandb()
		wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.Keeper = apiv2.ClickHouseKeeperSpec{}
		_, err := ToKeeperVendorSpec(context.Background(), wandb, wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse, keeperScheme(), keeperNsName())
		Expect(err).To(HaveOccurred())
	})

	It("omits fixed IDs in OpenShift mode", func() {
		utils.SetOpenShiftMode(true)
		wandb := keeperWandb()
		chk, err := ToKeeperVendorSpec(context.Background(), wandb, wandb.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse, keeperScheme(), keeperNsName())
		Expect(err).NotTo(HaveOccurred())
		sc := chk.Spec.Templates.PodTemplates[0].Spec.SecurityContext
		Expect(sc.RunAsUser).To(BeNil())
		Expect(sc.RunAsNonRoot).NotTo(BeNil())
		Expect(*sc.RunAsNonRoot).To(BeTrue())
	})
})

// keeperNsName mirrors what altinity.KeeperNsName derives for keeperWandb().
func keeperNsName() types.NamespacedName {
	return types.NamespacedName{Namespace: "wandb", Name: InstallationName("clickhouse")}
}

func keeperScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(apiv2.AddToScheme(scheme)).To(Succeed())
	Expect(chiv1.AddToScheme(scheme)).To(Succeed())
	Expect(chkv1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func keeperWandb() *apiv2.WeightsAndBiases {
	tolerations := []corev1.Toleration{}
	return &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiv2.GroupVersion.String(),
			Kind:       "WeightsAndBiases",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb"},
		Spec: apiv2.WeightsAndBiasesSpec{
			Tolerations: &tolerations,
			ClickHouse: map[string]apiv2.ClickHouseSpec{
				apiv2.DefaultInstanceName: {
					ManagedClickHouse: &apiv2.ManagedClickHouseSpec{
						Name:      "clickhouse",
						Namespace: "wandb",
						Keeper: apiv2.ClickHouseKeeperSpec{
							Replicas:    3,
							StorageSize: "10Gi",
						},
					},
				},
			},
		},
	}
}
