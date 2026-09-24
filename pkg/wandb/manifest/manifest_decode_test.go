package manifest_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	manifest "github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("Server manifest YAML decode", func() {
	manifestRepository := func() string {
		manifestRoot, err := filepath.Abs("../../../hack/testing-manifests/server-manifest")
		Expect(err).NotTo(HaveOccurred())
		return "file://" + filepath.ToSlash(manifestRoot)
	}

	It("merges multiple manifest files from a version directory", func() {
		m, err := manifest.LoadManifestFromFile(context.Background(), manifestRepository(), "0.83.0-clickhouse-keeper.2")
		Expect(err).NotTo(HaveOccurred())

		// Features (match current testing manifest values)
		Expect(m.Features["filestreamQueue"]).To(BeFalse())
		Expect(m.Features["proxy"]).To(BeFalse())

		// Kafka topics at top-level
		Expect(m.Kafka.Topics).To(HaveLen(4))
		Expect(m.Kafka.Topics[0].Name).To(Equal("filestream"))
		Expect(m.Kafka.Topics[0].Topic).To(Equal("filestream"))
		Expect(m.Kafka.Topics[0].PartitionCount).To(Equal(96))
		Expect(m.Kafka.Topics[0].Features).To(ContainElement("filestreamQueue"))

		// Applications basic presence
		Expect(m.Applications).NotTo(BeEmpty())
		// Find api app
		api := m.Applications["api"]

		Expect(api).NotTo(BeNil())
		Expect(api.InitContainers).To(BeEmpty())

		// Migrations
		Expect(m.Migrations).To(HaveKey("gorilla"))
		Expect(m.Migrations).To(HaveKey("weave-trace"))
		Expect(m.Migrations["gorilla"].Image.Repository).To(Equal("us-docker.pkg.dev/wandb-production/public/wandb/megabinary"))
		Expect(m.Migrations["gorilla"].Args).To(ContainElement("migrate"))
		Expect(m.Migrations["gorilla"].IgnoredErrorCodes).To(Equal([]int32{manifest.DefaultIgnoredMigrationErrorCode}))

		// Sizing comes from the split sizing.yaml file.
		Expect(m.Kafka.Sizing["default"].Replicas).To(Equal(int32(2)))
		Expect(m.Bucket["default"].Sizing["default"].Replicas).To(Equal(int32(1)))
	})

	It("defaults omitted migration error codes while preserving explicit values", func() {
		manifestRoot := GinkgoT().TempDir()
		manifestYAML := []byte(`migrations:
  defaulted:
    image:
      repository: example/defaulted
  disabled:
    image:
      repository: example/disabled
    ignoredErrorCodes: []
  configured:
    image:
      repository: example/configured
    ignoredErrorCodes: [7, 23]
`)
		Expect(os.WriteFile(filepath.Join(manifestRoot, "test.yaml"), manifestYAML, 0o600)).To(Succeed())

		m, err := manifest.LoadManifestFromFile(
			context.Background(),
			"file://"+filepath.ToSlash(manifestRoot),
			"test",
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(m.Migrations["defaulted"].IgnoredErrorCodes).To(Equal([]int32{manifest.DefaultIgnoredMigrationErrorCode}))
		Expect(m.Migrations["disabled"].IgnoredErrorCodes).To(BeEmpty())
		Expect(m.Migrations["disabled"].IgnoredErrorCodes).NotTo(BeNil())
		Expect(m.Migrations["configured"].IgnoredErrorCodes).To(Equal([]int32{7, 23}))
	})
})
