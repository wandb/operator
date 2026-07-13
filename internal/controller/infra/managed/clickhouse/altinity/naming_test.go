package altinity

import (
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
			chiRoom := validation.DNS1123LabelMaxLength - len(perHostConfigVolumeName("", ShardsCount-1, assumedMaxHostOrdinal))
			Expect(MaxSpecNameLength()).To(Equal(chiRoom))
		})
	})

	Describe("DefaultSpecName", func() {
		It("keeps the plain '<cr>-chi' for CR names that fit", func() {
			Expect(DefaultSpecName("wandb")).To(Equal("wandb-chi"))
			// 25 chars — wedged the old "<cr>-clickhouse"/"-keeper" naming, but
			// fits the terse suffixes without shortening.
			Expect(DefaultSpecName("wandb-legacy-overrides-v1")).To(Equal("wandb-legacy-overrides-v1-chi"))
		})

		It("derives a deployable name for CR names the plain default would wedge", func() {
			// 32 chars: "<cr>-chi" would push the per-host volume names past 63
			// chars and the Altinity operator would never converge.
			name := DefaultSpecName("wandb-integration-environments-2")

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

		It("accounts for replica counts that add host-ordinal digits", func() {
			// 30-char CR name: the plain default lands exactly on the budget.
			atBudget := &apiv2.ManagedClickHouseSpec{Name: DefaultSpecName("wandb-integration-environments")}
			Expect(len(atBudget.Name)).To(Equal(MaxSpecNameLength()))
			Expect(ValidateDerivedNames(atBudget)).To(Succeed())

			atBudget.Keeper.Replicas = 11
			Expect(ValidateDerivedNames(atBudget)).To(HaveOccurred())
		})

		It("rejects characters that are invalid in derived label names", func() {
			Expect(ValidateDerivedNames(&apiv2.ManagedClickHouseSpec{Name: "wandb.prod"})).To(HaveOccurred())
		})
	})
})
