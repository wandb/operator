package manifest_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	manifest "github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("Server manifest YAML decode", func() {
	It("decodes 0.76.1.yaml into Manifest struct", func() {
		data, err := os.ReadFile("hack/testing-manifests/server-manifest/0.76.1.yaml")
		Expect(err).NotTo(HaveOccurred())

		var m manifest.Manifest
		Expect(yaml.Unmarshal(data, &m)).To(Succeed())

		// Features
		Expect(m.Features).NotTo(BeNil())
		Expect(m.Features.RunsV2).To(BeFalse())
		Expect(m.Features.FilestreamQueue).To(BeFalse())
		Expect(m.Features.MetricObserver).To(BeFalse())
		Expect(m.Features.WeaveTrace).To(BeFalse())

		// Kafka topics at top-level
		Expect(m.Kafka).To(HaveLen(4))
		Expect(m.Kafka[0].Name).To(Equal("filestream"))
		Expect(m.Kafka[0].Topic).To(Equal("filestream"))
		Expect(m.Kafka[0].PartitionCount).To(Equal(48))
		Expect(m.Kafka[0].Features).To(ContainElement("filestreamQueue"))

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
		Expect(api.InitContainers).NotTo(BeEmpty())
		// Per-app kafka exists and has filestream topic mapping
		Expect(api.Kafka).NotTo(BeNil())
		Expect(api.Kafka.Filestream).NotTo(BeNil())
		Expect(api.Kafka.Filestream.Topic).To(Equal("filestream"))

		// Migrations
		Expect(m.Migrations).To(HaveKey("default"))
		Expect(m.Migrations).To(HaveKey("runsdb"))
		Expect(m.Migrations).To(HaveKey("usagedb"))
		Expect(m.Migrations["default"].Image.Repository).To(Equal("wandb/megabinary"))
		Expect(m.Migrations["default"].Args).To(ContainElement("migrate"))
	})
})
