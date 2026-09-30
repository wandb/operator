package controller

import (
	"net/url"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Manifest compatibility through the v2 API", func() {
	It("persists a blocked condition while allowing reads, edits, and deletion", func() {
		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "future.yaml"), []byte("manifestVersion: 2\napplications: [future, shape]\n"), 0600)).To(Succeed())
		w := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "manifest-compatibility", Namespace: "default"},
		}
		w.Spec.Wandb.Hostname = "http://localhost"
		w.Spec.Wandb.Version = "future"
		w.Spec.Wandb.ManifestRepository = (&url.URL{Scheme: "file", Path: dir}).String()
		Expect(k8sClient.Create(ctx, w)).To(Succeed())
		key := client.ObjectKeyFromObject(w)
		DeferCleanup(func() {
			if err := k8sClient.Get(ctx, key, w); apierrors.IsNotFound(err) {
				return
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			w.Finalizers = nil
			Expect(k8sClient.Update(ctx, w)).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, w))).To(Succeed())
		})
		r := &WeightsAndBiasesReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Recorder: record.NewFakeRecorder(10)}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		Expect(k8sClient.Get(ctx, key, w)).To(Succeed())
		condition := apimeta.FindStatusCondition(w.Status.Conditions, "ManifestCompatible")
		Expect(condition).NotTo(BeNil())
		Expect(condition.Reason).To(Equal("UnsupportedManifestVersion"))
		Expect(condition.ObservedGeneration).To(Equal(w.Generation))
		Expect(w.Status.Ready).To(BeFalse())

		// A v2 correction must pass admission even while its artifact is blocked.
		w.Spec.Wandb.Version = "corrected-reference"
		Expect(k8sClient.Update(ctx, w)).To(Succeed())
		Expect(k8sClient.Get(ctx, key, w)).To(Succeed())
		Expect(w.Spec.Wandb.Version).To(Equal("corrected-reference"))
		Expect(k8sClient.Delete(ctx, w)).To(Succeed())
		_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, w))).To(BeTrue())
	})
})
