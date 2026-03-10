package manifest_test

import (
	"context"
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

	It("loads a single root manifest file", func() {
		m, err := manifest.LoadManifestFromFile(context.Background(), manifestRepository(), "0.78.0-single-file")
		Expect(err).NotTo(HaveOccurred())

		Expect(m.Features).NotTo(BeNil())
		Expect(m.Features["filestreamQueue"]).To(BeFalse())
		Expect(m.Kafka.Topics).To(HaveLen(4))
		Expect(m.Applications).NotTo(BeEmpty())
	})

	It("loads a single manifest file from a version directory", func() {
		m, err := manifest.LoadManifestFromFile(context.Background(), manifestRepository(), "0.78.0")
		Expect(err).NotTo(HaveOccurred())

		Expect(m.Features).NotTo(BeNil())
		Expect(m.Features["proxy"]).To(BeFalse())
		Expect(m.Kafka.Topics).To(HaveLen(4))
		Expect(m.Migrations).To(HaveKey("gorilla"))
	})

	It("merges multiple manifest files from a version directory", func() {
		m, err := manifest.LoadManifestFromFile(context.Background(), manifestRepository(), "0.78.0-pre")
		Expect(err).NotTo(HaveOccurred())

		// Features (match current testing manifest values)
		Expect(m.Features["filestreamQueue"]).To(BeFalse())
		Expect(m.Features["proxy"]).To(BeFalse())

		// Kafka topics at top-level
		Expect(m.Kafka.Topics).To(HaveLen(4))
		Expect(m.Kafka.Topics[0].Name).To(Equal("filestream"))
		Expect(m.Kafka.Topics[0].Topic).To(Equal("filestream"))
		Expect(m.Kafka.Topics[0].PartitionCount).To(Equal(48))
		Expect(m.Kafka.Topics[0].Features).To(ContainElement("filestreamQueue"))

		// Applications basic presence
		Expect(m.Applications).NotTo(BeEmpty())
		// Find api app
		var api *manifest.Application
		for i := range m.Applications {
			if m.Applications[i].Name == "api" {
				api = &m.Applications[i]
				break
			}
		}
		Expect(api).NotTo(BeNil())
		Expect(api.InitContainers).To(BeEmpty())

		// Migrations
		Expect(m.Migrations).To(HaveKey("gorilla"))
		Expect(m.Migrations).To(HaveKey("weave-trace"))
		Expect(m.Migrations["gorilla"].Image.Repository).To(Equal("us-docker.pkg.dev/wandb-production/public/wandb/megabinary"))
		Expect(m.Migrations["gorilla"].Args).To(ContainElement("migrate"))

		// Sizing comes from the split sizing.yaml file.
		Expect(m.Kafka.Sizing["default"].Replicas).To(Equal(int32(1)))
		Expect(m.Bucket["default"].Sizing["default"].Replicas).To(Equal(int32(1)))
	})
})
