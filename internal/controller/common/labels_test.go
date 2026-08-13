package common

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("BuildIdentityLabels", func() {
	It("sets descriptive identity labels with CR name as instance", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb-ns"},
		}

		Expect(BuildIdentityLabels(wandb, "api")).To(Equal(map[string]string{
			AppNameLabel:      "api",
			AppInstanceLabel:  "wandb",
			AppPartOfLabel:    PartOfWandb,
			AppManagedByLabel: ManagedByWandbOperator,
		}))
	})
})

var _ = Describe("BuildApplicationLabels", func() {
	It("merges ownership and identity labels", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb-ns"},
		}

		Expect(BuildApplicationLabels(wandb, "api")).To(Equal(map[string]string{
			WandbNameLabel:      "wandb",
			WandbNamespaceLabel: "wandb-ns",
			WandbComponentLabel: "api",
			AppNameLabel:        "api",
			AppInstanceLabel:    "wandb",
			AppPartOfLabel:      PartOfWandb,
			AppManagedByLabel:   ManagedByWandbOperator,
		}))
	})
})

var _ = Describe("HasAllLabelKeys", func() {
	It("returns true when every desired key is present", func() {
		Expect(HasAllLabelKeys(
			map[string]string{"a": "1", "b": "2", "c": "3"},
			map[string]string{"a": "x", "b": "y"},
		)).To(BeTrue())
	})

	It("returns false when a desired key is missing", func() {
		Expect(HasAllLabelKeys(
			map[string]string{"a": "1"},
			map[string]string{"a": "1", "b": "2"},
		)).To(BeFalse())
	})
})
