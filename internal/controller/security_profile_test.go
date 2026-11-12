package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Workload security profile CR", func() {
	It("persists partial overrides and explicit false without defaulting unset fields", func() {
		shared := apiv2.WorkloadSecurityProfile{RunAsNonRoot: ptr.To(true)}
		application := apiv2.WorkloadSecurityProfile{RunAsNonRoot: ptr.To(false), ReadOnlyRootFilesystem: ptr.To(true)}
		migration := apiv2.WorkloadSecurityProfile{ReadOnlyRootFilesystem: ptr.To(false)}
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "security-profile-", Namespace: "default"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Size: apiv2.SizeDev,
				Wandb: apiv2.WandbAppSpec{
					Hostname: "http://localhost", Version: "0.85.0", Features: map[string]bool{},
					SecurityProfile: shared,
					Applications:    map[string]apiv2.WandbApplicationOverride{"api": {SecurityProfile: application}},
					Migrations:      map[string]apiv2.WandbMigrationOverride{"gorilla": {SecurityProfile: migration}},
				},
			},
		}
		Expect(k8sClient.Create(ctx, wandb)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, wandb)).To(Succeed()) })
		actual := &apiv2.WeightsAndBiases{}
		key := client.ObjectKeyFromObject(wandb)
		Expect(k8sClient.Get(ctx, key, actual)).To(Succeed())
		Expect(actual.Spec.Wandb.SecurityProfile).To(Equal(shared))
		Expect(actual.Spec.Wandb.Applications["api"].SecurityProfile).To(Equal(application))
		Expect(actual.Spec.Wandb.Migrations["gorilla"].SecurityProfile).To(Equal(migration))

		copy := actual.DeepCopy()
		*copy.Spec.Wandb.SecurityProfile.RunAsNonRoot = false
		*copy.Spec.Wandb.Applications["api"].SecurityProfile.RunAsNonRoot = true
		*copy.Spec.Wandb.Migrations["gorilla"].SecurityProfile.ReadOnlyRootFilesystem = true
		Expect(actual.Spec.Wandb.SecurityProfile).To(Equal(shared))
		Expect(actual.Spec.Wandb.Applications["api"].SecurityProfile).To(Equal(application))
		Expect(actual.Spec.Wandb.Migrations["gorilla"].SecurityProfile).To(Equal(migration))

		Expect(k8sClient.Patch(ctx, actual, client.RawPatch(types.MergePatchType,
			[]byte(`{"spec":{"wandb":{"securityProfile":{"runAsNonRoot":"false"}}}}`)))).NotTo(Succeed())

		Expect(k8sClient.Get(ctx, key, actual)).To(Succeed())
		actual.Spec.Wandb.SecurityProfile = apiv2.WorkloadSecurityProfile{}
		actual.Spec.Wandb.Applications = nil
		actual.Spec.Wandb.Migrations = nil
		Expect(k8sClient.Update(ctx, actual)).To(Succeed())
		Expect(k8sClient.Get(ctx, key, actual)).To(Succeed())
		Expect(actual.Spec.Wandb.SecurityProfile).To(Equal(apiv2.WorkloadSecurityProfile{}))
		Expect(actual.Spec.Wandb.Applications).To(BeEmpty())
		Expect(actual.Spec.Wandb.Migrations).To(BeEmpty())
	})
})
