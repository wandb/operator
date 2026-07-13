package common

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation"
)

var _ = Describe("FitDefaultInfraName", func() {
	// A deliberately tight budget so the long-suffix cases exercise shortening;
	// real budgets come from each infra package's naming helpers.
	const budget = 27

	It("keeps the plain name when it fits the budget", func() {
		Expect(FitDefaultInfraName("wandb", "-clickhouse", budget)).To(Equal("wandb-clickhouse"))
	})

	It("shortens over-budget names deterministically and keeps the suffix", func() {
		first := FitDefaultInfraName("wandb-legacy-overrides-v1", "-clickhouse", budget)
		second := FitDefaultInfraName("wandb-legacy-overrides-v1", "-clickhouse", budget)

		Expect(first).To(Equal(second))
		Expect(len(first)).To(BeNumerically("<=", budget))
		Expect(first).To(HaveSuffix("-clickhouse"))
		Expect(validation.IsDNS1123Label(first)).To(BeEmpty())
	})

	It("derives distinct names for distinct CR names sharing a long prefix", func() {
		one := FitDefaultInfraName("wandb-production-eu-west-1", "-clickhouse", budget)
		two := FitDefaultInfraName("wandb-production-eu-west-2", "-clickhouse", budget)

		Expect(one).NotTo(Equal(two))
	})

	It("sanitizes dots, which are legal in CR names but not in labels", func() {
		name := FitDefaultInfraName("wandb.prod.eu", "-clickhouse", budget)

		Expect(name).NotTo(ContainSubstring("."))
		Expect(len(name)).To(BeNumerically("<=", budget))
		Expect(validation.IsDNS1123Label(name)).To(BeEmpty())
	})

	It("survives truncation points that land on a hyphen", func() {
		// prefix budget is 10 here; the 10th char of the CR name is '-'
		name := FitDefaultInfraName("wandb-abcd-something-long-enough", "-clickhouse", budget)

		Expect(name).NotTo(ContainSubstring("--"))
		Expect(validation.IsDNS1123Label(name)).To(BeEmpty())
	})
})
