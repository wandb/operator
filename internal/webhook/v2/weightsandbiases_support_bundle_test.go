package v2

import (
	"context"
	"math"

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
		Expect(stored.Spec.SupportBundle).To(Equal(appsv2.SupportBundleSpec{
			Enabled: ptr.To(true),
			VictoriaStack: appsv2.SupportBundleVictoriaStack{
				Metrics: appsv2.SupportBundleVictoriaExport{Enabled: ptr.To(true)},
				Logs:    appsv2.SupportBundleVictoriaExport{Enabled: ptr.To(true)},
				Traces:  appsv2.SupportBundleVictoriaExport{Enabled: ptr.To(true)},
			},
			SelectedResources: appsv2.SupportBundleSelectedResources{Enabled: ptr.To(true)},
		}))

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
		Expect(stored.Spec.SupportBundle.VictoriaStack.Traces.Enabled).To(Equal(ptr.To(true)))
		Expect(stored.Spec.SupportBundle.SelectedResources.Enabled).To(Equal(ptr.To(true)))
	})

	It("keeps explicit opt-outs and bounds and defaults the rest", func() {
		ctx := context.Background()
		wandb := newWandb("support-bundle-scope")
		wandb.Spec.SupportBundle.VictoriaStack = appsv2.SupportBundleVictoriaStack{
			LookbackDuration: "12h",
			Logs:             appsv2.SupportBundleVictoriaExport{Enabled: ptr.To(false), MaxRecords: ptr.To[int64](0)},
		}
		selectors := []appsv2.SupportBundleResourceSelector{{Kind: "WeightsAndBiases", Resource: "weightsandbiases.apps.wandb.com", LabelSelector: "app=wandb"}}
		wandb.Spec.SupportBundle.SelectedResources.Selectors = selectors
		Expect(k8sClient.Create(ctx, wandb)).To(Succeed())
		DeferCleanup(k8sClient.Delete, ctx, wandb)

		stored := &appsv2.WeightsAndBiases{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(wandb), stored)).To(Succeed())
		stack := stored.Spec.SupportBundle.VictoriaStack
		Expect(stack.LookbackDuration).To(Equal("12h"))
		Expect(stack.Logs).To(Equal(appsv2.SupportBundleVictoriaExport{Enabled: ptr.To(false), MaxRecords: ptr.To[int64](0)}))
		Expect(stack.Metrics.Enabled).To(Equal(ptr.To(true)))
		Expect(stored.Spec.SupportBundle.SelectedResources).To(Equal(appsv2.SupportBundleSelectedResources{
			Enabled: ptr.To(true), Selectors: selectors,
		}))
	})

	DescribeTable("rejects invalid Victoria bounds",
		func(field string, edit func(*appsv2.SupportBundleVictoriaStack)) {
			wandb := newWandb("support-bundle-bad-bounds")
			edit(&wandb.Spec.SupportBundle.VictoriaStack)
			err := k8sClient.Create(context.Background(), wandb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(field))
		},
		Entry("zero lookback", "victoriaStack.lookbackDuration", func(s *appsv2.SupportBundleVictoriaStack) { s.LookbackDuration = "0s" }),
		Entry("unparsable timeout", "victoriaStack.timeout", func(s *appsv2.SupportBundleVictoriaStack) { s.Timeout = "soon" }),
		Entry("negative lookback", "victoriaStack.traces.lookbackDuration", func(s *appsv2.SupportBundleVictoriaStack) { s.Traces.LookbackDuration = "-1h" }),
		Entry("timeout past the Job deadline", "at most 25m", func(s *appsv2.SupportBundleVictoriaStack) { s.Timeout = "26m" }),
		Entry("negative cap", "victoriaStack.metrics.maxRecords", func(s *appsv2.SupportBundleVictoriaStack) { s.Metrics.MaxRecords = ptr.To[int64](-1) }),
		Entry("cap past the collector's limit", "victoriaStack.logs.maxRecords", func(s *appsv2.SupportBundleVictoriaStack) { s.Logs.MaxRecords = ptr.To[int64](math.MaxInt64) }),
	)

	DescribeTable("rejects invalid resource selectors",
		func(message string, selector appsv2.SupportBundleResourceSelector) {
			wandb := newWandb("support-bundle-bad-selector")
			wandb.Spec.SupportBundle.SelectedResources.Selectors = []appsv2.SupportBundleResourceSelector{selector}
			err := k8sClient.Create(context.Background(), wandb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(message))
		},
		Entry("neither kind nor resource", "set kind or resource", appsv2.SupportBundleResourceSelector{Namespace: "wandb"}),
		Entry("namespaced CRD", "cluster-scoped", appsv2.SupportBundleResourceSelector{Kind: "CustomResourceDefinition", Namespace: "wandb"}),
		Entry("unqualified resource", "selectors[0].resource", appsv2.SupportBundleResourceSelector{Resource: "weightsandbiases"}),
		Entry("kind with spaces", "selectors[0].kind", appsv2.SupportBundleResourceSelector{Kind: "Weights And Biases"}),
		Entry("miscased CRD kind", "must be spelled CustomResourceDefinition", appsv2.SupportBundleResourceSelector{Kind: "customresourcedefinition"}),
		Entry("bad label selector", "selectors[0].labelSelector", appsv2.SupportBundleResourceSelector{Kind: "Application", LabelSelector: "app in ("}),
		Entry("bad field selector", "selectors[0].fieldSelector", appsv2.SupportBundleResourceSelector{Kind: "Application", FieldSelector: "metadata.name"}),
	)

	It("accepts any namespace but warns when the in-cluster collector can't read it", func() {
		wandb := newWandb("support-bundle-other-ns")
		wandb.Spec.SupportBundle.SelectedResources.Selectors = []appsv2.SupportBundleResourceSelector{
			{Resource: "mysqlclusters.moco.cybozu.com", Namespace: "default"},
			{Resource: "mysqlclusters.moco.cybozu.com", Namespace: "mysql"},
		}
		Expect(k8sClient.Create(context.Background(), wandb)).To(Succeed())
		DeferCleanup(k8sClient.Delete, context.Background(), wandb)

		errs, warnings := validateSupportBundleSpec(wandb)
		Expect(errs).To(BeEmpty())
		Expect(warnings).To(ConsistOf(`spec.supportBundle.selectedResources.selectors[1].namespace: the in-cluster collector can only read namespace "default"`))
	})

	It("rejects an invalid version tag", func() {
		wandb := newWandb("support-bundle-bad-version")
		wandb.Spec.SupportBundle = appsv2.SupportBundleSpec{Enabled: ptr.To(true), Version: "0.4.8; rm -rf /"}
		err := k8sClient.Create(context.Background(), wandb)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.supportBundle.version"))
	})
})
