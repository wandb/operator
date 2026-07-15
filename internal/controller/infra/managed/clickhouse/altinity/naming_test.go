package altinity

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/util/validation"
)

var _ = Describe("managed ClickHouse naming", func() {
	Describe("KeeperNsName", func() {
		It("pairs the Keeper with the installation via the shared base name", func() {
			spec := &apiv2.ManagedClickHouseSpec{Name: "wandb-legacy-overrides-v1-chi", Namespace: "wandb"}
			Expect(KeeperNsName(spec).Name).To(Equal("wandb-legacy-overrides-v1-chk"))
		})

		It("appends -chk to explicit names without the default suffix", func() {
			spec := &apiv2.ManagedClickHouseSpec{Name: "myclickhouse", Namespace: "wandb"}
			Expect(KeeperNsName(spec).Name).To(Equal("myclickhouse-chk"))
		})
	})

	Describe("MaxSpecNameLength", func() {
		It("costs the Keeper chain no more than the CHI chain, thanks to the suffix swap", func() {
			chiRoom := validation.DNS1123LabelMaxLength -
				len(perHostConfigVolumeName("", maxExpectedHostOrdinal, maxExpectedHostOrdinal))
			Expect(MaxSpecNameLength()).To(Equal(chiRoom))
		})
	})

	Describe("DefaultSpecName", func() {
		It("keeps the plain '<cr>-chi' for CR names that fit", func() {
			Expect(DefaultSpecName("wandb", apiv2.DefaultInstanceName)).To(Equal("wandb-chi"))
			// 25 chars — wedged the old "-clickhouse"/"-keeper" naming
			Expect(DefaultSpecName("wandb-legacy-overrides-v1", apiv2.DefaultInstanceName)).To(Equal("wandb-legacy-overrides-v1-chi"))
		})

		It("keys non-default instances before the suffix so derivations still work", func() {
			name := DefaultSpecName("wandb", "analytics")

			Expect(name).To(Equal("wandb-analytics-chi"))
			spec := &apiv2.ManagedClickHouseSpec{Name: name}
			Expect(KeeperNsName(spec).Name).To(Equal("wandb-analytics-chk"))
			Expect(ValidateDerivedNames(spec)).To(Succeed())
		})

		It("derives a deployable name for CR names the plain default would wedge", func() {
			// 32 chars: "<cr>-chi" would overflow the per-host volume names
			name := DefaultSpecName("wandb-integration-environments-2", apiv2.DefaultInstanceName)

			Expect(name).To(HaveSuffix("-chi"))
			Expect(ValidateDerivedNames(&apiv2.ManagedClickHouseSpec{Name: name})).To(Succeed())
		})
	})

	Describe("ValidateDerivedNames", func() {
		It("accepts a defaulted name", func() {
			Expect(ValidateDerivedNames(&apiv2.ManagedClickHouseSpec{Name: "wandb-chi"})).To(Succeed())
		})

		It("rejects a name whose Keeper volume name exceeds the DNS-1123 label limit", func() {
			err := ValidateDerivedNames(&apiv2.ManagedClickHouseSpec{Name: "wandb-legacy-overrides-v1-clickhouse"})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("chk-wandb-legacy-overrides-v1-clickhouse-chk-deploy-confd"))
			Expect(err.Error()).To(ContainSubstring("spec.clickhouse.managedClickhouse.name"))
		})

		It("reserves ordinal digits so a name at the budget survives scaling to 100 pods", func() {
			atBudget := &apiv2.ManagedClickHouseSpec{
				Name:     strings.Repeat("a", MaxSpecNameLength()-len(defaultNameSuffix)) + defaultNameSuffix,
				Replicas: 99,
			}
			atBudget.Keeper.Replicas = 99
			Expect(ValidateDerivedNames(atBudget)).To(Succeed())

			atBudget.Name = "a" + atBudget.Name
			Expect(ValidateDerivedNames(atBudget)).To(HaveOccurred())
		})

		It("rejects characters that are invalid in derived label names", func() {
			Expect(ValidateDerivedNames(&apiv2.ManagedClickHouseSpec{Name: "wandb.prod"})).To(HaveOccurred())
		})
	})
})
