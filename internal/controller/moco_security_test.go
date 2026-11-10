package controller

import (
	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	mococonstants "github.com/cybozu-go/moco/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MOCO copy init security API", func() {
	It("admits and retains the supported copy init override", func() {
		security := (*mocov1beta2.SecurityContextApplyConfiguration)(corev1ac.SecurityContext().
			WithAllowPrivilegeEscalation(false).
			WithReadOnlyRootFilesystem(true).
			WithCapabilities(corev1ac.Capabilities().WithDrop("ALL")))
		cluster := &mocov1beta2.MySQLCluster{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "security-profile-", Namespace: "default"},
			Spec: mocov1beta2.MySQLClusterSpec{
				Replicas: 1,
				VolumeClaimTemplates: []mocov1beta2.PersistentVolumeClaim{{
					ObjectMeta: mocov1beta2.ObjectMeta{Name: mococonstants.MySQLDataVolumeName},
					Spec: mocov1beta2.PersistentVolumeClaimSpecApplyConfiguration(*corev1ac.PersistentVolumeClaimSpec().
						WithAccessModes(corev1.ReadWriteOnce).
						WithResources(corev1ac.VolumeResourceRequirements().WithRequests(corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}))),
				}},
				PodTemplate: mocov1beta2.PodTemplateSpec{
					Spec: mocov1beta2.PodSpecApplyConfiguration(*corev1ac.PodSpec().WithContainers(
						corev1ac.Container().WithName("mysqld").WithImage("ghcr.io/cybozu-go/moco/mysql:8.4.8"))),
					OverwriteContainers: []mocov1beta2.OverwriteContainer{{
						Name:            mocov1beta2.OverwriteableContainerName(mococonstants.CopyInitContainerName),
						SecurityContext: security,
					}},
				},
			},
		}
		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cluster)).To(Succeed()) })
		actual := &mocov1beta2.MySQLCluster{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), actual)).To(Succeed())
		Expect(actual.Spec.PodTemplate.OverwriteContainers).To(HaveLen(1))
		Expect(actual.Spec.PodTemplate.OverwriteContainers[0].Name.String()).To(Equal(mococonstants.CopyInitContainerName))
		Expect(actual.Spec.PodTemplate.OverwriteContainers[0].SecurityContext).To(Equal(security))
	})
})
