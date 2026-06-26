package strimzi

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	"github.com/wandb/operator/pkg/utils"
	v1 "github.com/wandb/operator/pkg/vendored/strimzi-kafka/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Kafka vendor specs", func() {
	BeforeEach(func() {
		utils.SetOpenShiftMode(false)
	})

	It("renders hardened Strimzi templates and preserves KAFKA_FSGROUP", func() {
		originalFSGroup, hadFSGroup := os.LookupEnv("KAFKA_FSGROUP")
		Expect(os.Setenv("KAFKA_FSGROUP", "4242")).To(Succeed())
		DeferCleanup(func() {
			if hadFSGroup {
				Expect(os.Setenv("KAFKA_FSGROUP", originalFSGroup)).To(Succeed())
				return
			}
			Expect(os.Unsetenv("KAFKA_FSGROUP")).To(Succeed())
		})

		wandb := kafkaWandb()

		kafka, err := ToKafkaVendorSpec(context.Background(), wandb, kafkaScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(kafka).NotTo(BeNil())
		expectKafkaDefaultPodSecurityContext(kafka.Spec.Kafka.Template.Pod.SecurityContext, int64(4242))
		expectKafkaDefaultPodSecurityContext(kafka.Spec.EntityOperator.Template.Pod.SecurityContext, int64(4242))
		expectKafkaDefaultContainerSecurityContext(kafka.Spec.EntityOperator.Template.TopicOperatorContainer.SecurityContext)
		expectKafkaDefaultContainerSecurityContext(kafka.Spec.EntityOperator.Template.UserOperatorContainer.SecurityContext)
		expectKafkaDefaultContainerSecurityContext(kafka.Spec.EntityOperator.Template.TlsSidecarContainer.SecurityContext)

		nodePool, err := ToKafkaNodePoolVendorSpec(context.Background(), wandb, kafkaScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(nodePool).NotTo(BeNil())
		expectKafkaDefaultPodSecurityContext(nodePool.Spec.Template.Pod.SecurityContext, int64(4242))
		expectKafkaDefaultContainerSecurityContext(nodePool.Spec.Template.InitContainer.SecurityContext)
		expectKafkaDefaultContainerSecurityContext(nodePool.Spec.Template.KafkaContainer.SecurityContext)
	})

	It("omits fixed Strimzi IDs in OpenShift mode", func() {
		utils.SetOpenShiftMode(true)

		kafka, err := ToKafkaVendorSpec(context.Background(), kafkaWandb(), kafkaScheme(), manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(kafka).NotTo(BeNil())

		expectKafkaOpenShiftPodSecurityContext(kafka.Spec.Kafka.Template.Pod.SecurityContext)
		expectKafkaOpenShiftPodSecurityContext(kafka.Spec.EntityOperator.Template.Pod.SecurityContext)
		expectKafkaOpenShiftContainerSecurityContext(kafka.Spec.EntityOperator.Template.TopicOperatorContainer.SecurityContext)
	})
})

func kafkaScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(apiv2.AddToScheme(scheme)).To(Succeed())
	Expect(v1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func kafkaWandb() *apiv2.WeightsAndBiases {
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
			Kafka: apiv2.KafkaSpec{
				ManagedKafka: &apiv2.ManagedKafkaSpec{
					Name:        "kafka",
					Namespace:   "wandb",
					Replicas:    3,
					StorageSize: "10Gi",
				},
			},
		},
	}
}

func expectKafkaDefaultPodSecurityContext(securityContext *corev1.PodSecurityContext, fsGroup int64) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).NotTo(BeNil())
	Expect(*securityContext.RunAsUser).To(Equal(kafkaRunAsUser))
	Expect(securityContext.RunAsGroup).NotTo(BeNil())
	Expect(*securityContext.RunAsGroup).To(Equal(kafkaRunAsGroup))
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.FSGroup).NotTo(BeNil())
	Expect(*securityContext.FSGroup).To(Equal(fsGroup))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectKafkaDefaultContainerSecurityContext(securityContext *corev1.SecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).NotTo(BeNil())
	Expect(*securityContext.RunAsUser).To(Equal(kafkaRunAsUser))
	Expect(securityContext.RunAsGroup).NotTo(BeNil())
	Expect(*securityContext.RunAsGroup).To(Equal(kafkaRunAsGroup))
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.AllowPrivilegeEscalation).NotTo(BeNil())
	Expect(*securityContext.AllowPrivilegeEscalation).To(BeFalse())
	Expect(securityContext.Capabilities).NotTo(BeNil())
	Expect(securityContext.Capabilities.Drop).To(ContainElement(kafkaCapabilityAll))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectKafkaOpenShiftPodSecurityContext(securityContext *corev1.PodSecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).To(BeNil())
	Expect(securityContext.RunAsGroup).To(BeNil())
	Expect(securityContext.FSGroup).To(BeNil())
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

func expectKafkaOpenShiftContainerSecurityContext(securityContext *corev1.SecurityContext) {
	Expect(securityContext).NotTo(BeNil())
	Expect(securityContext.RunAsUser).To(BeNil())
	Expect(securityContext.RunAsGroup).To(BeNil())
	Expect(securityContext.RunAsNonRoot).NotTo(BeNil())
	Expect(*securityContext.RunAsNonRoot).To(BeTrue())
	Expect(securityContext.AllowPrivilegeEscalation).NotTo(BeNil())
	Expect(*securityContext.AllowPrivilegeEscalation).To(BeFalse())
	Expect(securityContext.Capabilities).NotTo(BeNil())
	Expect(securityContext.Capabilities.Drop).To(ContainElement(kafkaCapabilityAll))
	Expect(securityContext.SeccompProfile).NotTo(BeNil())
	Expect(securityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}
