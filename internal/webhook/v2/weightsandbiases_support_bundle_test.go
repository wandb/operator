package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("WeightsAndBiases supportBundle schema", func() {
	newWandb := func(name string) *appsv2.WeightsAndBiases {
		wandb := &appsv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
		wandb.Spec.Wandb.Hostname = "https://wandb.example.com"
		wandb.Spec.Wandb.Version = testPinnedVersion
		return wandb
	}

	It("defaults enabled to true and round-trips the version override and status", func() {
		ctx := context.Background()
		wandb := newWandb("support-bundle-defaults")
		Expect(k8sClient.Create(ctx, wandb)).To(Succeed())
		DeferCleanup(k8sClient.Delete, ctx, wandb)

		stored := &appsv2.WeightsAndBiases{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(wandb), stored)).To(Succeed())
		Expect(stored.Spec.SupportBundle).To(Equal(appsv2.SupportBundleSpec{Enabled: ptr.To(true)}))

		stored.Spec.SupportBundle = appsv2.SupportBundleSpec{Enabled: ptr.To(true), Version: "0.4.8-rc1"}
		Expect(k8sClient.Update(ctx, stored)).To(Succeed())

		stored.Status.SupportBundle = &appsv2.SupportBundleStatusSummary{
			Ready:   true,
			CronJob: "support-bundle-defaults-support-bundle",
			Version: "0.4.8-rc1",
			Image:   "us-docker.pkg.dev/wandb-production/public/wandb/lumen:0.4.8-rc1",
		}
		Expect(k8sClient.Status().Update(ctx, stored)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(wandb), stored)).To(Succeed())
		Expect(stored.Spec.SupportBundle.Version).To(Equal("0.4.8-rc1"))
		Expect(stored.Status.SupportBundle.Version).To(Equal("0.4.8-rc1"))
	})

	It("keeps an explicit enabled: false distinct from unset", func() {
		ctx := context.Background()
		wandb := newWandb("support-bundle-explicit-off")
		wandb.Spec.SupportBundle.Enabled = ptr.To(false)
		Expect(k8sClient.Create(ctx, wandb)).To(Succeed())
		DeferCleanup(k8sClient.Delete, ctx, wandb)

		stored := &appsv2.WeightsAndBiases{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(wandb), stored)).To(Succeed())
		Expect(stored.Spec.SupportBundle.Enabled).To(Equal(ptr.To(false)))
	})

	It("writes enabled: true when the supportBundle block is omitted entirely", func() {
		ctx := context.Background()
		wandb := newWandb("support-bundle-omitted")
		raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(wandb)
		Expect(err).NotTo(HaveOccurred())
		spec := raw["spec"].(map[string]any)
		delete(spec, "supportBundle")
		obj := &unstructured.Unstructured{Object: raw}
		obj.SetGroupVersionKind(appsv2.GroupVersion.WithKind("WeightsAndBiases"))
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(k8sClient.Delete, ctx, wandb)

		stored := &appsv2.WeightsAndBiases{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(wandb), stored)).To(Succeed())
		Expect(stored.Spec.SupportBundle.Enabled).To(Equal(ptr.To(true)))
	})

	It("rejects an invalid version tag", func() {
		wandb := newWandb("support-bundle-bad-version")
		wandb.Spec.SupportBundle = appsv2.SupportBundleSpec{Enabled: ptr.To(true), Version: "0.4.8; rm -rf /"}
		err := k8sClient.Create(context.Background(), wandb)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.supportBundle.version"))
	})
})
