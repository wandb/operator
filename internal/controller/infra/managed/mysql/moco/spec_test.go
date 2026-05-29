package moco

import (
	"context"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

var _ = Describe("Moco MySQL specs", func() {
	BeforeEach(func() {
		utils.SetOpenShiftMode(false)
	})

	It("renders hardened pod and container security settings", func() {
		cluster, _, err := ToMocoMySQLClusterSpec(
			context.Background(),
			apiv2.ManagedMysqlSpec{
				Name:        "mysql",
				Namespace:   "wandb",
				Replicas:    3,
				StorageSize: "10Gi",
			},
			mocoWandb(),
			mocoScheme(),
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(cluster).NotTo(BeNil())

		podSpec := cluster.Spec.PodTemplate.Spec
		expectMocoPodSecurityContext(podSpec.SecurityContext)
		Expect(podSpec.Containers).To(HaveLen(1))
		Expect(*podSpec.Containers[0].Name).To(Equal("mysqld"))
		expectMocoContainerSecurityContext(podSpec.Containers[0].SecurityContext)

		Expect(cluster.Spec.PodTemplate.OverwriteContainers).To(HaveLen(4))
		for _, overwrite := range cluster.Spec.PodTemplate.OverwriteContainers {
			Expect(overwrite.Name).To(BeElementOf(
				mocov1beta2.AgentContainerName,
				mocov1beta2.InitContainerName,
				mocov1beta2.SlowQueryLogAgentContainerName,
				mocov1beta2.ExporterContainerName,
			))
			Expect(overwrite.SecurityContext).NotTo(BeNil())
			expectMocoContainerSecurityContext((*corev1ac.SecurityContextApplyConfiguration)(overwrite.SecurityContext))
		}
	})

	It("omits fixed Moco IDs in OpenShift mode", func() {
		utils.SetOpenShiftMode(true)

		cluster, _, err := ToMocoMySQLClusterSpec(
			context.Background(),
			apiv2.ManagedMysqlSpec{
				Name:        "mysql",
				Namespace:   "wandb",
				Replicas:    3,
				StorageSize: "10Gi",
			},
			mocoWandb(),
			mocoScheme(),
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(cluster).NotTo(BeNil())

		podSpec := cluster.Spec.PodTemplate.Spec
		expectMocoOpenShiftPodSecurityContext(podSpec.SecurityContext)
		expectMocoOpenShiftContainerSecurityContext(podSpec.Containers[0].SecurityContext)
	})
})

func mocoScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(apiv2.AddToScheme(scheme)).To(Succeed())
	Expect(mocov1beta2.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func mocoWandb() *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiv2.GroupVersion.String(),
			Kind:       "WeightsAndBiases",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "wandb",
		},
	}
}

func expectMocoPodSecurityContext(securityContext *corev1ac.PodSecurityContextApplyConfiguration) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).NotTo(BeNil())
	Expect(*securityContext.RunAsUser).To(Equal(mocoMySQLRunAsUser))
	Expect(securityContext.RunAsGroup).NotTo(BeNil())
	Expect(*securityContext.RunAsGroup).To(Equal(mocoMySQLRunAsGroup))
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.FSGroup).NotTo(BeNil())
	Expect(*securityContext.FSGroup).To(Equal(mocoMySQLFSGroup))
	Expect(securityContext.FSGroupChangePolicy).NotTo(BeNil())
	Expect(*securityContext.FSGroupChangePolicy).To(Equal(corev1.FSGroupChangeOnRootMismatch))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).NotTo(BeNil())
	Expect(*securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectMocoContainerSecurityContext(securityContext *corev1ac.SecurityContextApplyConfiguration) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).NotTo(BeNil())
	Expect(*securityContext.RunAsUser).To(Equal(mocoMySQLRunAsUser))
	Expect(securityContext.RunAsGroup).NotTo(BeNil())
	Expect(*securityContext.RunAsGroup).To(Equal(mocoMySQLRunAsGroup))
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.AllowPrivilegeEscalation).NotTo(BeNil())
	Expect(*securityContext.AllowPrivilegeEscalation).To(BeFalse())
	Expect(securityContext.Capabilities).NotTo(BeNil())
	Expect(securityContext.Capabilities.Drop).To(ContainElement(mocoMySQLCapabilityAll))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).NotTo(BeNil())
	Expect(*securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectMocoOpenShiftPodSecurityContext(securityContext *corev1ac.PodSecurityContextApplyConfiguration) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).To(BeNil())
	Expect(securityContext.RunAsGroup).To(BeNil())
	Expect(securityContext.FSGroup).To(BeNil())
	Expect(securityContext.FSGroupChangePolicy).To(BeNil())
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).NotTo(BeNil())
	Expect(*securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectMocoOpenShiftContainerSecurityContext(securityContext *corev1ac.SecurityContextApplyConfiguration) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).To(BeNil())
	Expect(securityContext.RunAsGroup).To(BeNil())
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.AllowPrivilegeEscalation).NotTo(BeNil())
	Expect(*securityContext.AllowPrivilegeEscalation).To(BeFalse())
	Expect(securityContext.Capabilities).NotTo(BeNil())
	Expect(securityContext.Capabilities.Drop).To(ContainElement(mocoMySQLCapabilityAll))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).NotTo(BeNil())
	Expect(*securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}
