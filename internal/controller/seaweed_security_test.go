package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Seaweed component security API", func() {
	It("persists contexts for all managed components", func() {
		component := seaweedv1.ComponentSpec{
			PodSecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   new(true),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			ContainerSecurityContext: &corev1.SecurityContext{
				ReadOnlyRootFilesystem: new(true), AllowPrivilegeEscalation: new(false),
				Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			},
		}
		seaweed := &seaweedv1.Seaweed{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "security-profile-", Namespace: "default"},
			Spec: seaweedv1.SeaweedSpec{
				Image:  "chrislusf/seaweedfs:4.35",
				Master: &seaweedv1.MasterSpec{Replicas: 1, ComponentSpec: component},
				Volume: &seaweedv1.VolumeSpec{Replicas: 1, VolumeServerConfig: seaweedv1.VolumeServerConfig{ComponentSpec: component}},
				Filer:  &seaweedv1.FilerSpec{Replicas: 1, ComponentSpec: component},
				S3:     &seaweedv1.S3GatewaySpec{Replicas: 1, ComponentSpec: component},
			},
		}
		Expect(k8sClient.Create(ctx, seaweed)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, seaweed)).To(Succeed()) })
		actual := &seaweedv1.Seaweed{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(seaweed), actual)).To(Succeed())
		for _, got := range []seaweedv1.ComponentSpec{actual.Spec.Master.ComponentSpec, actual.Spec.Volume.ComponentSpec, actual.Spec.Filer.ComponentSpec, actual.Spec.S3.ComponentSpec} {
			Expect(got.PodSecurityContext).To(Equal(component.PodSecurityContext))
			Expect(got.ContainerSecurityContext).To(Equal(component.ContainerSecurityContext))
		}
		copy := actual.DeepCopy()
		*copy.Spec.Master.ContainerSecurityContext.ReadOnlyRootFilesystem = false
		*copy.Spec.Master.PodSecurityContext.RunAsNonRoot = false
		Expect(actual.Spec.Master.ContainerSecurityContext.ReadOnlyRootFilesystem).To(HaveValue(BeTrue()))
		Expect(actual.Spec.Master.PodSecurityContext.RunAsNonRoot).To(HaveValue(BeTrue()))
	})
})
