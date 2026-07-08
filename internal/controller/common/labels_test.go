package common

import (
	apiv2 "github.com/wandb/operator/api/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("StandardLabels", func() {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb-system"},
	}

	It("sets instance to the CR name, not the namespace", func() {
		labels := StandardLabels(wandb, "api", RoleServer, "0.80.0")
		Expect(labels[StandardInstanceLabel]).To(Equal("wandb"))
		Expect(labels[StandardNameLabel]).To(Equal("api"))
		Expect(labels[StandardComponentLabel]).To(Equal(RoleServer))
		Expect(labels[StandardPartOfLabel]).To(Equal(PartOfValue))
		Expect(labels[StandardManagedByLabel]).To(Equal(ManagedByValue))
		Expect(labels[StandardVersionLabel]).To(Equal("0.80.0"))
	})

	It("omits component and version when empty", func() {
		labels := StandardLabels(wandb, "generated-secret", "", "")
		Expect(labels).NotTo(HaveKey(StandardComponentLabel))
		Expect(labels).NotTo(HaveKey(StandardVersionLabel))
		Expect(labels).To(HaveKeyWithValue(StandardPartOfLabel, PartOfValue))
	})
})

var _ = Describe("AppComponentRole", func() {
	It("maps known workers and proxies, defaulting others to server", func() {
		Expect(AppComponentRole("executor")).To(Equal(RoleWorker))
		Expect(AppComponentRole("parquet")).To(Equal(RoleWorker))
		Expect(AppComponentRole("nginx-proxy")).To(Equal(RoleProxy))
		Expect(AppComponentRole("api")).To(Equal(RoleServer))
		Expect(AppComponentRole("some-new-service")).To(Equal(RoleServer))
	})
})
